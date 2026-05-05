package feishuadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"neo-code/internal/gateway/protocol"
)

const defaultSignatureMaxSkew = 5 * time.Minute
const defaultProgressNotifyInterval = 5 * time.Second

type sessionBinding struct {
	SessionID string
	ChatID    string
	RunID     string
}

// Adapter 负责桥接飞书回调与 Gateway JSON-RPC 长连接。
type Adapter struct {
	cfg       Config
	gateway   GatewayClient
	messenger Messenger
	logger    *log.Logger
	idem      *idempotencyStore

	nowFn func() time.Time

	mu             sync.RWMutex
	activeRuns     map[string]sessionBinding
	sessionChats   map[string]string
	lastProgressAt map[string]time.Time
}

// New 创建飞书适配器实例。
func New(cfg Config, gateway GatewayClient, messenger Messenger, logger *log.Logger) (*Adapter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if gateway == nil {
		return nil, fmt.Errorf("gateway client is required")
	}
	if messenger == nil {
		return nil, fmt.Errorf("messenger is required")
	}
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return &Adapter{
		cfg:            cfg,
		gateway:        gateway,
		messenger:      messenger,
		logger:         logger,
		idem:           newIdempotencyStore(cfg.IdempotencyTTL),
		nowFn:          func() time.Time { return time.Now().UTC() },
		activeRuns:     make(map[string]sessionBinding),
		sessionChats:   make(map[string]string),
		lastProgressAt: make(map[string]time.Time),
	}, nil
}

// Run 启动飞书适配器 HTTP 服务与网关事件消费循环。
func (a *Adapter) Run(ctx context.Context) error {
	if err := a.gateway.Authenticate(ctx); err != nil {
		return fmt.Errorf("authenticate gateway: %w", err)
	}

	go a.consumeGatewayEvents(ctx)
	go a.reconnectAndRebindLoop(ctx)
	ingress := a.buildIngress()
	err := ingress.Run(ctx, a)
	_ = a.gateway.Close()
	if err != nil && err != context.Canceled {
		return err
	}
	return nil
}

// buildIngress 根据配置模式构建飞书事件入站实现。
func (a *Adapter) buildIngress() Ingress {
	switch normalizeIngressMode(a.cfg.IngressMode) {
	case IngressModeSDK:
		return NewSDKIngress(a.cfg, a.safeLog)
	default:
		return NewWebhookIngress(a.cfg, a.nowFn)
	}
}

// handleFeishuEvent 保留给现有测试使用，实际逻辑委托给 WebhookIngress。
func (a *Adapter) handleFeishuEvent(writer http.ResponseWriter, request *http.Request) {
	ingress := NewWebhookIngress(a.cfg, a.nowFn)
	webhook, ok := ingress.(*WebhookIngress)
	if !ok {
		http.Error(writer, "ingress unavailable", http.StatusInternalServerError)
		return
	}
	webhook.handleFeishuEvent(a)(writer, request)
}

// handleCardCallback 保留给现有测试使用，实际逻辑委托给 WebhookIngress。
func (a *Adapter) handleCardCallback(writer http.ResponseWriter, request *http.Request) {
	ingress := NewWebhookIngress(a.cfg, a.nowFn)
	webhook, ok := ingress.(*WebhookIngress)
	if !ok {
		http.Error(writer, "ingress unavailable", http.StatusInternalServerError)
		return
	}
	webhook.handleCardCallback(a)(writer, request)
}

// HandleMessage 处理标准化后的飞书消息事件，并复用统一的网关执行链路。
func (a *Adapter) HandleMessage(ctx context.Context, event FeishuMessageEvent) error {
	chatType := strings.TrimSpace(strings.ToLower(event.ChatType))
	if chatType == "" {
		chatType = "p2p"
	}
	if chatType == "group" && !isMentionCurrentBot(event, a.cfg.AppID) {
		return nil
	}
	if strings.TrimSpace(event.MessageID) == "" || strings.TrimSpace(event.ChatID) == "" {
		return fmt.Errorf("missing message_id or chat_id")
	}
	dedupeKey := "msg:" + strings.TrimSpace(event.EventID) + ":" + strings.TrimSpace(event.MessageID)
	if !a.idem.TryStart(dedupeKey, a.nowFn()) {
		return nil
	}
	succeeded := false
	defer func() {
		if succeeded {
			a.idem.MarkDone(dedupeKey, a.nowFn())
			return
		}
		a.idem.MarkFailed(dedupeKey)
	}()

	text := strings.TrimSpace(event.ContentText)
	if text == "" {
		return nil
	}
	if handled, err := a.tryHandleTextPermission(ctx, event.ChatID, text); handled {
		if err == nil {
			succeeded = true
		}
		return err
	}

	sessionID := BuildSessionID(event.ChatID)
	runID := BuildRunID(event.MessageID)
	if err := a.bindThenRun(ctx, sessionID, runID, event.ChatID, text); err != nil {
		a.safeLog("handle message failed: %v", err)
		_ = a.messenger.SendText(context.Background(), event.ChatID, "任务受理失败，请稍后重试。")
		return err
	}
	succeeded = true
	return nil
}

// HandleCardAction 处理标准化后的审批动作事件并映射到网关授权接口。
func (a *Adapter) HandleCardAction(ctx context.Context, event FeishuCardActionEvent) error {
	requestID := strings.TrimSpace(event.RequestID)
	decision := strings.TrimSpace(strings.ToLower(event.Decision))
	if requestID == "" || (decision != "allow_once" && decision != "reject") {
		return nil
	}
	dedupeKey := "card:" + requestID + ":" + decision
	if !a.idem.TryStart(dedupeKey, a.nowFn()) {
		return nil
	}
	succeeded := false
	defer func() {
		if succeeded {
			a.idem.MarkDone(dedupeKey, a.nowFn())
			return
		}
		a.idem.MarkFailed(dedupeKey)
	}()

	callCtx, cancel := context.WithTimeout(ctx, a.cfg.RequestTimeout)
	defer cancel()
	if err := a.gateway.ResolvePermission(callCtx, requestID, decision); err != nil {
		a.safeLog("resolve permission failed: %v", err)
		return err
	}
	succeeded = true
	return nil
}

// bindThenRun 按 authenticate -> bindStream -> run 的顺序提交一次请求并记录会话绑定。
func (a *Adapter) bindThenRun(ctx context.Context, sessionID string, runID string, chatID string, text string) error {
	callCtx, cancel := context.WithTimeout(ctx, a.cfg.RequestTimeout)
	defer cancel()
	if err := a.gateway.Authenticate(callCtx); err != nil {
		return err
	}
	if err := a.gateway.BindStream(callCtx, sessionID, runID); err != nil {
		return err
	}
	a.trackSession(sessionID, runID, chatID)
	if err := a.gateway.Run(callCtx, sessionID, runID, text); err != nil {
		// run 受理失败时及时回收活跃绑定，避免重连阶段反复重绑无效 run。
		a.untrackRun(sessionID, runID)
		return err
	}
	_ = a.messenger.SendText(context.Background(), chatID, "任务已受理，正在执行。")
	return nil
}

// trackSession 记录 session 到飞书 chat 的映射，用于事件回推。
func (a *Adapter) trackSession(sessionID string, runID string, chatID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.activeRuns[runBindingKey(sessionID, runID)] = sessionBinding{
		SessionID: sessionID,
		ChatID:    chatID,
		RunID:     runID,
	}
	if sessionID != "" && chatID != "" {
		a.sessionChats[sessionID] = chatID
	}
}

// untrackRun 在 run 终态事件到达后移除活跃 run 绑定，避免重连重绑与内存累积。
func (a *Adapter) untrackRun(sessionID string, runID string) {
	if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(runID) == "" {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.activeRuns, runBindingKey(sessionID, runID))
}

// lookupChatID 根据 session_id 查找需要回推的飞书 chat_id。
func (a *Adapter) lookupChatID(sessionID string, runID string) string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if sessionID != "" && runID != "" {
		if binding, ok := a.activeRuns[runBindingKey(sessionID, runID)]; ok {
			return binding.ChatID
		}
	}
	return a.sessionChats[sessionID]
}

// consumeGatewayEvents 持续消费网关通知流并转发到飞书侧展示。
func (a *Adapter) consumeGatewayEvents(ctx context.Context) {
	notifications := a.gateway.Notifications()
	for {
		select {
		case <-ctx.Done():
			return
		case notification, ok := <-notifications:
			if !ok {
				return
			}
			if strings.TrimSpace(notification.Method) != protocol.MethodGatewayEvent {
				continue
			}
			a.handleGatewayEvent(ctx, notification.Params)
		}
	}
}

// handleGatewayEvent 将 gateway.event 映射成飞书文本或审批卡片。
func (a *Adapter) handleGatewayEvent(ctx context.Context, raw json.RawMessage) {
	eventType, sessionID, runID, envelope, err := parseGatewayRuntimeEvent(raw)
	if err != nil {
		a.safeLog("decode gateway event failed: %v", err)
		return
	}
	chatID := a.lookupChatID(sessionID, runID)
	if chatID == "" {
		return
	}
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "run_progress":
		if envelope != nil {
			if runtimeType := readString(envelope, "runtime_event_type"); runtimeType != "" {
				if strings.EqualFold(runtimeType, "permission_requested") {
					requestID, reason := extractPermissionRequest(envelope)
					if requestID != "" {
						_ = a.messenger.SendPermissionCard(ctx, chatID, PermissionCardPayload{
							RequestID: requestID,
							Message:   reason,
						})
						return
					}
				}
			}
		}
		// 除审批请求外，内部 runtime_event_type 不直接透出到飞书用户视图，避免暴露控制面细节。
		return
	case "run_done":
		doneText := extractUserVisibleDoneText(envelope)
		if doneText == "" {
			doneText = "任务完成。"
		}
		_ = a.messenger.SendText(ctx, chatID, doneText)
		a.untrackRun(sessionID, runID)
	case "run_error":
		errText := extractUserVisibleErrorText(envelope)
		if errText == "" {
			errText = "任务失败，请稍后重试。"
		}
		_ = a.messenger.SendText(ctx, chatID, errText)
		a.untrackRun(sessionID, runID)
	}
}

// reconnectAndRebindLoop 定期保活网关连接，并在重连后重绑活跃会话。
func (a *Adapter) reconnectAndRebindLoop(ctx context.Context) {
	delay := a.cfg.ReconnectBackoffMin
	ticker := time.NewTicker(a.cfg.RebindInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			callCtx, cancel := context.WithTimeout(ctx, a.cfg.RequestTimeout)
			err := a.gateway.Ping(callCtx)
			cancel()
			if err == nil {
				delay = a.cfg.ReconnectBackoffMin
				continue
			}
			a.safeLog("gateway ping failed, will reconnect: %v", err)
			if !a.retryAuthenticateAndRebind(ctx, delay) {
				return
			}
			delay = nextBackoff(delay, a.cfg.ReconnectBackoffMax)
		}
	}
}

// retryAuthenticateAndRebind 在连接异常后执行一次认证重试与会话重绑。
func (a *Adapter) retryAuthenticateAndRebind(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delayWithJitter(delay))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
	}
	callCtx, cancel := context.WithTimeout(ctx, a.cfg.RequestTimeout)
	defer cancel()
	if err := a.gateway.Authenticate(callCtx); err != nil {
		a.safeLog("gateway re-authenticate failed: %v", err)
		return true
	}
	a.rebindActiveSessions(callCtx)
	return true
}

// rebindActiveSessions 对当前活跃会话重新执行 bindStream，恢复事件订阅关系。
func (a *Adapter) rebindActiveSessions(ctx context.Context) {
	a.mu.RLock()
	snapshot := make([]sessionBinding, 0, len(a.activeRuns))
	for _, binding := range a.activeRuns {
		snapshot = append(snapshot, binding)
	}
	a.mu.RUnlock()

	for _, binding := range snapshot {
		callCtx, cancel := context.WithTimeout(ctx, a.cfg.RequestTimeout)
		err := a.gateway.BindStream(callCtx, binding.SessionID, binding.RunID)
		cancel()
		if err != nil {
			a.safeLog("rebind session failed session_id=%s run_id=%s err=%v", binding.SessionID, binding.RunID, err)
		}
	}
}

// shouldEmitProgress 控制普通运行进度消息推送频率，避免飞书侧刷屏。
func (a *Adapter) shouldEmitProgress(sessionID string, runID string, runtimeEventType string) bool {
	key := sessionID + "|" + runID + "|" + strings.TrimSpace(strings.ToLower(runtimeEventType))
	now := a.nowFn()
	a.mu.Lock()
	defer a.mu.Unlock()
	last, ok := a.lastProgressAt[key]
	if ok && now.Sub(last) < defaultProgressNotifyInterval {
		return false
	}
	a.lastProgressAt[key] = now
	return true
}

// isMentionCurrentBot 判断群聊消息是否明确 @ 到当前机器人应用。
func isMentionCurrentBot(event FeishuMessageEvent, configuredAppID string) bool {
	expected := strings.TrimSpace(strings.ToLower(configuredAppID))
	if expected == "" {
		expected = strings.TrimSpace(strings.ToLower(event.HeaderAppID))
	}
	if expected == "" {
		return false
	}
	for _, mention := range event.Mentions {
		for _, candidate := range []string{mention.AppID, mention.UserID, mention.OpenID, mention.UnionID} {
			if strings.TrimSpace(strings.ToLower(candidate)) == expected {
				return true
			}
		}
	}
	normalizedText := strings.TrimSpace(strings.ToLower(event.ContentText))
	return strings.Contains(normalizedText, `<at user_id="`+expected+`"`) ||
		strings.Contains(normalizedText, `<at user_id='`+expected+`'`) ||
		strings.Contains(normalizedText, `<at id="`+expected+`"`) ||
		strings.Contains(normalizedText, `<at id='`+expected+`'`)
}

// tryHandleTextPermission 处理 SDK 模式下的文本审批降级指令。
func (a *Adapter) tryHandleTextPermission(ctx context.Context, chatID string, text string) (bool, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false, nil
	}
	normalized := strings.ToLower(trimmed)
	switch {
	case strings.HasPrefix(normalized, "允许 "):
		requestID := strings.TrimSpace(trimmed[len("允许 "):])
		if requestID == "" {
			return true, nil
		}
		err := a.HandleCardAction(ctx, FeishuCardActionEvent{RequestID: requestID, Decision: "allow_once"})
		if err != nil {
			_ = a.messenger.SendText(context.Background(), chatID, "审批提交失败，请稍后重试。")
			return true, err
		}
		_ = a.messenger.SendText(context.Background(), chatID, "审批已提交：允许一次。")
		return true, nil
	case strings.HasPrefix(normalized, "拒绝 "):
		requestID := strings.TrimSpace(trimmed[len("拒绝 "):])
		if requestID == "" {
			return true, nil
		}
		err := a.HandleCardAction(ctx, FeishuCardActionEvent{RequestID: requestID, Decision: "reject"})
		if err != nil {
			_ = a.messenger.SendText(context.Background(), chatID, "审批提交失败，请稍后重试。")
			return true, err
		}
		_ = a.messenger.SendText(context.Background(), chatID, "审批已提交：拒绝。")
		return true, nil
	default:
		return false, nil
	}
}

// runBindingKey 生成稳定的 session/run 复合键，避免同会话多 run 相互覆盖。
func runBindingKey(sessionID string, runID string) string {
	return strings.TrimSpace(sessionID) + "|" + strings.TrimSpace(runID)
}

// decodeMessageText 从飞书消息 content JSON 中提取文本内容。
func decodeMessageText(rawContent string) (string, error) {
	trimmed := strings.TrimSpace(rawContent)
	if trimmed == "" {
		return "", nil
	}
	var payload inboundMessageContent
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.Text), nil
}

// extractPermissionRequest 从 permission_requested 事件中抽取审批请求关键信息。
func extractPermissionRequest(envelope map[string]any) (string, string) {
	payload, _ := envelope["payload"].(map[string]any)
	if payload == nil {
		return "", "需要审批"
	}
	requestID := readString(payload, "request_id")
	reason := readString(payload, "reason")
	if reason == "" {
		reason = "工具执行请求审批，请确认是否放行。"
	}
	return requestID, reason
}

// extractUserVisibleDoneText 从 run_done 事件中提取可展示给飞书用户的最终文本。
func extractUserVisibleDoneText(envelope map[string]any) string {
	if envelope == nil {
		return ""
	}
	payload, _ := envelope["payload"].(map[string]any)
	if payload == nil {
		return ""
	}
	if text := strings.TrimSpace(readString(payload, "content")); text != "" {
		return text
	}
	if text := strings.TrimSpace(readString(payload, "text")); text != "" {
		return text
	}
	parts, _ := payload["parts"].([]any)
	if len(parts) == 0 {
		return ""
	}
	lines := make([]string, 0, len(parts))
	for _, raw := range parts {
		part, _ := raw.(map[string]any)
		if part == nil {
			continue
		}
		partType := strings.TrimSpace(strings.ToLower(readString(part, "type")))
		if partType != "" && partType != "text" {
			continue
		}
		text := strings.TrimSpace(readString(part, "text"))
		if text == "" {
			text = strings.TrimSpace(readString(part, "content"))
		}
		if text != "" {
			lines = append(lines, text)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// extractUserVisibleErrorText 从 run_error 事件中提取对用户友好的失败摘要。
func extractUserVisibleErrorText(envelope map[string]any) string {
	if envelope == nil {
		return ""
	}
	payload, _ := envelope["payload"].(map[string]any)
	if payload == nil {
		return ""
	}
	message := strings.TrimSpace(readString(payload, "message"))
	if message == "" {
		message = strings.TrimSpace(readString(payload, "error"))
	}
	if message == "" {
		return ""
	}
	return "任务失败：" + message
}

// nextBackoff 计算指数退避下一步等待时间。
func nextBackoff(current time.Duration, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	return next
}

// delayWithJitter 为退避时间添加轻量随机抖动，减少重连风暴。
func delayWithJitter(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 200 * time.Millisecond
	}
	span := int64(delay / 4)
	if span <= 0 {
		return delay
	}
	jitter := rand.Int63n(span)
	return delay - time.Duration(span/2) + time.Duration(jitter)
}

// writeJSON 向回调响应写入 JSON 内容。
func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

// safeLog 输出适配器日志，并避免 nil logger 导致 panic。
func (a *Adapter) safeLog(format string, args ...any) {
	if a.logger == nil {
		return
	}
	a.logger.Printf(format, args...)
}
