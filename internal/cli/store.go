package cli

import (
	"context"
	"path/filepath"

	"github.com/valpere/kvach/internal/config"
	"github.com/valpere/kvach/internal/session"
)

func openSessionStore(ctx context.Context) (*session.SQLiteStore, error) {
	paths := config.ResolvePaths()
	dbPath := filepath.Join(paths.DataHome, "sessions.db")
	return session.NewSQLiteStore(ctx, dbPath)
}
