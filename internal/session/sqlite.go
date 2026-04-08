package session

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("session record not found")

// SQLiteStore is a SQLite-backed implementation of [Store].
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) a SQLite database at dbPath and ensures
// the required schema exists.
func NewSQLiteStore(ctx context.Context, dbPath string) (*SQLiteStore, error) {
	if dbPath == "" {
		return nil, errors.New("db path is required")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	return store, nil
}

// Close closes the underlying database handle.
func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) migrate(ctx context.Context) error {
	stmts := []string{
		`PRAGMA foreign_keys = ON;`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			directory TEXT NOT NULL,
			title TEXT NOT NULL,
			parent_id TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
			updated_at INTEGER NOT NULL,
			compacted_at INTEGER,
			archived_at INTEGER
		);`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_project_updated
			ON sessions(project_id, updated_at DESC);`,

		`CREATE TABLE IF NOT EXISTS messages (
			id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			role TEXT NOT NULL,
			agent_name TEXT NOT NULL DEFAULT '',
			model_id TEXT NOT NULL DEFAULT '',
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			cost_usd REAL NOT NULL DEFAULT 0,
			finish_reason TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL,
			FOREIGN KEY(session_id) REFERENCES sessions(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_messages_session_created
			ON messages(session_id, created_at ASC);`,

		`CREATE TABLE IF NOT EXISTS parts (
			id TEXT PRIMARY KEY,
			message_id TEXT NOT NULL,
			type TEXT NOT NULL,
			data BLOB NOT NULL,
			created_at INTEGER NOT NULL,
			FOREIGN KEY(message_id) REFERENCES messages(id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_parts_message_created
			ON parts(message_id, created_at ASC);`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) CreateSession(ctx context.Context, sess Session) error {
	now := time.Now().UTC()
	if sess.ID == "" {
		sess.ID = uuid.NewString()
	}
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = now
	}
	if sess.UpdatedAt.IsZero() {
		sess.UpdatedAt = sess.CreatedAt
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sessions (id, project_id, directory, title, parent_id, created_at, updated_at, compacted_at, archived_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		sess.ID,
		sess.ProjectID,
		sess.Directory,
		sess.Title,
		sess.ParentID,
		sess.CreatedAt.Unix(),
		sess.UpdatedAt.Unix(),
		nullUnix(sess.CompactedAt),
		nullUnix(sess.ArchivedAt),
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetSession(ctx context.Context, id string) (Session, error) {
	var (
		sess                Session
		created, updated    int64
		compacted, archived sql.NullInt64
	)

	err := s.db.QueryRowContext(ctx, `
		SELECT id, project_id, directory, title, parent_id, created_at, updated_at, compacted_at, archived_at
		FROM sessions WHERE id = ?
	`, id).Scan(
		&sess.ID,
		&sess.ProjectID,
		&sess.Directory,
		&sess.Title,
		&sess.ParentID,
		&created,
		&updated,
		&compacted,
		&archived,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf("get session: %w", err)
	}

	sess.CreatedAt = time.Unix(created, 0).UTC()
	sess.UpdatedAt = time.Unix(updated, 0).UTC()
	if compacted.Valid {
		t := time.Unix(compacted.Int64, 0).UTC()
		sess.CompactedAt = &t
	}
	if archived.Valid {
		t := time.Unix(archived.Int64, 0).UTC()
		sess.ArchivedAt = &t
	}

	return sess, nil
}

func (s *SQLiteStore) ListSessions(ctx context.Context, projectID string) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, directory, title, parent_id, created_at, updated_at, compacted_at, archived_at
		FROM sessions
		WHERE project_id = ?
		ORDER BY updated_at DESC
	`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var out []Session
	for rows.Next() {
		var (
			sess                Session
			created, updated    int64
			compacted, archived sql.NullInt64
		)
		if err := rows.Scan(
			&sess.ID,
			&sess.ProjectID,
			&sess.Directory,
			&sess.Title,
			&sess.ParentID,
			&created,
			&updated,
			&compacted,
			&archived,
		); err != nil {
			return nil, fmt.Errorf("scan session row: %w", err)
		}
		sess.CreatedAt = time.Unix(created, 0).UTC()
		sess.UpdatedAt = time.Unix(updated, 0).UTC()
		if compacted.Valid {
			t := time.Unix(compacted.Int64, 0).UTC()
			sess.CompactedAt = &t
		}
		if archived.Valid {
			t := time.Unix(archived.Int64, 0).UTC()
			sess.ArchivedAt = &t
		}
		out = append(out, sess)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session rows: %w", err)
	}
	return out, nil
}

func (s *SQLiteStore) UpdateSession(ctx context.Context, sess Session) error {
	if sess.ID == "" {
		return errors.New("session id is required")
	}
	if sess.UpdatedAt.IsZero() {
		sess.UpdatedAt = time.Now().UTC()
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE sessions
		SET project_id = ?, directory = ?, title = ?, parent_id = ?, updated_at = ?, compacted_at = ?, archived_at = ?
		WHERE id = ?
	`,
		sess.ProjectID,
		sess.Directory,
		sess.Title,
		sess.ParentID,
		sess.UpdatedAt.Unix(),
		nullUnix(sess.CompactedAt),
		nullUnix(sess.ArchivedAt),
		sess.ID,
	)
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) ArchiveSession(ctx context.Context, id string) error {
	now := time.Now().UTC().Unix()
	res, err := s.db.ExecContext(ctx, `
		UPDATE sessions SET archived_at = ?, updated_at = ? WHERE id = ?
	`, now, now, id)
	if err != nil {
		return fmt.Errorf("archive session: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) AppendMessage(ctx context.Context, m Message) error {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO messages (id, session_id, role, agent_name, model_id, input_tokens, output_tokens, cost_usd, finish_reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		m.ID,
		m.SessionID,
		m.Role,
		m.AgentName,
		m.ModelID,
		m.InputTokens,
		m.OutputTokens,
		m.CostUSD,
		m.FinishReason,
		m.CreatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	_, _ = s.db.ExecContext(ctx, `UPDATE sessions SET updated_at = ? WHERE id = ?`, m.CreatedAt.Unix(), m.SessionID)
	return nil
}

func (s *SQLiteStore) AppendPart(ctx context.Context, p Part) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO parts (id, message_id, type, data, created_at)
		VALUES (?, ?, ?, ?, ?)
	`,
		p.ID,
		p.MessageID,
		string(p.Type),
		p.Data,
		p.CreatedAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("insert part: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdatePart(ctx context.Context, p Part) error {
	if p.ID == "" {
		return errors.New("part id is required")
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE parts SET message_id = ?, type = ?, data = ?, created_at = ? WHERE id = ?
	`, p.MessageID, string(p.Type), p.Data, p.CreatedAt.Unix(), p.ID)
	if err != nil {
		return fmt.Errorf("update part: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *SQLiteStore) GetMessages(ctx context.Context, sessionID string) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, session_id, role, agent_name, model_id, input_tokens, output_tokens, cost_usd, finish_reason, created_at
		FROM messages WHERE session_id = ? ORDER BY created_at ASC
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var m Message
		var created int64
		if err := rows.Scan(
			&m.ID,
			&m.SessionID,
			&m.Role,
			&m.AgentName,
			&m.ModelID,
			&m.InputTokens,
			&m.OutputTokens,
			&m.CostUSD,
			&m.FinishReason,
			&created,
		); err != nil {
			return nil, fmt.Errorf("scan message row: %w", err)
		}
		m.CreatedAt = time.Unix(created, 0).UTC()
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate message rows: %w", err)
	}
	return out, nil
}

func (s *SQLiteStore) GetParts(ctx context.Context, messageID string) ([]Part, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, message_id, type, data, created_at
		FROM parts WHERE message_id = ? ORDER BY created_at ASC
	`, messageID)
	if err != nil {
		return nil, fmt.Errorf("get parts: %w", err)
	}
	defer rows.Close()

	var out []Part
	for rows.Next() {
		var p Part
		var t string
		var created int64
		if err := rows.Scan(&p.ID, &p.MessageID, &t, &p.Data, &created); err != nil {
			return nil, fmt.Errorf("scan part row: %w", err)
		}
		p.Type = PartType(t)
		p.CreatedAt = time.Unix(created, 0).UTC()
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate part rows: %w", err)
	}
	return out, nil
}

func nullUnix(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Unix()
}
