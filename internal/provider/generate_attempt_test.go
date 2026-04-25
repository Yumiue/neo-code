package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	providertypes "neo-code/internal/provider/types"
)

func TestRunGenerateWithRetryUsingRetriesBeforePayloadStarts(t *testing.T) {
	t.Parallel()

	cfg := RuntimeConfig{
		GenerateMaxRetries:   2,
		GenerateStartTimeout: time.Second,
		GenerateIdleTimeout:  time.Second,
	}
	events := make(chan providertypes.StreamEvent, 8)
	attempts := 0

	err := RunGenerateWithRetryUsing(
		context.Background(),
		cfg,
		events,
		func(int) time.Duration { return 0 },
		func(context.Context, time.Duration) error { return nil },
		func(ctx context.Context, attemptEvents chan<- providertypes.StreamEvent) error {
			attempts++
			if attempts < 3 {
				return NewProviderErrorFromStatus(500, "temporary")
			}
			if emitErr := EmitTextDelta(ctx, attemptEvents, "ok"); emitErr != nil {
				return emitErr
			}
			return EmitMessageDone(ctx, attemptEvents, "stop", nil)
		},
	)
	if err != nil {
		t.Fatalf("RunGenerateWithRetryUsing() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}

	drained := drainAttemptEvents(events)
	if len(drained) != 2 {
		t.Fatalf("expected success events to be forwarded once, got %+v", drained)
	}
}

func TestRunGenerateWithRetryUsingDoesNotRetryAfterPayloadStarts(t *testing.T) {
	t.Parallel()

	cfg := RuntimeConfig{
		GenerateMaxRetries:   3,
		GenerateStartTimeout: time.Second,
		GenerateIdleTimeout:  time.Second,
	}
	events := make(chan providertypes.StreamEvent, 8)
	attempts := 0

	err := RunGenerateWithRetryUsing(
		context.Background(),
		cfg,
		events,
		func(int) time.Duration { return 0 },
		func(context.Context, time.Duration) error { return nil },
		func(ctx context.Context, attemptEvents chan<- providertypes.StreamEvent) error {
			attempts++
			if emitErr := EmitTextDelta(ctx, attemptEvents, "partial"); emitErr != nil {
				return emitErr
			}
			return NewProviderErrorFromStatus(500, "temporary")
		},
	)
	if err == nil {
		t.Fatal("expected error after payload-started failure")
	}
	if attempts != 1 {
		t.Fatalf("expected exactly 1 attempt, got %d", attempts)
	}
}

func TestRunGenerateWithRetryUsingRetriesOnStartTimeout(t *testing.T) {
	t.Parallel()

	cfg := RuntimeConfig{
		GenerateMaxRetries:   1,
		GenerateStartTimeout: 20 * time.Millisecond,
		GenerateIdleTimeout:  time.Second,
	}
	events := make(chan providertypes.StreamEvent, 8)
	attempts := 0

	err := RunGenerateWithRetryUsing(
		context.Background(),
		cfg,
		events,
		func(int) time.Duration { return 0 },
		func(context.Context, time.Duration) error { return nil },
		func(ctx context.Context, attemptEvents chan<- providertypes.StreamEvent) error {
			attempts++
			if attempts == 1 {
				<-ctx.Done()
				return ctx.Err()
			}
			if emitErr := EmitTextDelta(ctx, attemptEvents, "ok"); emitErr != nil {
				return emitErr
			}
			return EmitMessageDone(ctx, attemptEvents, "stop", nil)
		},
	)
	if err != nil {
		t.Fatalf("RunGenerateWithRetryUsing() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected retry after start timeout, got %d attempts", attempts)
	}
}

func TestRunGenerateWithRetryUsingStopsOnIdleTimeout(t *testing.T) {
	t.Parallel()

	cfg := RuntimeConfig{
		GenerateMaxRetries:   3,
		GenerateStartTimeout: time.Second,
		GenerateIdleTimeout:  20 * time.Millisecond,
	}
	events := make(chan providertypes.StreamEvent, 8)
	attempts := 0

	err := RunGenerateWithRetryUsing(
		context.Background(),
		cfg,
		events,
		func(int) time.Duration { return 0 },
		func(context.Context, time.Duration) error { return nil },
		func(ctx context.Context, attemptEvents chan<- providertypes.StreamEvent) error {
			attempts++
			if emitErr := EmitTextDelta(ctx, attemptEvents, "partial"); emitErr != nil {
				return emitErr
			}
			<-ctx.Done()
			return ctx.Err()
		},
	)
	if !errors.Is(err, ErrGenerateIdleTimeout) {
		t.Fatalf("expected idle timeout error, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected idle timeout to stop retries, got %d attempts", attempts)
	}
}

func TestRunGenerateWithRetryUsingDoesNotStartTimeoutAfterMessageDone(t *testing.T) {
	t.Parallel()

	cfg := RuntimeConfig{
		GenerateMaxRetries:   1,
		GenerateStartTimeout: 20 * time.Millisecond,
		GenerateIdleTimeout:  time.Second,
	}
	events := make(chan providertypes.StreamEvent, 8)

	err := RunGenerateWithRetryUsing(
		context.Background(),
		cfg,
		events,
		func(int) time.Duration { return 0 },
		func(context.Context, time.Duration) error { return nil },
		func(ctx context.Context, attemptEvents chan<- providertypes.StreamEvent) error {
			if emitErr := EmitMessageDone(ctx, attemptEvents, "stop", nil); emitErr != nil {
				return emitErr
			}
			time.Sleep(40 * time.Millisecond)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("RunGenerateWithRetryUsing() error = %v", err)
	}
}

func TestRunGenerateWithRetryUsingDrainsEventsAfterTimeoutCancel(t *testing.T) {
	t.Parallel()

	cfg := RuntimeConfig{
		GenerateMaxRetries:   0,
		GenerateStartTimeout: 20 * time.Millisecond,
		GenerateIdleTimeout:  time.Second,
	}
	events := make(chan providertypes.StreamEvent, 8)

	done := make(chan error, 1)
	go func() {
		done <- RunGenerateWithRetryUsing(
			context.Background(),
			cfg,
			events,
			func(int) time.Duration { return 0 },
			func(context.Context, time.Duration) error { return nil },
			func(ctx context.Context, attemptEvents chan<- providertypes.StreamEvent) error {
				<-ctx.Done()
				for i := 0; i < 128; i++ {
					if err := EmitTextDelta(ctx, attemptEvents, "ignored"); err != nil {
						return err
					}
				}
				return ctx.Err()
			},
		)
	}()

	select {
	case err := <-done:
		if !errors.Is(err, ErrGenerateStartTimeout) {
			t.Fatalf("expected start timeout, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected generate attempt to return without deadlock")
	}
}

func TestRunGenerateWithRetryUsingTreatsMessageDoneAsCompletedState(t *testing.T) {
	t.Parallel()

	cfg := RuntimeConfig{
		GenerateMaxRetries:   1,
		GenerateStartTimeout: time.Second,
		GenerateIdleTimeout:  time.Second,
	}
	events := make(chan providertypes.StreamEvent, 8)

	err := RunGenerateWithRetryUsing(
		context.Background(),
		cfg,
		events,
		func(int) time.Duration { return 0 },
		func(context.Context, time.Duration) error { return nil },
		func(ctx context.Context, attemptEvents chan<- providertypes.StreamEvent) error {
			if emitErr := EmitMessageDone(ctx, attemptEvents, "stop", nil); emitErr != nil {
				return emitErr
			}
			<-ctx.Done()
			return ctx.Err()
		},
	)
	if err != nil {
		t.Fatalf("expected completed attempt to ignore trailing cancellation, got %v", err)
	}

	drained := drainAttemptEvents(events)
	if len(drained) != 1 || drained[0].Type != providertypes.StreamEventMessageDone {
		t.Fatalf("expected only message_done to be forwarded, got %+v", drained)
	}
}

func drainAttemptEvents(events <-chan providertypes.StreamEvent) []providertypes.StreamEvent {
	out := make([]providertypes.StreamEvent, 0, len(events))
	for {
		select {
		case event := <-events:
			out = append(out, event)
		default:
			return out
		}
	}
}
