package feishuadapter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

type sdkEventClient interface {
	Start(ctx context.Context) error
}

var newSDKEventClient = func(cfg Config, handler IngressHandler) sdkEventClient {
	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			msg, ok := mapSDKMessageEvent(event)
			if !ok {
				return nil
			}
			return handler.HandleMessage(ctx, msg)
		}).
		OnCustomizedEvent("card.action.trigger", func(ctx context.Context, event *larkevent.EventReq) error {
			cardEvent, ok := mapSDKCardActionEvent(event)
			if !ok {
				return nil
			}
			return handler.HandleCardAction(ctx, cardEvent)
		})

	client := larkws.NewClient(cfg.AppID, cfg.AppSecret, larkws.WithEventHandler(eventHandler))
	return client
}

// SDKIngress 基于飞书 SDK 长连接接收事件，适合本地无公网接入场景。
type SDKIngress struct {
	cfg    Config
	logger func(string, ...any)
}

// NewSDKIngress 创建 SDK 长连接入站实现。
func NewSDKIngress(cfg Config, logger func(string, ...any)) Ingress {
	if logger == nil {
		logger = func(string, ...any) {}
	}
	return &SDKIngress{
		cfg:    cfg,
		logger: logger,
	}
}

// Run 建立 SDK 长连接并将接收到的事件转发到统一 handler。
func (s *SDKIngress) Run(ctx context.Context, handler IngressHandler) error {
	client := newSDKEventClient(s.cfg, handler)
	if client == nil {
		return fmt.Errorf("create sdk event client: nil client")
	}
	err := client.Start(ctx)
	if err != nil && err != context.Canceled {
		s.logger("feishu sdk ingress stopped with error: %v", err)
	}
	return err
}

// mapSDKMessageEvent 将 SDK 消息事件转换为标准化消息事件。
func mapSDKMessageEvent(event *larkim.P2MessageReceiveV1) (FeishuMessageEvent, bool) {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		return FeishuMessageEvent{}, false
	}
	message := event.Event.Message
	messageID := stringOrEmpty(message.MessageId)
	chatID := stringOrEmpty(message.ChatId)
	if strings.TrimSpace(messageID) == "" || strings.TrimSpace(chatID) == "" {
		return FeishuMessageEvent{}, false
	}

	messageEvent := FeishuMessageEvent{
		EventID:     headerEventID(event),
		MessageID:   messageID,
		ChatID:      chatID,
		ChatType:    stringOrEmpty(message.ChatType),
		ContentText: extractSDKMessageText(stringOrEmpty(message.Content)),
		HeaderAppID: headerAppID(event),
		Mentions:    mapSDKMentions(message.Mentions),
	}
	return messageEvent, true
}

// mapSDKCardActionEvent 尝试从 SDK 自定义事件中提取审批动作。
func mapSDKCardActionEvent(event *larkevent.EventReq) (FeishuCardActionEvent, bool) {
	if event == nil || len(event.Body) == 0 {
		return FeishuCardActionEvent{}, false
	}
	var payload struct {
		Header struct {
			EventID string `json:"event_id"`
		} `json:"header"`
		Event struct {
			Action struct {
				Value map[string]string `json:"value"`
			} `json:"action"`
		} `json:"event"`
	}
	if err := json.Unmarshal(event.Body, &payload); err != nil {
		return FeishuCardActionEvent{}, false
	}
	requestID := strings.TrimSpace(payload.Event.Action.Value["request_id"])
	decision := strings.TrimSpace(strings.ToLower(payload.Event.Action.Value["decision"]))
	if requestID == "" || (decision != "allow_once" && decision != "reject") {
		return FeishuCardActionEvent{}, false
	}
	return FeishuCardActionEvent{
		EventID:   strings.TrimSpace(payload.Header.EventID),
		RequestID: requestID,
		Decision:  decision,
	}, true
}

// mapSDKMentions 将 SDK mentions 映射为统一提及身份列表。
func mapSDKMentions(mentions []*larkim.MentionEvent) []FeishuMention {
	if len(mentions) == 0 {
		return nil
	}
	out := make([]FeishuMention, 0, len(mentions))
	for _, mention := range mentions {
		if mention == nil {
			continue
		}
		identity := mention.Id
		item := FeishuMention{}
		if identity != nil {
			item.UserID = stringOrEmpty(identity.UserId)
			item.OpenID = stringOrEmpty(identity.OpenId)
			item.UnionID = stringOrEmpty(identity.UnionId)
		}
		out = append(out, item)
	}
	return out
}

// headerEventID 提取 SDK 事件头中的 event_id。
func headerEventID(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.EventV2Base == nil || event.EventV2Base.Header == nil {
		return ""
	}
	return strings.TrimSpace(event.EventV2Base.Header.EventID)
}

// headerAppID 提取 SDK 事件头中的 app_id。
func headerAppID(event *larkim.P2MessageReceiveV1) string {
	if event == nil || event.EventV2Base == nil || event.EventV2Base.Header == nil {
		return ""
	}
	return strings.TrimSpace(event.EventV2Base.Header.AppID)
}

// extractSDKMessageText 从 SDK 消息 content JSON 里提取 text。
func extractSDKMessageText(content string) string {
	text, err := decodeMessageText(content)
	if err == nil {
		return text
	}
	return strings.TrimSpace(content)
}

// stringOrEmpty 安全解引用 SDK 字符串指针。
func stringOrEmpty(raw *string) string {
	if raw == nil {
		return ""
	}
	return strings.TrimSpace(*raw)
}
