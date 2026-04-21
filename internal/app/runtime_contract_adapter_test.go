package app

import (
	"context"
	"errors"
	"testing"
	"time"

	providertypes "neo-code/internal/provider/types"
	agentruntime "neo-code/internal/runtime"
	agentsession "neo-code/internal/session"
	"neo-code/internal/skills"
	"neo-code/internal/tools"
	tuiservices "neo-code/internal/tui/services"
)

type runtimeContractAdapterTestRuntime struct {
	events chan agentruntime.RuntimeEvent

	submitInput               agentruntime.PrepareInput
	submitErr                 error
	prepareUserInputInput     agentruntime.PrepareInput
	prepareUserInputOutput    agentruntime.UserInput
	prepareUserInputErr       error
	runInput                  agentruntime.UserInput
	runErr                    error
	compactInput              agentruntime.CompactInput
	compactOutput             agentruntime.CompactResult
	compactErr                error
	systemToolInput           agentruntime.SystemToolInput
	systemToolOutput          tools.ToolResult
	systemToolErr             error
	resolvePermissionInput    agentruntime.PermissionResolutionInput
	resolvePermissionErr      error
	cancelActiveRunOutput     bool
	listSessionsOutput        []agentsession.Summary
	listSessionsErr           error
	loadSessionID             string
	loadSessionOutput         agentsession.Session
	loadSessionErr            error
	activateSessionSkillInput struct {
		sessionID string
		skillID   string
	}
	activateSessionSkillErr error
	deactivateSessionSkill  struct {
		sessionID string
		skillID   string
	}
	deactivateSessionSkillErr error
	listSessionSkillsID       string
	listSessionSkillsOutput   []agentruntime.SessionSkillState
	listSessionSkillsErr      error
	loadLogSessionID          string
	loadLogOutput             []agentruntime.SessionLogEntry
	loadLogErr                error
	saveLogSessionID          string
	saveLogEntries            []agentruntime.SessionLogEntry
	saveLogErr                error
}

type runtimeContractAdapterNoLogStore struct {
	events chan agentruntime.RuntimeEvent
}

func (s *runtimeContractAdapterNoLogStore) Submit(context.Context, agentruntime.PrepareInput) error {
	return nil
}
func (s *runtimeContractAdapterNoLogStore) PrepareUserInput(context.Context, agentruntime.PrepareInput) (agentruntime.UserInput, error) {
	return agentruntime.UserInput{}, nil
}
func (s *runtimeContractAdapterNoLogStore) Run(context.Context, agentruntime.UserInput) error {
	return nil
}
func (s *runtimeContractAdapterNoLogStore) Compact(context.Context, agentruntime.CompactInput) (agentruntime.CompactResult, error) {
	return agentruntime.CompactResult{}, nil
}
func (s *runtimeContractAdapterNoLogStore) ExecuteSystemTool(context.Context, agentruntime.SystemToolInput) (tools.ToolResult, error) {
	return tools.ToolResult{}, nil
}
func (s *runtimeContractAdapterNoLogStore) ResolvePermission(context.Context, agentruntime.PermissionResolutionInput) error {
	return nil
}
func (s *runtimeContractAdapterNoLogStore) CancelActiveRun() bool { return false }
func (s *runtimeContractAdapterNoLogStore) Events() <-chan agentruntime.RuntimeEvent {
	if s.events == nil {
		s.events = make(chan agentruntime.RuntimeEvent)
	}
	return s.events
}
func (s *runtimeContractAdapterNoLogStore) ListSessions(context.Context) ([]agentsession.Summary, error) {
	return nil, nil
}
func (s *runtimeContractAdapterNoLogStore) LoadSession(context.Context, string) (agentsession.Session, error) {
	return agentsession.Session{}, nil
}
func (s *runtimeContractAdapterNoLogStore) ActivateSessionSkill(context.Context, string, string) error {
	return nil
}
func (s *runtimeContractAdapterNoLogStore) DeactivateSessionSkill(context.Context, string, string) error {
	return nil
}
func (s *runtimeContractAdapterNoLogStore) ListSessionSkills(context.Context, string) ([]agentruntime.SessionSkillState, error) {
	return nil, nil
}

func (s *runtimeContractAdapterTestRuntime) Submit(_ context.Context, input agentruntime.PrepareInput) error {
	s.submitInput = input
	return s.submitErr
}
func (s *runtimeContractAdapterTestRuntime) PrepareUserInput(_ context.Context, input agentruntime.PrepareInput) (agentruntime.UserInput, error) {
	s.prepareUserInputInput = input
	return s.prepareUserInputOutput, s.prepareUserInputErr
}
func (s *runtimeContractAdapterTestRuntime) Run(_ context.Context, input agentruntime.UserInput) error {
	s.runInput = input
	return s.runErr
}
func (s *runtimeContractAdapterTestRuntime) Compact(_ context.Context, input agentruntime.CompactInput) (agentruntime.CompactResult, error) {
	s.compactInput = input
	return s.compactOutput, s.compactErr
}
func (s *runtimeContractAdapterTestRuntime) ExecuteSystemTool(_ context.Context, input agentruntime.SystemToolInput) (tools.ToolResult, error) {
	s.systemToolInput = input
	return s.systemToolOutput, s.systemToolErr
}
func (s *runtimeContractAdapterTestRuntime) ResolvePermission(_ context.Context, input agentruntime.PermissionResolutionInput) error {
	s.resolvePermissionInput = input
	return s.resolvePermissionErr
}
func (s *runtimeContractAdapterTestRuntime) CancelActiveRun() bool { return s.cancelActiveRunOutput }
func (s *runtimeContractAdapterTestRuntime) Events() <-chan agentruntime.RuntimeEvent {
	if s.events == nil {
		s.events = make(chan agentruntime.RuntimeEvent, 8)
	}
	return s.events
}
func (s *runtimeContractAdapterTestRuntime) ListSessions(context.Context) ([]agentsession.Summary, error) {
	return s.listSessionsOutput, s.listSessionsErr
}
func (s *runtimeContractAdapterTestRuntime) LoadSession(_ context.Context, id string) (agentsession.Session, error) {
	s.loadSessionID = id
	return s.loadSessionOutput, s.loadSessionErr
}
func (s *runtimeContractAdapterTestRuntime) ActivateSessionSkill(_ context.Context, sessionID string, skillID string) error {
	s.activateSessionSkillInput = struct {
		sessionID string
		skillID   string
	}{sessionID: sessionID, skillID: skillID}
	return s.activateSessionSkillErr
}
func (s *runtimeContractAdapterTestRuntime) DeactivateSessionSkill(_ context.Context, sessionID string, skillID string) error {
	s.deactivateSessionSkill = struct {
		sessionID string
		skillID   string
	}{sessionID: sessionID, skillID: skillID}
	return s.deactivateSessionSkillErr
}
func (s *runtimeContractAdapterTestRuntime) ListSessionSkills(_ context.Context, sessionID string) ([]agentruntime.SessionSkillState, error) {
	s.listSessionSkillsID = sessionID
	return s.listSessionSkillsOutput, s.listSessionSkillsErr
}
func (s *runtimeContractAdapterTestRuntime) LoadSessionLogEntries(_ context.Context, sessionID string) ([]agentruntime.SessionLogEntry, error) {
	s.loadLogSessionID = sessionID
	return s.loadLogOutput, s.loadLogErr
}
func (s *runtimeContractAdapterTestRuntime) SaveSessionLogEntries(_ context.Context, sessionID string, entries []agentruntime.SessionLogEntry) error {
	s.saveLogSessionID = sessionID
	s.saveLogEntries = append([]agentruntime.SessionLogEntry(nil), entries...)
	return s.saveLogErr
}

func TestRuntimeContractAdapterNilGuards(t *testing.T) {
	var adapter *runtimeContractAdapter

	if err := adapter.Submit(context.Background(), tuiservices.PrepareInput{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Submit() error = %v", err)
	}
	if _, err := adapter.PrepareUserInput(context.Background(), tuiservices.PrepareInput{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("PrepareUserInput() error = %v", err)
	}
	if err := adapter.Run(context.Background(), tuiservices.UserInput{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v", err)
	}
	if _, err := adapter.Compact(context.Background(), tuiservices.CompactInput{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Compact() error = %v", err)
	}
	if _, err := adapter.ExecuteSystemTool(context.Background(), tuiservices.SystemToolInput{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("ExecuteSystemTool() error = %v", err)
	}
	if err := adapter.ResolvePermission(context.Background(), tuiservices.PermissionResolutionInput{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("ResolvePermission() error = %v", err)
	}
	if adapter.CancelActiveRun() {
		t.Fatalf("CancelActiveRun() should return false")
	}
	if adapter.Events() != nil {
		t.Fatalf("Events() on nil adapter should return nil")
	}
	if _, err := adapter.ListSessions(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if _, err := adapter.LoadSession(context.Background(), "x"); !errors.Is(err, context.Canceled) {
		t.Fatalf("LoadSession() error = %v", err)
	}
	if err := adapter.ActivateSessionSkill(context.Background(), "s", "k"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ActivateSessionSkill() error = %v", err)
	}
	if err := adapter.DeactivateSessionSkill(context.Background(), "s", "k"); !errors.Is(err, context.Canceled) {
		t.Fatalf("DeactivateSessionSkill() error = %v", err)
	}
	if _, err := adapter.ListSessionSkills(context.Background(), "s"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ListSessionSkills() error = %v", err)
	}
	logEntries, err := adapter.LoadSessionLogEntries(context.Background(), "s")
	if err != nil || logEntries != nil {
		t.Fatalf("LoadSessionLogEntries() = (%v, %v), want (nil, nil)", logEntries, err)
	}
	if err := adapter.SaveSessionLogEntries(context.Background(), "s", nil); err != nil {
		t.Fatalf("SaveSessionLogEntries() error = %v", err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestRuntimeContractAdapterForwardsRuntimeCalls(t *testing.T) {
	runtimeSvc := &runtimeContractAdapterTestRuntime{
		cancelActiveRunOutput: true,
		prepareUserInputOutput: agentruntime.UserInput{
			SessionID: " session-a ",
			RunID:     " run-a ",
			Parts:     []providertypes.ContentPart{providertypes.NewTextPart("ok")},
			Workdir:   " /workspace/a ",
			TaskID:    " task-a ",
			AgentID:   " agent-a ",
		},
		compactOutput: agentruntime.CompactResult{
			Applied:        true,
			BeforeChars:    100,
			AfterChars:     60,
			BeforeTokens:   12,
			SavedRatio:     0.4,
			TriggerMode:    "auto",
			TranscriptID:   "tid",
			TranscriptPath: "/tmp/tid.md",
		},
		systemToolOutput: tools.ToolResult{Name: "memo_read", Content: "ok"},
		listSessionsOutput: []agentsession.Summary{
			{ID: "s1", Title: "session-1"},
		},
		loadSessionOutput: agentsession.Session{ID: "session-load"},
		listSessionSkillsOutput: []agentruntime.SessionSkillState{
			{SkillID: "skill-x", Missing: false, Descriptor: &skills.Descriptor{ID: "skill-x", Name: "Skill X"}},
		},
	}
	adapter := newRuntimeContractAdapter(runtimeSvc)
	defer func() { _ = adapter.Close() }()

	prepareInput := tuiservices.PrepareInput{
		SessionID: " session-a ",
		RunID:     " run-a ",
		Workdir:   " /workspace/a ",
		Text:      "hello",
		Images: []tuiservices.UserImageInput{
			{Path: " /tmp/a.png ", MimeType: " image/png "},
		},
	}
	if err := adapter.Submit(context.Background(), prepareInput); err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if runtimeSvc.submitInput.SessionID != "session-a" || runtimeSvc.submitInput.Workdir != "/workspace/a" {
		t.Fatalf("Submit() input mismatch: %#v", runtimeSvc.submitInput)
	}
	if runtimeSvc.submitInput.Images[0].Path != "/tmp/a.png" || runtimeSvc.submitInput.Images[0].MimeType != "image/png" {
		t.Fatalf("Submit() image mapping mismatch: %#v", runtimeSvc.submitInput.Images)
	}

	prepared, err := adapter.PrepareUserInput(context.Background(), prepareInput)
	if err != nil {
		t.Fatalf("PrepareUserInput() error = %v", err)
	}
	if prepared.SessionID != "session-a" || prepared.Workdir != "/workspace/a" || prepared.TaskID != "task-a" {
		t.Fatalf("PrepareUserInput() output mismatch: %#v", prepared)
	}
	if runtimeSvc.prepareUserInputInput.SessionID != "session-a" {
		t.Fatalf("PrepareUserInput() input mismatch: %#v", runtimeSvc.prepareUserInputInput)
	}

	runInput := tuiservices.UserInput{
		SessionID: " session-run ",
		RunID:     " run-1 ",
		Workdir:   " /workspace/run ",
		TaskID:    " task-1 ",
		AgentID:   " agent-1 ",
		Parts:     []providertypes.ContentPart{providertypes.NewTextPart("hello")},
	}
	if err := adapter.Run(context.Background(), runInput); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if runtimeSvc.runInput.SessionID != "session-run" || runtimeSvc.runInput.RunID != "run-1" {
		t.Fatalf("Run() input mismatch: %#v", runtimeSvc.runInput)
	}
	if len(runtimeSvc.runInput.Parts) != 1 {
		t.Fatalf("Run() parts not forwarded: %#v", runtimeSvc.runInput.Parts)
	}

	compactResult, err := adapter.Compact(context.Background(), tuiservices.CompactInput{SessionID: " s1 ", RunID: " r1 "})
	if err != nil {
		t.Fatalf("Compact() error = %v", err)
	}
	if runtimeSvc.compactInput.SessionID != "s1" || runtimeSvc.compactInput.RunID != "r1" {
		t.Fatalf("Compact() input mismatch: %#v", runtimeSvc.compactInput)
	}
	if !compactResult.Applied || compactResult.BeforeChars != 100 || compactResult.TranscriptID != "tid" {
		t.Fatalf("Compact() output mismatch: %#v", compactResult)
	}

	args := []byte("payload")
	toolResult, err := adapter.ExecuteSystemTool(context.Background(), tuiservices.SystemToolInput{
		SessionID: " s1 ",
		RunID:     " r1 ",
		Workdir:   " /workspace ",
		ToolName:  " memo_read ",
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("ExecuteSystemTool() error = %v", err)
	}
	args[0] = 'X'
	if runtimeSvc.systemToolInput.SessionID != "s1" || string(runtimeSvc.systemToolInput.Arguments) != "payload" {
		t.Fatalf("ExecuteSystemTool() input mismatch: %#v", runtimeSvc.systemToolInput)
	}
	if toolResult.Name != "memo_read" {
		t.Fatalf("ExecuteSystemTool() output mismatch: %#v", toolResult)
	}

	if err := adapter.ResolvePermission(context.Background(), tuiservices.PermissionResolutionInput{
		RequestID: " req-1 ",
		Decision:  tuiservices.DecisionAllowSession,
	}); err != nil {
		t.Fatalf("ResolvePermission() error = %v", err)
	}
	if runtimeSvc.resolvePermissionInput.RequestID != "req-1" ||
		string(runtimeSvc.resolvePermissionInput.Decision) != string(tuiservices.DecisionAllowSession) {
		t.Fatalf("ResolvePermission() input mismatch: %#v", runtimeSvc.resolvePermissionInput)
	}

	if !adapter.CancelActiveRun() {
		t.Fatalf("CancelActiveRun() should forward runtime response")
	}
	sessions, err := adapter.ListSessions(context.Background())
	if err != nil || len(sessions) != 1 || sessions[0].ID != "s1" {
		t.Fatalf("ListSessions() = (%#v, %v)", sessions, err)
	}
	session, err := adapter.LoadSession(context.Background(), " session-load ")
	if err != nil || session.ID != "session-load" || runtimeSvc.loadSessionID != "session-load" {
		t.Fatalf("LoadSession() = (%#v, %v), runtime id %q", session, err, runtimeSvc.loadSessionID)
	}
	if err := adapter.ActivateSessionSkill(context.Background(), " s1 ", " skill-x "); err != nil {
		t.Fatalf("ActivateSessionSkill() error = %v", err)
	}
	if runtimeSvc.activateSessionSkillInput.sessionID != "s1" || runtimeSvc.activateSessionSkillInput.skillID != "skill-x" {
		t.Fatalf("ActivateSessionSkill() input mismatch: %#v", runtimeSvc.activateSessionSkillInput)
	}
	if err := adapter.DeactivateSessionSkill(context.Background(), " s1 ", " skill-x "); err != nil {
		t.Fatalf("DeactivateSessionSkill() error = %v", err)
	}
	if runtimeSvc.deactivateSessionSkill.sessionID != "s1" || runtimeSvc.deactivateSessionSkill.skillID != "skill-x" {
		t.Fatalf("DeactivateSessionSkill() input mismatch: %#v", runtimeSvc.deactivateSessionSkill)
	}
	skillStates, err := adapter.ListSessionSkills(context.Background(), " s1 ")
	if err != nil || len(skillStates) != 1 || skillStates[0].SkillID != "skill-x" {
		t.Fatalf("ListSessionSkills() = (%#v, %v)", skillStates, err)
	}
}

func TestRuntimeContractAdapterSessionLogPersistence(t *testing.T) {
	timestamp := time.Now().UTC().Truncate(time.Second)
	runtimeSvc := &runtimeContractAdapterTestRuntime{
		loadLogOutput: []agentruntime.SessionLogEntry{
			{Timestamp: timestamp, Level: "info", Source: "gateway", Message: "ok"},
		},
	}
	adapter := newRuntimeContractAdapter(runtimeSvc)
	defer func() { _ = adapter.Close() }()

	entries, err := adapter.LoadSessionLogEntries(context.Background(), " s1 ")
	if err != nil {
		t.Fatalf("LoadSessionLogEntries() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Level != "info" || runtimeSvc.loadLogSessionID != "s1" {
		t.Fatalf("LoadSessionLogEntries() mismatch entries=%#v id=%q", entries, runtimeSvc.loadLogSessionID)
	}

	saveEntries := []tuiservices.SessionLogEntry{{Timestamp: timestamp, Level: "warn", Source: "runtime", Message: "m"}}
	if err := adapter.SaveSessionLogEntries(context.Background(), " s2 ", saveEntries); err != nil {
		t.Fatalf("SaveSessionLogEntries() error = %v", err)
	}
	if runtimeSvc.saveLogSessionID != "s2" || len(runtimeSvc.saveLogEntries) != 1 || runtimeSvc.saveLogEntries[0].Level != "warn" {
		t.Fatalf("SaveSessionLogEntries() mismatch id=%q entries=%#v", runtimeSvc.saveLogSessionID, runtimeSvc.saveLogEntries)
	}
}

func TestRuntimeContractAdapterErrorPaths(t *testing.T) {
	runtimeSvc := &runtimeContractAdapterTestRuntime{
		prepareUserInputErr:  errors.New("prepare failed"),
		compactErr:           errors.New("compact failed"),
		listSessionSkillsErr: errors.New("list skills failed"),
		loadLogErr:           errors.New("load logs failed"),
	}
	adapter := newRuntimeContractAdapter(runtimeSvc)
	defer func() { _ = adapter.Close() }()

	if _, err := adapter.PrepareUserInput(context.Background(), tuiservices.PrepareInput{}); err == nil {
		t.Fatalf("PrepareUserInput() should fail")
	}
	if _, err := adapter.Compact(context.Background(), tuiservices.CompactInput{}); err == nil {
		t.Fatalf("Compact() should fail")
	}
	if _, err := adapter.ListSessionSkills(context.Background(), "s1"); err == nil {
		t.Fatalf("ListSessionSkills() should fail")
	}
	if _, err := adapter.LoadSessionLogEntries(context.Background(), "s1"); err == nil {
		t.Fatalf("LoadSessionLogEntries() should fail")
	}
}

func TestRuntimeContractAdapterSessionLogNoStore(t *testing.T) {
	adapter := newRuntimeContractAdapter(&runtimeContractAdapterNoLogStore{})
	defer func() { _ = adapter.Close() }()

	entries, err := adapter.LoadSessionLogEntries(context.Background(), "s1")
	if err != nil || entries != nil {
		t.Fatalf("LoadSessionLogEntries() = (%v, %v), want (nil, nil)", entries, err)
	}
	if err := adapter.SaveSessionLogEntries(context.Background(), "s1", []tuiservices.SessionLogEntry{{Level: "info"}}); err != nil {
		t.Fatalf("SaveSessionLogEntries() error = %v", err)
	}
}

func TestRuntimeContractAdapterEventForwardingAndClose(t *testing.T) {
	runtimeSvc := &runtimeContractAdapterTestRuntime{events: make(chan agentruntime.RuntimeEvent, 1)}
	adapter := newRuntimeContractAdapter(runtimeSvc)

	runtimeSvc.events <- agentruntime.RuntimeEvent{
		Type:           agentruntime.EventPhaseChanged,
		RunID:          " run-1 ",
		SessionID:      " session-1 ",
		Turn:           2,
		Phase:          " running ",
		Timestamp:      time.Now().UTC(),
		PayloadVersion: 2,
		Payload:        agentruntime.PhaseChangedPayload{From: "bootstrap", To: "running"},
	}
	close(runtimeSvc.events)

	select {
	case event := <-adapter.Events():
		typed, ok := event.Payload.(tuiservices.PhaseChangedPayload)
		if !ok {
			t.Fatalf("payload type = %T", event.Payload)
		}
		if event.Type != tuiservices.EventPhaseChanged || event.RunID != "run-1" || typed.To != "running" {
			t.Fatalf("event mapping mismatch: %#v payload=%#v", event, typed)
		}
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for forwarded event")
	}

	// 二次关闭覆盖 closeOnce 分支。
	if err := adapter.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if err := adapter.Close(); err != nil {
		t.Fatalf("Close() second call error = %v", err)
	}

	if _, ok := <-adapter.Events(); ok {
		t.Fatalf("Events() channel should be closed")
	}
}

func TestRuntimeContractAdapterForwardEventsGuards(t *testing.T) {
	adapter := &runtimeContractAdapter{
		closeCh: make(chan struct{}),
		done:    make(chan struct{}),
		events:  make(chan tuiservices.RuntimeEvent, 1),
	}
	go adapter.forwardEvents()
	select {
	case <-adapter.done:
	case <-time.After(time.Second):
		t.Fatalf("forwardEvents() should exit when runtime is nil")
	}
	if _, ok := <-adapter.events; ok {
		t.Fatalf("events channel should be closed")
	}

	runtimeSvc := &runtimeContractAdapterNoLogStore{events: make(chan agentruntime.RuntimeEvent)}
	adapter = newRuntimeContractAdapter(runtimeSvc)
	close(adapter.closeCh)
	select {
	case <-adapter.done:
	case <-time.After(time.Second):
		t.Fatalf("forwardEvents() should exit when closeCh is closed")
	}
}

func TestConvertHelpersAndPayloadMapping(t *testing.T) {
	convertedPrepare := convertPrepareInputToRuntime(tuiservices.PrepareInput{
		SessionID: " s ",
		RunID:     " r ",
		Workdir:   " /w ",
		Text:      "hello",
		Images:    []tuiservices.UserImageInput{{Path: " /a.png ", MimeType: " image/png "}},
	})
	if convertedPrepare.SessionID != "s" || convertedPrepare.Images[0].MimeType != "image/png" {
		t.Fatalf("convertPrepareInputToRuntime() mismatch: %#v", convertedPrepare)
	}

	runtimeInput := convertUserInputToRuntime(tuiservices.UserInput{
		SessionID: " s ",
		RunID:     " r ",
		Workdir:   " /w ",
		TaskID:    " t ",
		AgentID:   " a ",
		Parts:     []providertypes.ContentPart{providertypes.NewTextPart("x")},
	})
	if runtimeInput.SessionID != "s" || runtimeInput.AgentID != "a" || len(runtimeInput.Parts) != 1 {
		t.Fatalf("convertUserInputToRuntime() mismatch: %#v", runtimeInput)
	}
	contractInput := convertUserInputFromRuntime(runtimeInput)
	if contractInput.SessionID != "s" || contractInput.TaskID != "t" || len(contractInput.Parts) != 1 {
		t.Fatalf("convertUserInputFromRuntime() mismatch: %#v", contractInput)
	}

	event := convertRuntimeEventToContract(agentruntime.RuntimeEvent{
		Type:           agentruntime.EventStopReasonDecided,
		RunID:          " run ",
		SessionID:      " session ",
		Phase:          " done ",
		PayloadVersion: 1,
		Payload: agentruntime.StopReasonDecidedPayload{
			Reason: "max_turns",
			Detail: "limit",
		},
	})
	stopPayload, ok := event.Payload.(tuiservices.StopReasonDecidedPayload)
	if !ok || event.RunID != "run" || event.SessionID != "session" || stopPayload.Reason != "max_turns" {
		t.Fatalf("convertRuntimeEventToContract() mismatch: event=%#v payload=%#v", event, event.Payload)
	}

	payloadTests := []struct {
		name    string
		input   any
		assertf func(t *testing.T, mapped any)
	}{
		{
			name:  "permission request",
			input: agentruntime.PermissionRequestPayload{RequestID: "req"},
			assertf: func(t *testing.T, mapped any) {
				p, ok := mapped.(tuiservices.PermissionRequestPayload)
				if !ok || p.RequestID != "req" {
					t.Fatalf("mapped payload = %#v", mapped)
				}
			},
		},
		{
			name:  "permission resolved",
			input: agentruntime.PermissionResolvedPayload{ResolvedAs: "approved"},
			assertf: func(t *testing.T, mapped any) {
				p, ok := mapped.(tuiservices.PermissionResolvedPayload)
				if !ok || p.ResolvedAs != "approved" {
					t.Fatalf("mapped payload = %#v", mapped)
				}
			},
		},
		{
			name:  "compact result",
			input: agentruntime.CompactResult{Applied: true, TranscriptID: "tid"},
			assertf: func(t *testing.T, mapped any) {
				p, ok := mapped.(tuiservices.CompactResult)
				if !ok || !p.Applied || p.TranscriptID != "tid" {
					t.Fatalf("mapped payload = %#v", mapped)
				}
			},
		},
		{
			name:  "compact error",
			input: agentruntime.CompactErrorPayload{Message: "x"},
			assertf: func(t *testing.T, mapped any) {
				p, ok := mapped.(tuiservices.CompactErrorPayload)
				if !ok || p.Message != "x" {
					t.Fatalf("mapped payload = %#v", mapped)
				}
			},
		},
		{
			name:  "phase changed",
			input: agentruntime.PhaseChangedPayload{From: "a", To: "b"},
			assertf: func(t *testing.T, mapped any) {
				p, ok := mapped.(tuiservices.PhaseChangedPayload)
				if !ok || p.To != "b" {
					t.Fatalf("mapped payload = %#v", mapped)
				}
			},
		},
		{
			name:  "todo event",
			input: agentruntime.TodoEventPayload{Action: "update"},
			assertf: func(t *testing.T, mapped any) {
				p, ok := mapped.(tuiservices.TodoEventPayload)
				if !ok || p.Action != "update" {
					t.Fatalf("mapped payload = %#v", mapped)
				}
			},
		},
		{
			name:  "input normalized",
			input: agentruntime.InputNormalizedPayload{TextLength: 2},
			assertf: func(t *testing.T, mapped any) {
				p, ok := mapped.(tuiservices.InputNormalizedPayload)
				if !ok || p.TextLength != 2 {
					t.Fatalf("mapped payload = %#v", mapped)
				}
			},
		},
		{
			name:  "asset saved",
			input: agentruntime.AssetSavedPayload{AssetID: "asset-1"},
			assertf: func(t *testing.T, mapped any) {
				p, ok := mapped.(tuiservices.AssetSavedPayload)
				if !ok || p.AssetID != "asset-1" {
					t.Fatalf("mapped payload = %#v", mapped)
				}
			},
		},
		{
			name:  "asset failed",
			input: agentruntime.AssetSaveFailedPayload{Message: "bad"},
			assertf: func(t *testing.T, mapped any) {
				p, ok := mapped.(tuiservices.AssetSaveFailedPayload)
				if !ok || p.Message != "bad" {
					t.Fatalf("mapped payload = %#v", mapped)
				}
			},
		},
		{
			name:  "passthrough default",
			input: "keep",
			assertf: func(t *testing.T, mapped any) {
				if mapped != "keep" {
					t.Fatalf("mapped payload = %#v", mapped)
				}
			},
		},
	}

	for _, tc := range payloadTests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			tc.assertf(t, convertRuntimePayloadToContract(tc.input))
		})
	}
}
