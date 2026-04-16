package gateway

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"neo-code/internal/gateway/protocol"
)

func TestDispatchRPCRequestResultEncodeError(t *testing.T) {
	originalHandlers := requestFrameHandlers
	requestFrameHandlers = map[FrameAction]requestFrameHandler{
		FrameActionPing: func(_ context.Context, frame MessageFrame) MessageFrame {
			return MessageFrame{
				Type:      FrameTypeAck,
				Action:    FrameActionPing,
				RequestID: frame.RequestID,
				Payload: map[string]any{
					"bad": make(chan int),
				},
			}
		},
	}
	t.Cleanup(func() {
		requestFrameHandlers = originalHandlers
	})

	response := dispatchRPCRequest(context.Background(), protocol.JSONRPCRequest{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      json.RawMessage(`"rpc-encode-1"`),
		Method:  protocol.MethodGatewayPing,
		Params:  json.RawMessage(`{}`),
	}, nil)
	if response.Error == nil {
		t.Fatal("expected jsonrpc internal error")
	}
	if response.Error.Code != protocol.JSONRPCCodeInternalError {
		t.Fatalf("rpc error code = %d, want %d", response.Error.Code, protocol.JSONRPCCodeInternalError)
	}
	if gatewayCode := protocol.GatewayCodeFromJSONRPCError(response.Error); gatewayCode != ErrorCodeInternalError.String() {
		t.Fatalf("gateway_code = %q, want %q", gatewayCode, ErrorCodeInternalError.String())
	}
}

func TestHydrateFrameSessionFromConnectionFallback(t *testing.T) {
	relay := NewStreamRelay(StreamRelayOptions{})
	baseContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	connectionID := NewConnectionID()
	connectionContext := WithConnectionID(baseContext, connectionID)
	connectionContext = WithStreamRelay(connectionContext, relay)
	if err := relay.RegisterConnection(ConnectionRegistration{
		ConnectionID: connectionID,
		Channel:      StreamChannelIPC,
		Context:      connectionContext,
		Cancel:       cancel,
		Write: func(message RelayMessage) error {
			_ = message
			return nil
		},
		Close: func() {},
	}); err != nil {
		t.Fatalf("register connection: %v", err)
	}
	defer relay.dropConnection(connectionID)

	if bindErr := relay.BindConnection(connectionID, StreamBinding{
		SessionID: "session-fallback",
		Channel:   StreamChannelAll,
		Explicit:  true,
	}); bindErr != nil {
		t.Fatalf("bind connection: %v", bindErr)
	}

	hydrated := hydrateFrameSessionFromConnection(connectionContext, MessageFrame{
		Type:   FrameTypeRequest,
		Action: FrameActionPing,
	})
	if hydrated.SessionID != "session-fallback" {
		t.Fatalf("session_id = %q, want %q", hydrated.SessionID, "session-fallback")
	}
}

func TestApplyAutomaticBindingPingRefreshesTTL(t *testing.T) {
	relay := NewStreamRelay(StreamRelayOptions{
		BindingTTL: 20 * time.Millisecond,
	})
	baseContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	connectionID := NewConnectionID()
	connectionContext := WithConnectionID(baseContext, connectionID)
	connectionContext = WithStreamRelay(connectionContext, relay)
	if err := relay.RegisterConnection(ConnectionRegistration{
		ConnectionID: connectionID,
		Channel:      StreamChannelIPC,
		Context:      connectionContext,
		Cancel:       cancel,
		Write: func(message RelayMessage) error {
			_ = message
			return nil
		},
		Close: func() {},
	}); err != nil {
		t.Fatalf("register connection: %v", err)
	}
	defer relay.dropConnection(connectionID)

	if bindErr := relay.BindConnection(connectionID, StreamBinding{
		SessionID: "session-ping",
		Channel:   StreamChannelAll,
		Explicit:  true,
	}); bindErr != nil {
		t.Fatalf("bind connection: %v", bindErr)
	}

	time.Sleep(10 * time.Millisecond)
	applyAutomaticBinding(connectionContext, MessageFrame{
		Type:   FrameTypeRequest,
		Action: FrameActionPing,
	})
	time.Sleep(15 * time.Millisecond)
	if !relay.RefreshConnectionBindings(connectionID) {
		t.Fatal("expected ping to refresh existing bindings")
	}
}

func TestDispatchFrameValidationBranches(t *testing.T) {
	response := dispatchFrame(context.Background(), MessageFrame{
		Type: FrameType("invalid"),
	}, nil)
	if response.Type != FrameTypeError {
		t.Fatalf("response type = %q, want %q", response.Type, FrameTypeError)
	}
	if response.Error == nil || response.Error.Code != ErrorCodeInvalidFrame.String() {
		t.Fatalf("response error = %#v, want invalid_frame", response.Error)
	}

	response = dispatchFrame(context.Background(), MessageFrame{
		Type:   FrameTypeEvent,
		Action: FrameActionPing,
	}, nil)
	if response.Type != FrameTypeError {
		t.Fatalf("response type = %q, want %q", response.Type, FrameTypeError)
	}
	if response.Error == nil || response.Error.Code != ErrorCodeInvalidFrame.String() {
		t.Fatalf("response error = %#v, want invalid_frame", response.Error)
	}
}

func TestDispatchRPCRequestUnauthorizedAndAccessDenied(t *testing.T) {
	authenticator := staticTokenAuthenticator{token: "t-1"}
	authState := NewConnectionAuthState()
	baseContext := WithRequestSource(context.Background(), RequestSourceHTTP)
	baseContext = WithTokenAuthenticator(baseContext, authenticator)
	baseContext = WithConnectionAuthState(baseContext, authState)
	baseContext = WithRequestACL(baseContext, NewStrictControlPlaneACL())

	unauthorizedResponse := dispatchRPCRequest(baseContext, protocol.JSONRPCRequest{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      json.RawMessage(`"req-unauthorized"`),
		Method:  protocol.MethodGatewayPing,
		Params:  json.RawMessage(`{}`),
	}, nil)
	if unauthorizedResponse.Error == nil {
		t.Fatal("expected unauthorized response")
	}
	if gatewayCode := protocol.GatewayCodeFromJSONRPCError(unauthorizedResponse.Error); gatewayCode != ErrorCodeUnauthorized.String() {
		t.Fatalf("gateway_code = %q, want %q", gatewayCode, ErrorCodeUnauthorized.String())
	}

	deniedACL := &ControlPlaneACL{
		mode:    ACLModeStrict,
		allow:   map[RequestSource]map[string]struct{}{},
		enabled: true,
	}
	deniedContext := WithRequestACL(baseContext, deniedACL)
	deniedContext = WithRequestToken(deniedContext, "t-1")
	deniedContext = WithConnectionAuthState(deniedContext, authState)
	authState.MarkAuthenticated()

	deniedResponse := dispatchRPCRequest(deniedContext, protocol.JSONRPCRequest{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      json.RawMessage(`"req-denied"`),
		Method:  protocol.MethodGatewayPing,
		Params:  json.RawMessage(`{}`),
	}, nil)
	if deniedResponse.Error == nil {
		t.Fatal("expected access denied response")
	}
	if gatewayCode := protocol.GatewayCodeFromJSONRPCError(deniedResponse.Error); gatewayCode != ErrorCodeAccessDenied.String() {
		t.Fatalf("gateway_code = %q, want %q", gatewayCode, ErrorCodeAccessDenied.String())
	}
}

func TestDispatchRPCRequestAuthenticateThenPing(t *testing.T) {
	authenticator := staticTokenAuthenticator{token: "token-2"}
	authState := NewConnectionAuthState()
	ctx := WithRequestSource(context.Background(), RequestSourceIPC)
	ctx = WithTokenAuthenticator(ctx, authenticator)
	ctx = WithConnectionAuthState(ctx, authState)
	ctx = WithRequestACL(ctx, NewStrictControlPlaneACL())

	authResponse := dispatchRPCRequest(ctx, protocol.JSONRPCRequest{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      json.RawMessage(`"req-auth"`),
		Method:  protocol.MethodGatewayAuthenticate,
		Params:  json.RawMessage(`{"token":"token-2"}`),
	}, nil)
	if authResponse.Error != nil {
		t.Fatalf("authenticate response error: %+v", authResponse.Error)
	}
	authFrame, err := decodeJSONRPCResultFrame(authResponse)
	if err != nil {
		t.Fatalf("decode auth frame: %v", err)
	}
	if authFrame.Action != FrameActionAuthenticate {
		t.Fatalf("auth action = %q, want %q", authFrame.Action, FrameActionAuthenticate)
	}

	pingResponse := dispatchRPCRequest(ctx, protocol.JSONRPCRequest{
		JSONRPC: protocol.JSONRPCVersion,
		ID:      json.RawMessage(`"req-ping"`),
		Method:  protocol.MethodGatewayPing,
		Params:  json.RawMessage(`{}`),
	}, nil)
	if pingResponse.Error != nil {
		t.Fatalf("ping response error: %+v", pingResponse.Error)
	}
	pingFrame, err := decodeJSONRPCResultFrame(pingResponse)
	if err != nil {
		t.Fatalf("decode ping frame: %v", err)
	}
	if pingFrame.Action != FrameActionPing {
		t.Fatalf("ping action = %q, want %q", pingFrame.Action, FrameActionPing)
	}
	payloadMap, ok := pingFrame.Payload.(map[string]any)
	if !ok {
		t.Fatalf("ping payload type = %T, want map[string]any", pingFrame.Payload)
	}
	version, _ := payloadMap["version"].(string)
	if strings.TrimSpace(version) == "" {
		t.Fatal("ping payload should include version")
	}
}
