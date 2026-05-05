package feishuadapter

import (
	"context"
	"testing"
	"time"
)

type fakeSDKEventClient struct {
	startFn func(ctx context.Context) error
}

func (f *fakeSDKEventClient) Start(ctx context.Context) error {
	if f.startFn != nil {
		return f.startFn(ctx)
	}
	return nil
}

type captureIngressHandler struct {
	messages []FeishuMessageEvent
	cards    []FeishuCardActionEvent
}

func (c *captureIngressHandler) HandleMessage(_ context.Context, event FeishuMessageEvent) error {
	c.messages = append(c.messages, event)
	return nil
}

func (c *captureIngressHandler) HandleCardAction(_ context.Context, event FeishuCardActionEvent) error {
	c.cards = append(c.cards, event)
	return nil
}

func TestSDKIngressRunDispatchesMessageToHandler(t *testing.T) {
	originalFactory := newSDKEventClient
	t.Cleanup(func() { newSDKEventClient = originalFactory })

	handler := &captureIngressHandler{}
	newSDKEventClient = func(cfg Config, ingressHandler IngressHandler) sdkEventClient {
		return &fakeSDKEventClient{
			startFn: func(ctx context.Context) error {
				return ingressHandler.HandleMessage(ctx, FeishuMessageEvent{
					EventID:     "evt-1",
					MessageID:   "msg-1",
					ChatID:      "chat-1",
					ChatType:    "p2p",
					ContentText: "hello",
					HeaderAppID: cfg.AppID,
				})
			},
		}
	}

	ingress := NewSDKIngress(Config{
		IngressMode:         IngressModeSDK,
		AppID:               "app",
		AppSecret:           "secret",
		RequestTimeout:      3 * time.Second,
		IdempotencyTTL:      time.Minute,
		ReconnectBackoffMin: time.Second,
		ReconnectBackoffMax: 2 * time.Second,
		RebindInterval:      3 * time.Second,
	}, nil)
	if err := ingress.Run(context.Background(), handler); err != nil {
		t.Fatalf("run sdk ingress: %v", err)
	}
	if len(handler.messages) != 1 {
		t.Fatalf("message count = %d, want 1", len(handler.messages))
	}
	if handler.messages[0].MessageID != "msg-1" {
		t.Fatalf("message id = %q, want msg-1", handler.messages[0].MessageID)
	}
}

func TestHandleMessageSDKPrivateChatTriggersRun(t *testing.T) {
	adapter := newTestAdapter(t)
	adapter.cfg.IngressMode = IngressModeSDK
	err := adapter.HandleMessage(context.Background(), FeishuMessageEvent{
		EventID:     "evt-p2p",
		MessageID:   "msg-p2p",
		ChatID:      "chat-p2p",
		ChatType:    "p2p",
		ContentText: "hello sdk",
		HeaderAppID: "app",
	})
	if err != nil {
		t.Fatalf("handle message: %v", err)
	}
	if adapterTestGateway(adapter).runCount != 1 {
		t.Fatalf("run count = %d, want 1", adapterTestGateway(adapter).runCount)
	}
}

func TestHandleMessageSDKGroupOnlyBotMentionTriggersRun(t *testing.T) {
	adapter := newTestAdapter(t)
	adapter.cfg.IngressMode = IngressModeSDK
	err := adapter.HandleMessage(context.Background(), FeishuMessageEvent{
		EventID:     "evt-group-no",
		MessageID:   "msg-group-no",
		ChatID:      "chat-group-no",
		ChatType:    "group",
		ContentText: "@alice hello",
		HeaderAppID: "app",
		Mentions: []FeishuMention{
			{UserID: "ou_other"},
		},
	})
	if err != nil {
		t.Fatalf("handle non-bot mention: %v", err)
	}
	if adapterTestGateway(adapter).runCount != 0 {
		t.Fatalf("run count = %d, want 0", adapterTestGateway(adapter).runCount)
	}

	err = adapter.HandleMessage(context.Background(), FeishuMessageEvent{
		EventID:     "evt-group-yes",
		MessageID:   "msg-group-yes",
		ChatID:      "chat-group-yes",
		ChatType:    "group",
		ContentText: "@neo hello",
		HeaderAppID: "app",
		Mentions: []FeishuMention{
			{UserID: "ou_bot"},
		},
	})
	if err != nil {
		t.Fatalf("handle bot mention: %v", err)
	}
	if adapterTestGateway(adapter).runCount != 1 {
		t.Fatalf("run count = %d, want 1", adapterTestGateway(adapter).runCount)
	}
}

func TestHandleMessageSDKDuplicateEventOnlyRunsOnce(t *testing.T) {
	adapter := newTestAdapter(t)
	adapter.cfg.IngressMode = IngressModeSDK
	event := FeishuMessageEvent{
		EventID:     "evt-dup",
		MessageID:   "msg-dup",
		ChatID:      "chat-dup",
		ChatType:    "p2p",
		ContentText: "hello sdk",
		HeaderAppID: "app",
	}
	if err := adapter.HandleMessage(context.Background(), event); err != nil {
		t.Fatalf("handle message first: %v", err)
	}
	if err := adapter.HandleMessage(context.Background(), event); err != nil {
		t.Fatalf("handle message second: %v", err)
	}
	if adapterTestGateway(adapter).runCount != 1 {
		t.Fatalf("run count = %d, want 1", adapterTestGateway(adapter).runCount)
	}
}

func TestHandleMessageSDKRunFailureCanRetry(t *testing.T) {
	adapter := newTestAdapter(t)
	adapter.cfg.IngressMode = IngressModeSDK
	gateway := adapterTestGateway(adapter)
	gateway.mu.Lock()
	gateway.runErr = assertErr("transient")
	gateway.runErrOnce = true
	gateway.mu.Unlock()

	event := FeishuMessageEvent{
		EventID:     "evt-retry-sdk",
		MessageID:   "msg-retry-sdk",
		ChatID:      "chat-retry-sdk",
		ChatType:    "p2p",
		ContentText: "hello sdk",
		HeaderAppID: "app",
	}
	_ = adapter.HandleMessage(context.Background(), event)
	if err := adapter.HandleMessage(context.Background(), event); err != nil {
		t.Fatalf("retry handle message: %v", err)
	}
	if adapterTestGateway(adapter).runCount != 2 {
		t.Fatalf("run count = %d, want 2", adapterTestGateway(adapter).runCount)
	}
}
