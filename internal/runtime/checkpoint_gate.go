package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"neo-code/internal/checkpoint"
	providertypes "neo-code/internal/provider/types"
	agentsession "neo-code/internal/session"
	"neo-code/internal/tools"
)

// createPreWriteCheckpoint 在工具执行前创建 checkpoint，采用两阶段提交。
// shadowRepo 可用时做完整快照，不可用时降级为 session-only checkpoint。
// 失败时不阻塞工具执行，仅返回 error 由调用方发 warning event。
func (s *Service) createPreWriteCheckpoint(ctx context.Context, state *runState) error {
	if s.checkpointStore == nil {
		return nil
	}

	state.mu.Lock()
	session := state.session
	runID := state.runID
	state.mu.Unlock()

	// 降级模式：shadowRepo 不可用时创建 session-only checkpoint
	if s.shadowRepo == nil || !s.shadowRepo.IsAvailable() {
		return s.createDegradedCheckpoint(ctx, session, runID)
	}

	checkpointID := agentsession.NewID("checkpoint")
	ref := checkpoint.RefForCheckpoint(session.ID, checkpointID)
	commitMsg := fmt.Sprintf("pre_write checkpoint for session %s", session.ID)

	// Phase 1: shadow snapshot
	commitHash, err := s.shadowRepo.Snapshot(ctx, ref, commitMsg)
	if err != nil {
		return fmt.Errorf("checkpoint: shadow snapshot: %w", err)
	}

	// Phase 2: DB write
	head := session.HeadSnapshot()
	headJSON, err := json.Marshal(head)
	if err != nil {
		_ = s.shadowRepo.DeleteRef(ctx, ref)
		return fmt.Errorf("checkpoint: marshal head: %w", err)
	}
	messagesJSON, err := json.Marshal(session.Messages)
	if err != nil {
		_ = s.shadowRepo.DeleteRef(ctx, ref)
		return fmt.Errorf("checkpoint: marshal messages: %w", err)
	}

	effectiveWorkdir := strings.TrimSpace(session.Workdir)
	now := time.Now()

	record := agentsession.CheckpointRecord{
		CheckpointID:      checkpointID,
		WorkspaceKey:      agentsession.WorkspacePathKey(effectiveWorkdir),
		SessionID:         session.ID,
		RunID:             runID,
		Workdir:           effectiveWorkdir,
		CreatedAt:         now,
		Reason:            agentsession.CheckpointReasonPreWrite,
		CodeCheckpointRef: ref,
		Restorable:        true,
		Status:            agentsession.CheckpointStatusCreating,
	}

	sessionCP := agentsession.SessionCheckpoint{
		ID:           agentsession.NewID("sc"),
		SessionID:    session.ID,
		HeadJSON:     string(headJSON),
		MessagesJSON: string(messagesJSON),
		CreatedAt:    now,
	}

	input := checkpoint.CreateCheckpointInput{
		Record:    record,
		SessionCP: sessionCP,
	}

	saved, err := s.checkpointStore.CreateCheckpoint(ctx, input)
	if err != nil {
		_ = s.shadowRepo.DeleteRef(ctx, ref)
		return fmt.Errorf("checkpoint: db write: %w", err)
	}

	s.emitRunScoped(ctx, EventCheckpointCreated, state, CheckpointCreatedPayload{
		CheckpointID:         saved.CheckpointID,
		CodeCheckpointRef:    saved.CodeCheckpointRef,
		SessionCheckpointRef: saved.SessionCheckpointRef,
		CommitHash:           commitHash,
		Reason:               string(saved.Reason),
	})
	return nil
}

// toolCallsContainWorkspaceWrite 检查工具调用列表中是否包含会修改工作区的调用。
func toolCallsContainWorkspaceWrite(calls []providertypes.ToolCall) bool {
	for _, call := range calls {
		if isWorkspaceWriteToolCall(call) {
			return true
		}
	}
	return false
}

func isWorkspaceWriteToolCall(call providertypes.ToolCall) bool {
	switch call.Name {
	case tools.ToolNameFilesystemWriteFile, tools.ToolNameFilesystemEdit:
		return true
	case tools.ToolNameBash:
		return isBashWriteCommand(call.Arguments)
	}
	return false
}

func isBashWriteCommand(argumentsJSON string) bool {
	trimmed := strings.TrimSpace(argumentsJSON)
	if trimmed == "" {
		return false
	}
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(trimmed), &args); err != nil {
		return false
	}
	intent := tools.AnalyzeBashCommand(args.Command)
	return intent.Classification == tools.BashIntentClassificationLocalMutation ||
		intent.Classification == tools.BashIntentClassificationDestructive
}

// createDegradedCheckpoint 创建 session-only checkpoint（无代码快照），用于 shadowRepo 不可用时。
func (s *Service) createDegradedCheckpoint(ctx context.Context, session agentsession.Session, runID string) error {
	checkpointID := agentsession.NewID("checkpoint")
	now := time.Now()

	head := session.HeadSnapshot()
	headJSON, err := json.Marshal(head)
	if err != nil {
		return fmt.Errorf("checkpoint: marshal degraded head: %w", err)
	}
	messagesJSON, err := json.Marshal(session.Messages)
	if err != nil {
		return fmt.Errorf("checkpoint: marshal degraded messages: %w", err)
	}

	record := agentsession.CheckpointRecord{
		CheckpointID: checkpointID,
		WorkspaceKey: agentsession.WorkspacePathKey(session.Workdir),
		SessionID:    session.ID,
		RunID:        runID,
		Workdir:      session.Workdir,
		CreatedAt:    now,
		Reason:       agentsession.CheckpointReasonPreWriteDegraded,
		Restorable:   true,
		Status:       agentsession.CheckpointStatusCreating,
	}
	sessionCP := agentsession.SessionCheckpoint{
		ID:           agentsession.NewID("sc"),
		SessionID:    session.ID,
		HeadJSON:     string(headJSON),
		MessagesJSON: string(messagesJSON),
		CreatedAt:    now,
	}

	saved, err := s.checkpointStore.CreateCheckpoint(ctx, checkpoint.CreateCheckpointInput{
		Record:    record,
		SessionCP: sessionCP,
	})
	if err != nil {
		return fmt.Errorf("checkpoint: degraded create: %w", err)
	}

	s.emitRunScoped(ctx, EventCheckpointCreated, nil, CheckpointCreatedPayload{
		CheckpointID:         saved.CheckpointID,
		CodeCheckpointRef:    "",
		SessionCheckpointRef: saved.SessionCheckpointRef,
		CommitHash:           "",
		Reason:               string(saved.Reason),
	})
	return nil
}
