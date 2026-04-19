package runtime

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	providertypes "neo-code/internal/provider/types"
	"neo-code/internal/runtime/controlplane"
	agentsession "neo-code/internal/session"
	"neo-code/internal/subagent"
	"neo-code/internal/tools"
	todotool "neo-code/internal/tools/todo"
)

func TestDispatchTodosExecutesSubAgentTasks(t *testing.T) {
	t.Parallel()

	manager := newRuntimeConfigManagerWithProviderEnvs(t, nil)
	store := newMemoryStore()
	service := NewWithFactory(
		manager,
		tools.NewRegistry(),
		store,
		&scriptedProviderFactory{provider: &scriptedProvider{}},
		&stubContextBuilder{},
	)
	service.SetSubAgentFactory(newSuccessSubAgentFactory())

	session := agentsession.New("dispatch-session")
	session.Workdir = manager.Get().Workdir
	if err := session.ReplaceTodos([]agentsession.TodoItem{
		{
			ID:       "a",
			Content:  "task-a",
			Executor: agentsession.TodoExecutorSubAgent,
			Priority: 2,
		},
		{
			ID:           "b",
			Content:      "task-b",
			Executor:     agentsession.TodoExecutorSubAgent,
			Dependencies: []string{"a"},
			Priority:     1,
		},
	}); err != nil {
		t.Fatalf("ReplaceTodos() error = %v", err)
	}
	saveSessionToMemoryStore(store, session)

	state := newRunState("run-dispatch", session)
	state.turn = 1
	state.phase = controlplane.PhaseDispatch
	progressed, err := service.dispatchTodos(context.Background(), &state, turnSnapshot{workdir: session.Workdir})
	if err != nil {
		t.Fatalf("dispatchTodos() error = %v", err)
	}
	if !progressed {
		t.Fatalf("dispatchTodos() progressed = false, want true")
	}

	a, ok := state.session.FindTodo("a")
	if !ok || a.Status != agentsession.TodoStatusCompleted {
		t.Fatalf("todo a = %+v, want completed", a)
	}
	b, ok := state.session.FindTodo("b")
	if !ok || b.Status != agentsession.TodoStatusCompleted {
		t.Fatalf("todo b = %+v, want completed", b)
	}
	if len(b.Artifacts) == 0 {
		t.Fatalf("todo b artifacts should not be empty")
	}

	events := collectRuntimeEvents(service.Events())
	assertEventContains(t, events, EventSubAgentCompleted)
	assertEventContains(t, events, EventSubAgentFinished)
}

func TestDispatchTodosRetriesTransientSubAgentFailureInSameRound(t *testing.T) {
	t.Parallel()

	manager := newRuntimeConfigManagerWithProviderEnvs(t, nil)
	store := newMemoryStore()
	service := NewWithFactory(
		manager,
		tools.NewRegistry(),
		store,
		&scriptedProviderFactory{provider: &scriptedProvider{}},
		&stubContextBuilder{},
	)
	service.SetSubAgentFactory(newFailOnceThenSuccessSubAgentFactory())

	session := agentsession.New("dispatch-retry-once")
	session.Workdir = manager.Get().Workdir
	if err := session.ReplaceTodos([]agentsession.TodoItem{
		{
			ID:       "retry-once",
			Content:  "transient failure should auto retry",
			Executor: agentsession.TodoExecutorSubAgent,
		},
	}); err != nil {
		t.Fatalf("ReplaceTodos() error = %v", err)
	}
	saveSessionToMemoryStore(store, session)

	state := newRunState("run-dispatch-retry-once", session)
	state.turn = 1
	state.phase = controlplane.PhaseDispatch
	progressed, err := service.dispatchTodos(context.Background(), &state, turnSnapshot{workdir: session.Workdir})
	if err != nil {
		t.Fatalf("dispatchTodos() error = %v", err)
	}
	if !progressed {
		t.Fatalf("dispatchTodos() progressed = false, want true")
	}

	task, ok := state.session.FindTodo("retry-once")
	if !ok {
		t.Fatalf("todo retry-once not found")
	}
	if task.Status != agentsession.TodoStatusCompleted {
		t.Fatalf("todo retry-once status = %q, want completed", task.Status)
	}

	events := collectRuntimeEvents(service.Events())
	assertEventContains(t, events, EventSubAgentRetried)
	assertEventContains(t, events, EventSubAgentCompleted)
	assertEventContains(t, events, EventSubAgentFinished)
}

func TestDispatchTodosSkipsAgentOwnedTodos(t *testing.T) {
	t.Parallel()

	manager := newRuntimeConfigManagerWithProviderEnvs(t, nil)
	store := newMemoryStore()
	service := NewWithFactory(
		manager,
		tools.NewRegistry(),
		store,
		&scriptedProviderFactory{provider: &scriptedProvider{}},
		&stubContextBuilder{},
	)

	session := agentsession.New("dispatch-skip")
	if err := session.ReplaceTodos([]agentsession.TodoItem{
		{
			ID:       "agent-task",
			Content:  "handled by agent",
			Executor: agentsession.TodoExecutorAgent,
		},
	}); err != nil {
		t.Fatalf("ReplaceTodos() error = %v", err)
	}
	state := newRunState("run-dispatch-skip", session)
	state.phase = controlplane.PhaseDispatch
	progressed, err := service.dispatchTodos(context.Background(), &state, turnSnapshot{})
	if err != nil {
		t.Fatalf("dispatchTodos() error = %v", err)
	}
	if progressed {
		t.Fatalf("dispatchTodos() progressed = true, want false")
	}

	task, ok := state.session.FindTodo("agent-task")
	if !ok {
		t.Fatalf("FindTodo(agent-task) not found")
	}
	if task.Status != agentsession.TodoStatusPending {
		t.Fatalf("status = %q, want pending", task.Status)
	}
	events := collectRuntimeEvents(service.Events())
	if len(events) != 0 {
		t.Fatalf("expected no dispatch events for agent-owned todos, got %d", len(events))
	}
}

func TestDispatchTodosUsesExtendedDefaultTaskTimeout(t *testing.T) {
	t.Parallel()

	manager := newRuntimeConfigManagerWithProviderEnvs(t, nil)
	store := newMemoryStore()
	service := NewWithFactory(
		manager,
		tools.NewRegistry(),
		store,
		&scriptedProviderFactory{provider: &scriptedProvider{}},
		&stubContextBuilder{},
	)

	var (
		mu             sync.Mutex
		capturedBudget time.Duration
	)
	service.SetSubAgentFactory(subagent.NewWorkerFactory(func(role subagent.Role, policy subagent.RolePolicy) subagent.Engine {
		_ = role
		_ = policy
		return subagent.EngineFunc(func(ctx context.Context, input subagent.StepInput) (subagent.StepOutput, error) {
			_ = ctx
			mu.Lock()
			capturedBudget = input.Budget.Timeout
			mu.Unlock()
			return subagent.StepOutput{
				Done: true,
				Output: subagent.Output{
					Summary:     "done",
					Findings:    []string{"ok"},
					Patches:     []string{"none"},
					Risks:       []string{"low"},
					NextActions: []string{"continue"},
					Artifacts:   []string{"timeout-check.artifact"},
				},
			}, nil
		})
	}))

	session := agentsession.New("dispatch-timeout-budget")
	session.Workdir = manager.Get().Workdir
	if err := session.ReplaceTodos([]agentsession.TodoItem{
		{
			ID:       "sub-timeout",
			Content:  "validate timeout",
			Executor: agentsession.TodoExecutorSubAgent,
		},
	}); err != nil {
		t.Fatalf("ReplaceTodos(session) error = %v", err)
	}
	saveSessionToMemoryStore(store, session)

	state := newRunState("run-dispatch-timeout-budget", session)
	state.phase = controlplane.PhaseDispatch
	progressed, err := service.dispatchTodos(context.Background(), &state, turnSnapshot{workdir: session.Workdir})
	if err != nil {
		t.Fatalf("dispatchTodos() error = %v", err)
	}
	if !progressed {
		t.Fatalf("dispatchTodos() progressed = false, want true")
	}

	mu.Lock()
	timeout := capturedBudget
	mu.Unlock()
	if timeout != defaultSubAgentDispatchTaskTimeout {
		t.Fatalf("captured timeout = %v, want %v", timeout, defaultSubAgentDispatchTaskTimeout)
	}
}

func TestRunAutoDispatchesSubAgentTodosFromTodoWrite(t *testing.T) {
	t.Parallel()

	manager := newRuntimeConfigManagerWithProviderEnvs(t, nil)
	store := newMemoryStore()
	scripted := &scriptedProvider{
		responses: []scriptedResponse{
			{
				Message: providertypes.Message{
					Role: providertypes.RoleAssistant,
					ToolCalls: []providertypes.ToolCall{
						{
							ID:        "todo-plan-1",
							Name:      tools.ToolNameTodoWrite,
							Arguments: `{"action":"plan","items":[{"id":"sub-1","content":"run sub agent","executor":"subagent"}]}`,
						},
					},
				},
				FinishReason: "tool_calls",
			},
			{
				Message: providertypes.Message{
					Role:  providertypes.RoleAssistant,
					Parts: []providertypes.ContentPart{providertypes.NewTextPart("all done")},
				},
			},
		},
	}
	service := NewWithFactory(
		manager,
		func() tools.Manager {
			registry := tools.NewRegistry()
			registry.Register(todotool.New())
			return registry
		}(),
		store,
		&scriptedProviderFactory{provider: scripted},
		&stubContextBuilder{},
	)
	service.SetSubAgentFactory(newSuccessSubAgentFactory())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := service.Run(ctx, UserInput{
		RunID: "run-auto-dispatch",
		Parts: []providertypes.ContentPart{providertypes.NewTextPart("start")},
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	session := firstSessionFromMemoryStore(t, store)
	task, ok := session.FindTodo("sub-1")
	if !ok {
		t.Fatalf("todo sub-1 not found")
	}
	if task.Status != agentsession.TodoStatusCompleted {
		t.Fatalf("todo sub-1 status = %q, want completed", task.Status)
	}
	if len(task.Artifacts) == 0 {
		t.Fatalf("todo sub-1 artifacts should not be empty")
	}

	events := collectRuntimeEvents(service.Events())
	assertEventContains(t, events, EventSubAgentStarted)
	assertEventContains(t, events, EventSubAgentCompleted)
	assertEventContains(t, events, EventSubAgentFinished)
}

func TestRunAutoDispatchesExistingSubAgentTodosWithoutToolCalls(t *testing.T) {
	t.Parallel()

	manager := newRuntimeConfigManagerWithProviderEnvs(t, nil)
	store := newMemoryStore()
	scripted := &scriptedProvider{
		responses: []scriptedResponse{
			{
				Message: providertypes.Message{
					Role:  providertypes.RoleAssistant,
					Parts: []providertypes.ContentPart{providertypes.NewTextPart("skip direct tools")},
				},
			},
			{
				Message: providertypes.Message{
					Role:  providertypes.RoleAssistant,
					Parts: []providertypes.ContentPart{providertypes.NewTextPart("all done")},
				},
			},
		},
	}
	service := NewWithFactory(
		manager,
		tools.NewRegistry(),
		store,
		&scriptedProviderFactory{provider: scripted},
		&stubContextBuilder{},
	)
	service.SetSubAgentFactory(newSuccessSubAgentFactory())

	seed := agentsession.New("dispatch-seeded")
	seed.Workdir = manager.Get().Workdir
	if err := seed.ReplaceTodos([]agentsession.TodoItem{
		{
			ID:       "seed-sub-1",
			Content:  "run from existing todo",
			Executor: agentsession.TodoExecutorSubAgent,
		},
	}); err != nil {
		t.Fatalf("ReplaceTodos(seed) error = %v", err)
	}
	saveSessionToMemoryStore(store, seed)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := service.Run(ctx, UserInput{
		SessionID: seed.ID,
		RunID:     "run-auto-dispatch-existing",
		Parts:     []providertypes.ContentPart{providertypes.NewTextPart("continue")},
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	session := firstSessionFromMemoryStore(t, store)
	task, ok := session.FindTodo("seed-sub-1")
	if !ok {
		t.Fatalf("todo seed-sub-1 not found")
	}
	if task.Status != agentsession.TodoStatusCompleted {
		t.Fatalf("todo seed-sub-1 status = %q, want completed", task.Status)
	}

	events := collectRuntimeEvents(service.Events())
	assertEventContains(t, events, EventSubAgentStarted)
	assertEventContains(t, events, EventSubAgentCompleted)
	assertEventContains(t, events, EventSubAgentFinished)
}

func TestRunKeepsDrivingAgentPathForMixedExecutorDependencies(t *testing.T) {
	t.Parallel()

	manager := newRuntimeConfigManagerWithProviderEnvs(t, nil)
	store := newMemoryStore()
	scripted := &scriptedProvider{
		responses: []scriptedResponse{
			{
				Message: providertypes.Message{
					Role:  providertypes.RoleAssistant,
					Parts: []providertypes.ContentPart{providertypes.NewTextPart("continue planning")},
				},
			},
			{
				Message: providertypes.Message{
					Role: providertypes.RoleAssistant,
					ToolCalls: []providertypes.ToolCall{
						{
							ID:        "todo-claim-agent",
							Name:      tools.ToolNameTodoWrite,
							Arguments: `{"action":"claim","id":"agent-1","owner_type":"agent","owner_id":"main-agent"}`,
						},
						{
							ID:        "todo-complete-agent",
							Name:      tools.ToolNameTodoWrite,
							Arguments: `{"action":"complete","id":"agent-1","artifacts":["agent-1.done"]}`,
						},
					},
				},
				FinishReason: "tool_calls",
			},
			{
				Message: providertypes.Message{
					Role:  providertypes.RoleAssistant,
					Parts: []providertypes.ContentPart{providertypes.NewTextPart("all done")},
				},
			},
		},
	}
	service := NewWithFactory(
		manager,
		func() tools.Manager {
			registry := tools.NewRegistry()
			registry.Register(todotool.New())
			return registry
		}(),
		store,
		&scriptedProviderFactory{provider: scripted},
		&stubContextBuilder{},
	)
	service.SetSubAgentFactory(newSuccessSubAgentFactory())

	seed := agentsession.New("dispatch-mixed-deps")
	seed.Workdir = manager.Get().Workdir
	if err := seed.ReplaceTodos([]agentsession.TodoItem{
		{
			ID:       "agent-1",
			Content:  "agent prerequisite",
			Executor: agentsession.TodoExecutorAgent,
		},
		{
			ID:           "sub-1",
			Content:      "subagent follow-up",
			Executor:     agentsession.TodoExecutorSubAgent,
			Dependencies: []string{"agent-1"},
		},
	}); err != nil {
		t.Fatalf("ReplaceTodos(seed) error = %v", err)
	}
	saveSessionToMemoryStore(store, seed)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := service.Run(ctx, UserInput{
		SessionID: seed.ID,
		RunID:     "run-mixed-dependency-keep-driving",
		Parts:     []providertypes.ContentPart{providertypes.NewTextPart("continue")},
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if scripted.callCount != 3 {
		t.Fatalf("provider call count = %d, want 3", scripted.callCount)
	}

	session := firstSessionFromMemoryStore(t, store)
	agentTodo, ok := session.FindTodo("agent-1")
	if !ok || agentTodo.Status != agentsession.TodoStatusCompleted {
		t.Fatalf("agent todo = %+v, want completed", agentTodo)
	}
	subTodo, ok := session.FindTodo("sub-1")
	if !ok || subTodo.Status != agentsession.TodoStatusCompleted {
		t.Fatalf("sub todo = %+v, want completed", subTodo)
	}
}

func TestDispatchTodosFinishedQueueSizeExcludesAgentTodos(t *testing.T) {
	t.Parallel()

	manager := newRuntimeConfigManagerWithProviderEnvs(t, nil)
	store := newMemoryStore()
	service := NewWithFactory(
		manager,
		tools.NewRegistry(),
		store,
		&scriptedProviderFactory{provider: &scriptedProvider{}},
		&stubContextBuilder{},
	)

	session := agentsession.New("dispatch-finished-queue-size")
	session.Workdir = manager.Get().Workdir
	if err := session.ReplaceTodos([]agentsession.TodoItem{
		{
			ID:       "agent-1",
			Content:  "agent prerequisite",
			Executor: agentsession.TodoExecutorAgent,
			Status:   agentsession.TodoStatusPending,
		},
		{
			ID:           "sub-1",
			Content:      "subagent waiting for agent",
			Executor:     agentsession.TodoExecutorSubAgent,
			Status:       agentsession.TodoStatusBlocked,
			Dependencies: []string{"agent-1"},
		},
	}); err != nil {
		t.Fatalf("ReplaceTodos(session) error = %v", err)
	}
	saveSessionToMemoryStore(store, session)

	state := newRunState("run-finished-queue-size", session)
	state.phase = controlplane.PhaseDispatch
	progressed, err := service.dispatchTodos(context.Background(), &state, turnSnapshot{workdir: session.Workdir})
	if err != nil {
		t.Fatalf("dispatchTodos() error = %v", err)
	}
	if !progressed {
		t.Fatalf("dispatchTodos() progressed = false, want true")
	}

	events := collectRuntimeEvents(service.Events())
	foundFinished := false
	for _, event := range events {
		if event.Type != EventSubAgentFinished {
			continue
		}
		foundFinished = true
		payload, ok := event.Payload.(SubAgentEventPayload)
		if !ok {
			t.Fatalf("payload type = %T, want SubAgentEventPayload", event.Payload)
		}
		if payload.QueueSize != 1 {
			t.Fatalf("finished payload queue_size = %d, want 1", payload.QueueSize)
		}
		if payload.Running != 0 {
			t.Fatalf("finished payload running = %d, want 0", payload.Running)
		}
	}
	if !foundFinished {
		t.Fatalf("expected EventSubAgentFinished")
	}
}

func TestHasSubAgentTodoWaitingForAgentDependency(t *testing.T) {
	t.Parallel()

	if !hasSubAgentTodoWaitingForAgentDependency([]agentsession.TodoItem{
		{
			ID:       "agent",
			Executor: agentsession.TodoExecutorAgent,
			Status:   agentsession.TodoStatusPending,
		},
		{
			ID:           "sub",
			Executor:     agentsession.TodoExecutorSubAgent,
			Status:       agentsession.TodoStatusBlocked,
			Dependencies: []string{"agent"},
		},
	}) {
		t.Fatalf("expected pending agent dependency to require follow-up")
	}

	if hasSubAgentTodoWaitingForAgentDependency([]agentsession.TodoItem{
		{
			ID:       "agent",
			Executor: agentsession.TodoExecutorAgent,
			Status:   agentsession.TodoStatusCompleted,
		},
		{
			ID:           "sub",
			Executor:     agentsession.TodoExecutorSubAgent,
			Status:       agentsession.TodoStatusBlocked,
			Dependencies: []string{"agent"},
		},
	}) {
		t.Fatalf("completed agent dependency should not require follow-up")
	}
}

func TestEmitSubAgentSchedulerEventEmitsOnlySchedulerSpecificEvents(t *testing.T) {
	t.Parallel()

	manager := newRuntimeConfigManagerWithProviderEnvs(t, nil)
	store := newMemoryStore()
	service := NewWithFactory(
		manager,
		tools.NewRegistry(),
		store,
		&scriptedProviderFactory{provider: &scriptedProvider{}},
		&stubContextBuilder{},
	)
	state := newRunState("run-emit-scheduler-events", agentsession.New("emit-scheduler-events"))

	service.emitSubAgentSchedulerEvent(context.Background(), &state, subagent.SchedulerEvent{
		Type:    subagent.SchedulerEventSubAgentStarted,
		TaskID:  "task-1",
		Attempt: 1,
	})
	service.emitSubAgentSchedulerEvent(context.Background(), &state, subagent.SchedulerEvent{
		Type:    subagent.SchedulerEventSubAgentCompleted,
		TaskID:  "task-1",
		Attempt: 1,
	})
	service.emitSubAgentSchedulerEvent(context.Background(), &state, subagent.SchedulerEvent{
		Type:    subagent.SchedulerEventSubAgentRetried,
		TaskID:  "task-1",
		Attempt: 2,
		Reason:  "retry_after_failure",
		QueueSize: 5,
		Running:   1,
	})
	service.emitSubAgentSchedulerEvent(context.Background(), &state, subagent.SchedulerEvent{
		Type:   subagent.SchedulerEventBlocked,
		TaskID: "task-2",
		Reason: "dependency_unmet",
		QueueSize: 4,
		Running:   2,
	})
	service.emitSubAgentSchedulerEvent(context.Background(), &state, subagent.SchedulerEvent{
		Type:      subagent.SchedulerEventFinished,
		QueueSize: 3,
		Running:   0,
	})

	events := collectRuntimeEvents(service.Events())
	if len(events) != 3 {
		t.Fatalf("event count = %d, want 3", len(events))
	}
	assertEventContains(t, events, EventSubAgentRetried)
	assertEventContains(t, events, EventSubAgentBlocked)
	assertEventContains(t, events, EventSubAgentFinished)

	for _, event := range events {
		payload, ok := event.Payload.(SubAgentEventPayload)
		if !ok {
			t.Fatalf("payload type = %T, want SubAgentEventPayload", event.Payload)
		}
		switch event.Type {
		case EventSubAgentRetried:
			if payload.QueueSize != 5 || payload.Running != 1 {
				t.Fatalf("retried payload queue/running = %d/%d, want 5/1", payload.QueueSize, payload.Running)
			}
		case EventSubAgentBlocked:
			if payload.QueueSize != 4 || payload.Running != 2 {
				t.Fatalf("blocked payload queue/running = %d/%d, want 4/2", payload.QueueSize, payload.Running)
			}
		case EventSubAgentFinished:
			if payload.TaskID != "" {
				t.Fatalf("finished payload task_id = %q, want empty", payload.TaskID)
			}
			if payload.State != "" {
				t.Fatalf("finished payload state = %q, want empty", payload.State)
			}
			if payload.Reason != "dispatch_round_finished" {
				t.Fatalf("finished payload reason = %q, want dispatch_round_finished", payload.Reason)
			}
			if payload.QueueSize != 3 || payload.Running != 0 {
				t.Fatalf("finished payload queue/running = %d/%d, want 3/0", payload.QueueSize, payload.Running)
			}
		}
	}
}

func TestRunStopsMixedExecutorNoToolCallStallByNoProgressLimit(t *testing.T) {
	t.Parallel()

	manager := newRuntimeConfigManagerWithProviderEnvs(t, nil)
	store := newMemoryStore()
	scripted := &scriptedProvider{
		chatFn: func(ctx context.Context, req providertypes.GenerateRequest, events chan<- providertypes.StreamEvent) error {
			_ = ctx
			_ = req
			events <- providertypes.NewTextDeltaStreamEvent("still waiting")
			events <- providertypes.NewMessageDoneStreamEvent("stop", nil)
			return nil
		},
	}
	service := NewWithFactory(
		manager,
		tools.NewRegistry(),
		store,
		&scriptedProviderFactory{provider: scripted},
		&stubContextBuilder{},
	)

	seed := agentsession.New("dispatch-mixed-no-tool-stall")
	seed.Workdir = manager.Get().Workdir
	if err := seed.ReplaceTodos([]agentsession.TodoItem{
		{
			ID:       "agent-1",
			Content:  "agent prerequisite",
			Executor: agentsession.TodoExecutorAgent,
			Status:   agentsession.TodoStatusPending,
		},
		{
			ID:           "sub-1",
			Content:      "subagent follow-up",
			Executor:     agentsession.TodoExecutorSubAgent,
			Status:       agentsession.TodoStatusBlocked,
			Dependencies: []string{"agent-1"},
		},
	}); err != nil {
		t.Fatalf("ReplaceTodos(seed) error = %v", err)
	}
	saveSessionToMemoryStore(store, seed)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := service.Run(ctx, UserInput{
		SessionID: seed.ID,
		RunID:     "run-mixed-no-tool-stall",
		Parts:     []providertypes.ContentPart{providertypes.NewTextPart("continue")},
	})
	if !errors.Is(err, ErrNoProgressStreakLimit) {
		t.Fatalf("Run() error = %v, want ErrNoProgressStreakLimit", err)
	}

	if scripted.callCount != 3 {
		t.Fatalf("provider call count = %d, want 3", scripted.callCount)
	}

	session := firstSessionFromMemoryStore(t, store)
	agentTodo, ok := session.FindTodo("agent-1")
	if !ok || agentTodo.Status != agentsession.TodoStatusPending {
		t.Fatalf("agent todo = %+v, want pending", agentTodo)
	}
	subTodo, ok := session.FindTodo("sub-1")
	if !ok || subTodo.Status != agentsession.TodoStatusBlocked {
		t.Fatalf("sub todo = %+v, want blocked", subTodo)
	}

	events := collectRuntimeEvents(service.Events())
	assertStopReasonDecided(t, events, controlplane.StopReasonError, ErrNoProgressStreakLimit.Error())
}

func newSuccessSubAgentFactory() subagent.Factory {
	return subagent.NewWorkerFactory(func(role subagent.Role, policy subagent.RolePolicy) subagent.Engine {
		_ = role
		_ = policy
		return subagent.EngineFunc(func(ctx context.Context, input subagent.StepInput) (subagent.StepOutput, error) {
			_ = ctx
			return subagent.StepOutput{
				Done:  true,
				Delta: "completed",
				Output: subagent.Output{
					Summary:     "completed " + input.Task.ID,
					Findings:    []string{"ok"},
					Patches:     []string{"none"},
					Risks:       []string{"low"},
					NextActions: []string{"continue"},
					Artifacts:   []string{input.Task.ID + ".artifact"},
				},
			}, nil
		})
	})
}

func newFailOnceThenSuccessSubAgentFactory() subagent.Factory {
	var (
		mu       sync.Mutex
		attempts = make(map[string]int)
	)
	return subagent.NewWorkerFactory(func(role subagent.Role, policy subagent.RolePolicy) subagent.Engine {
		_ = role
		_ = policy
		return subagent.EngineFunc(func(ctx context.Context, input subagent.StepInput) (subagent.StepOutput, error) {
			_ = ctx

			mu.Lock()
			attempts[input.Task.ID]++
			attempt := attempts[input.Task.ID]
			mu.Unlock()
			if attempt == 1 {
				return subagent.StepOutput{}, errors.New("transient failure")
			}

			return subagent.StepOutput{
				Done:  true,
				Delta: "completed after retry",
				Output: subagent.Output{
					Summary:     "completed " + input.Task.ID,
					Findings:    []string{"ok"},
					Patches:     []string{"none"},
					Risks:       []string{"low"},
					NextActions: []string{"continue"},
					Artifacts:   []string{input.Task.ID + ".artifact"},
				},
			}, nil
		})
	})
}

func firstSessionFromMemoryStore(t *testing.T, store *memoryStore) agentsession.Session {
	t.Helper()
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, session := range store.sessions {
		return session
	}
	t.Fatalf("memory store has no sessions")
	return agentsession.Session{}
}

func saveSessionToMemoryStore(store *memoryStore, session agentsession.Session) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.saves++
	store.sessions[session.ID] = cloneSession(session)
}

func TestNewRuntimeSchedulerFactoryHandlesNilState(t *testing.T) {
	t.Parallel()

	factory := newRuntimeSchedulerFactory(nil, nil, "/tmp/workdir")
	worker, err := factory.Create(subagent.RoleCoder)
	if err != nil {
		t.Fatalf("Create(coder) error = %v", err)
	}
	impl, ok := worker.(*runtimeSchedulerWorker)
	if !ok {
		t.Fatalf("worker type = %T, want *runtimeSchedulerWorker", worker)
	}
	if impl.workdir != "" {
		t.Fatalf("workdir = %q, want empty when state is nil", impl.workdir)
	}
	if impl.runID != "" || impl.sessionID != "" || impl.agentID != "" {
		t.Fatalf("unexpected ids: run=%q session=%q agent=%q", impl.runID, impl.sessionID, impl.agentID)
	}
	if _, err := factory.Create(subagent.Role("invalid-role")); err == nil {
		t.Fatalf("Create(invalid-role) error = nil, want error")
	}
}

func TestRuntimeSchedulerWorkerStartAndStepGuards(t *testing.T) {
	t.Parallel()

	var nilWorker *runtimeSchedulerWorker
	if err := nilWorker.Start(subagent.Task{}, subagent.Budget{}, subagent.Capability{}); err == nil {
		t.Fatalf("nil Start() error = nil, want error")
	}
	if _, err := nilWorker.Step(context.Background()); err == nil {
		t.Fatalf("nil Step() error = nil, want error")
	}
	if err := nilWorker.Stop(subagent.StopReasonCanceled); err == nil {
		t.Fatalf("nil Stop() error = nil, want error")
	}
	if _, err := nilWorker.Result(); err == nil {
		t.Fatalf("nil Result() error = nil, want error")
	}
	if state := nilWorker.State(); state != subagent.StateIdle {
		t.Fatalf("nil State() = %q, want %q", state, subagent.StateIdle)
	}
	if policy := nilWorker.Policy(); !reflect.DeepEqual(policy, subagent.RolePolicy{}) {
		t.Fatalf("nil Policy() = %+v, want zero", policy)
	}

	worker := &runtimeSchedulerWorker{role: subagent.RoleCoder}
	if err := worker.Start(subagent.Task{}, subagent.Budget{}, subagent.Capability{}); err == nil {
		t.Fatalf("Start(invalid task) error = nil, want error")
	}
	if _, err := worker.Step(context.Background()); err == nil {
		t.Fatalf("Step(not started) error = nil, want error")
	}
	if _, err := worker.Result(); err == nil {
		t.Fatalf("Result(not completed) error = nil, want error")
	}

	validTask := subagent.Task{ID: "task-1", Goal: "implement task-1"}
	worker.result = subagent.Result{TaskID: "old", State: subagent.StateSucceeded}
	worker.resultErr = errors.New("old")
	worker.completed = true
	if err := worker.Start(validTask, subagent.Budget{MaxSteps: 1}, subagent.Capability{}); err != nil {
		t.Fatalf("Start(valid) error = %v", err)
	}
	if worker.state != subagent.StateRunning || worker.completed {
		t.Fatalf("worker state/completed = %q/%v, want running/false", worker.state, worker.completed)
	}
	if !reflect.DeepEqual(worker.result, subagent.Result{}) || worker.resultErr != nil {
		t.Fatalf("worker result reset failed: result=%+v err=%v", worker.result, worker.resultErr)
	}

	completedWorker := &runtimeSchedulerWorker{started: true, completed: true}
	if _, err := completedWorker.Step(context.Background()); err == nil {
		t.Fatalf("Step(completed) error = nil, want error")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := worker.Step(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Step(canceled ctx) error = %v, want context.Canceled", err)
	}

	worker.started = true
	worker.completed = false
	worker.service = nil
	if _, err := worker.Step(context.Background()); err == nil {
		t.Fatalf("Step(nil service) error = nil, want error")
	}
}

func TestRuntimeSchedulerWorkerStopPopulatesResultAndState(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		reason     subagent.StopReason
		wantState  subagent.State
		wantReason subagent.StopReason
	}{
		{
			name:       "completed",
			reason:     subagent.StopReasonCompleted,
			wantState:  subagent.StateSucceeded,
			wantReason: subagent.StopReasonCompleted,
		},
		{
			name:       "canceled",
			reason:     subagent.StopReasonCanceled,
			wantState:  subagent.StateCanceled,
			wantReason: subagent.StopReasonCanceled,
		},
		{
			name:       "timeout",
			reason:     subagent.StopReasonTimeout,
			wantState:  subagent.StateFailed,
			wantReason: subagent.StopReasonTimeout,
		},
		{
			name:       "empty reason fallback",
			reason:     subagent.StopReason(""),
			wantState:  subagent.StateFailed,
			wantReason: subagent.StopReasonError,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			worker := &runtimeSchedulerWorker{
				role:  subagent.RoleReviewer,
				task:  subagent.Task{ID: "task-stop"},
				state: subagent.StateRunning,
			}
			if err := worker.Stop(tc.reason); err != nil {
				t.Fatalf("Stop(%q) error = %v", tc.reason, err)
			}
			if !worker.completed {
				t.Fatalf("completed = false, want true")
			}
			if got := worker.State(); got != tc.wantState {
				t.Fatalf("State() = %q, want %q", got, tc.wantState)
			}
			if gotPolicy := worker.Policy(); !reflect.DeepEqual(gotPolicy, subagent.RolePolicy{}) {
				t.Fatalf("Policy() = %+v, want zero policy", gotPolicy)
			}
			result, err := worker.Result()
			if err != nil {
				t.Fatalf("Result() error = %v", err)
			}
			if result.TaskID != "task-stop" {
				t.Fatalf("result.TaskID = %q, want task-stop", result.TaskID)
			}
			if result.Role != subagent.RoleReviewer {
				t.Fatalf("result.Role = %q, want reviewer", result.Role)
			}
			if result.State != tc.wantState {
				t.Fatalf("result.State = %q, want %q", result.State, tc.wantState)
			}
			if result.StopReason != tc.wantReason {
				t.Fatalf("result.StopReason = %q, want %q", result.StopReason, tc.wantReason)
			}
		})
	}
}
