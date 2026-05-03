package checkpoint

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"neo-code/internal/session"
)

// CheckpointStore 定义 checkpoint 持久化的意图型接口。
type CheckpointStore interface {
	CreateCheckpoint(ctx context.Context, input CreateCheckpointInput) (session.CheckpointRecord, error)
	ListCheckpoints(ctx context.Context, sessionID string, opts ListCheckpointOpts) ([]session.CheckpointRecord, error)
	GetCheckpoint(ctx context.Context, checkpointID string) (session.CheckpointRecord, *session.SessionCheckpoint, error)
	UpdateCheckpointStatus(ctx context.Context, checkpointID string, status session.CheckpointStatus) error
	GetLatestResumeCheckpoint(ctx context.Context, sessionID string) (*session.ResumeCheckpoint, error)
}

// CreateCheckpointInput 描述一次 checkpoint 创建的完整输入。
type CreateCheckpointInput struct {
	Record    session.CheckpointRecord
	SessionCP session.SessionCheckpoint
}

// ListCheckpointOpts 描述 checkpoint 列表查询选项。
type ListCheckpointOpts struct {
	Limit          int
	RestorableOnly bool
}

// SQLiteCheckpointStore 基于 SQLite 实现 checkpoint 持久化。
type SQLiteCheckpointStore struct {
	dbPath string
	initMu sync.Mutex
	db     *sql.DB
}

// NewSQLiteCheckpointStore 创建 checkpoint 存储实例。
// dbPath 为 session.db 文件路径，可通过 session.DatabasePath 获取。
func NewSQLiteCheckpointStore(dbPath string) *SQLiteCheckpointStore {
	return &SQLiteCheckpointStore{
		dbPath: dbPath,
	}
}

// Close 释放数据库连接。
func (s *SQLiteCheckpointStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteCheckpointStore) ensureDB(ctx context.Context) (*sql.DB, error) {
	s.initMu.Lock()
	defer s.initMu.Unlock()
	if s.db != nil {
		return s.db, nil
	}
	db, err := sql.Open("sqlite", s.dbPath)
	if err != nil {
		return nil, fmt.Errorf("checkpoint: open sqlite db: %w", err)
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)

	pragmas := []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA synchronous=NORMAL`,
		`PRAGMA foreign_keys=ON`,
		`PRAGMA busy_timeout=5000`,
	}
	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("checkpoint: apply pragma %q: %w", pragma, err)
		}
	}
	s.db = db
	return db, nil
}

// CreateCheckpoint 在单一事务内写入 checkpoint record + session checkpoint。
// 事务内完成 record INSERT → session_cp INSERT → record UPDATE（设置 session_checkpoint_ref + status=available）。
func (s *SQLiteCheckpointStore) CreateCheckpoint(ctx context.Context, input CreateCheckpointInput) (session.CheckpointRecord, error) {
	if err := ctx.Err(); err != nil {
		return session.CheckpointRecord{}, err
	}
	db, err := s.ensureDB(ctx)
	if err != nil {
		return session.CheckpointRecord{}, err
	}

	record := input.Record
	sessionCP := input.SessionCP

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return session.CheckpointRecord{}, fmt.Errorf("checkpoint: begin create tx: %w", err)
	}
	defer rollbackTx(tx)

	// INSERT checkpoint_record (status = creating)
	_, err = tx.ExecContext(ctx, `
INSERT INTO checkpoint_records (
	id, workspace_key, session_id, run_id, workdir, created_at_ms,
	reason, code_checkpoint_ref, session_checkpoint_ref, resume_checkpoint_ref,
	transcript_revision, restorable, status
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`,
		record.CheckpointID,
		record.WorkspaceKey,
		record.SessionID,
		record.RunID,
		record.Workdir,
		toUnixMillis(record.CreatedAt),
		string(record.Reason),
		record.CodeCheckpointRef,
		"", // session_checkpoint_ref filled below
		record.ResumeCheckpointRef,
		record.TranscriptRevision,
		boolToInt(record.Restorable),
		string(session.CheckpointStatusCreating),
	)
	if err != nil {
		return session.CheckpointRecord{}, fmt.Errorf("checkpoint: insert record %s: %w", record.CheckpointID, err)
	}

	// INSERT session_checkpoint
	_, err = tx.ExecContext(ctx, `
INSERT INTO session_checkpoints (id, session_id, head_json, messages_json, created_at_ms)
VALUES (?, ?, ?, ?, ?)
`,
		sessionCP.ID,
		sessionCP.SessionID,
		sessionCP.HeadJSON,
		sessionCP.MessagesJSON,
		toUnixMillis(sessionCP.CreatedAt),
	)
	if err != nil {
		return session.CheckpointRecord{}, fmt.Errorf("checkpoint: insert session cp %s: %w", sessionCP.ID, err)
	}

	// UPDATE checkpoint_record: set session_checkpoint_ref + status = available
	_, err = tx.ExecContext(ctx, `
UPDATE checkpoint_records
SET session_checkpoint_ref = ?, status = ?
WHERE id = ?
`,
		sessionCP.ID,
		string(session.CheckpointStatusAvailable),
		record.CheckpointID,
	)
	if err != nil {
		return session.CheckpointRecord{}, fmt.Errorf("checkpoint: update record %s: %w", record.CheckpointID, err)
	}

	if err := tx.Commit(); err != nil {
		return session.CheckpointRecord{}, fmt.Errorf("checkpoint: commit create %s: %w", record.CheckpointID, err)
	}

	record.SessionCheckpointRef = sessionCP.ID
	record.Status = session.CheckpointStatusAvailable
	return record, nil
}

// ListCheckpoints 查询指定会话的 checkpoint 记录列表。
func (s *SQLiteCheckpointStore) ListCheckpoints(ctx context.Context, sessionID string, opts ListCheckpointOpts) ([]session.CheckpointRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	db, err := s.ensureDB(ctx)
	if err != nil {
		return nil, err
	}

	query := `
SELECT id, workspace_key, session_id, run_id, workdir, created_at_ms,
	reason, code_checkpoint_ref, session_checkpoint_ref, resume_checkpoint_ref,
	transcript_revision, restorable, status
FROM checkpoint_records
WHERE session_id = ?
`
	args := []any{sessionID}
	if opts.RestorableOnly {
		query += ` AND restorable = 1 AND status = ?`
		args = append(args, string(session.CheckpointStatusAvailable))
	}
	query += ` ORDER BY created_at_ms DESC`
	if opts.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, opts.Limit)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("checkpoint: list checkpoints for %s: %w", sessionID, err)
	}
	defer rows.Close()

	var records []session.CheckpointRecord
	for rows.Next() {
		var r session.CheckpointRecord
		var createdAtMS int64
		var reason, status string
		if err := rows.Scan(
			&r.CheckpointID, &r.WorkspaceKey, &r.SessionID, &r.RunID, &r.Workdir, &createdAtMS,
			&reason, &r.CodeCheckpointRef, &r.SessionCheckpointRef, &r.ResumeCheckpointRef,
			&r.TranscriptRevision, &r.Restorable, &status,
		); err != nil {
			return nil, fmt.Errorf("checkpoint: scan record: %w", err)
		}
		r.CreatedAt = fromUnixMillis(createdAtMS)
		r.Reason = session.CheckpointReason(reason)
		r.Status = session.CheckpointStatus(status)
		records = append(records, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("checkpoint: iterate records: %w", err)
	}
	return records, nil
}

// GetCheckpoint 查询单条 checkpoint record 及其关联的 session checkpoint。
func (s *SQLiteCheckpointStore) GetCheckpoint(ctx context.Context, checkpointID string) (session.CheckpointRecord, *session.SessionCheckpoint, error) {
	if err := ctx.Err(); err != nil {
		return session.CheckpointRecord{}, nil, err
	}
	db, err := s.ensureDB(ctx)
	if err != nil {
		return session.CheckpointRecord{}, nil, err
	}

	var r session.CheckpointRecord
	var createdAtMS int64
	var reason, status string
	err = db.QueryRowContext(ctx, `
SELECT id, workspace_key, session_id, run_id, workdir, created_at_ms,
	reason, code_checkpoint_ref, session_checkpoint_ref, resume_checkpoint_ref,
	transcript_revision, restorable, status
FROM checkpoint_records
WHERE id = ?
`, checkpointID).Scan(
		&r.CheckpointID, &r.WorkspaceKey, &r.SessionID, &r.RunID, &r.Workdir, &createdAtMS,
		&reason, &r.CodeCheckpointRef, &r.SessionCheckpointRef, &r.ResumeCheckpointRef,
		&r.TranscriptRevision, &r.Restorable, &status,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return session.CheckpointRecord{}, nil, fmt.Errorf("checkpoint: record %s not found", checkpointID)
		}
		return session.CheckpointRecord{}, nil, fmt.Errorf("checkpoint: query record %s: %w", checkpointID, err)
	}
	r.CreatedAt = fromUnixMillis(createdAtMS)
	r.Reason = session.CheckpointReason(reason)
	r.Status = session.CheckpointStatus(status)

	if r.SessionCheckpointRef == "" {
		return r, nil, nil
	}

	var sc session.SessionCheckpoint
	var scCreatedAtMS int64
	err = db.QueryRowContext(ctx, `
SELECT id, session_id, head_json, messages_json, created_at_ms
FROM session_checkpoints
WHERE id = ?
`, r.SessionCheckpointRef).Scan(
		&sc.ID, &sc.SessionID, &sc.HeadJSON, &sc.MessagesJSON, &scCreatedAtMS,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return r, nil, nil
		}
		return session.CheckpointRecord{}, nil, fmt.Errorf("checkpoint: query session cp %s: %w", r.SessionCheckpointRef, err)
	}
	sc.CreatedAt = fromUnixMillis(scCreatedAtMS)
	return r, &sc, nil
}

// UpdateCheckpointStatus 更新 checkpoint 的生命周期状态。
func (s *SQLiteCheckpointStore) UpdateCheckpointStatus(ctx context.Context, checkpointID string, status session.CheckpointStatus) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	db, err := s.ensureDB(ctx)
	if err != nil {
		return err
	}

	result, err := db.ExecContext(ctx, `UPDATE checkpoint_records SET status = ? WHERE id = ?`, string(status), checkpointID)
	if err != nil {
		return fmt.Errorf("checkpoint: update status %s: %w", checkpointID, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checkpoint: inspect rows affected for %s: %w", checkpointID, err)
	}
	if affected == 0 {
		return fmt.Errorf("checkpoint: record %s not found", checkpointID)
	}
	return nil
}

// GetLatestResumeCheckpoint 查询指定会话最新的 resume checkpoint。
func (s *SQLiteCheckpointStore) GetLatestResumeCheckpoint(ctx context.Context, sessionID string) (*session.ResumeCheckpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	db, err := s.ensureDB(ctx)
	if err != nil {
		return nil, err
	}

	var rc session.ResumeCheckpoint
	var updatedAtMS int64
	err = db.QueryRowContext(ctx, `
SELECT id, workspace_key, run_id, session_id, turn, phase, completion_state, transcript_revision, updated_at_ms
FROM resume_checkpoints
WHERE session_id = ?
ORDER BY updated_at_ms DESC
LIMIT 1
`, sessionID).Scan(
		&rc.ID, &rc.WorkspaceKey, &rc.RunID, &rc.SessionID,
		&rc.Turn, &rc.Phase, &rc.CompletionState,
		&rc.TranscriptRevision, &updatedAtMS,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("checkpoint: query resume checkpoint for %s: %w", sessionID, err)
	}
	rc.UpdatedAt = fromUnixMillis(updatedAtMS)
	return &rc, nil
}

func toUnixMillis(value time.Time) int64 {
	return value.UTC().UnixMilli()
}

func fromUnixMillis(value int64) time.Time {
	if value == 0 {
		return time.Time{}
	}
	return time.UnixMilli(value).UTC()
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func rollbackTx(tx *sql.Tx) {
	if tx != nil {
		_ = tx.Rollback()
	}
}
