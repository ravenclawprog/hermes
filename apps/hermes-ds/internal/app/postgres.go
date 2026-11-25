package app

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"hermes-messages"
)

//go:embed migrations/001_initial.sql
var initialSchema string

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("invalid HERMES_DS_DATABASE_URL")
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	if err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error { return migrate(ctx, tx) }); err != nil {
		pool.Close()
		return nil, fmt.Errorf("initialize PostgreSQL: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

func migrate(ctx context.Context, tx pgx.Tx) error {
	// Serialize schema setup when multiple service instances start together.
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(476391805, 1)"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "CREATE TABLE IF NOT EXISTS hermes_schema_migrations (version integer PRIMARY KEY)"); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, "INSERT INTO hermes_schema_migrations VALUES (1) ON CONFLICT DO NOTHING")
	if err != nil || tag.RowsAffected() == 0 {
		return err
	}
	_, err = tx.Exec(ctx, initialSchema, pgx.QueryExecModeSimpleProtocol)
	return err
}

func (p *Postgres) Close() {
	p.pool.Close()
}

func (p *Postgres) ListDocuments(ctx context.Context, workspaceID, after string, limit int) ([]messages.DocumentSummary, bool, error) {
	// A left join distinguishes an empty workspace from an inaccessible one.
	rows, err := p.pool.Query(ctx, `
		SELECT w.id, d.id, COALESCE(d.title, ''), COALESCE(d.version, 0)
		FROM workspaces w
		LEFT JOIN documents d ON d.workspace_id = w.id AND d.deleted_at IS NULL AND d.id > $3
		WHERE w.owner_id = $1 AND ($2 = '' OR w.id = $2)
		ORDER BY d.id NULLS LAST
		LIMIT $4`, demoUserID, workspaceID, after, limit)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	items := []messages.DocumentSummary{}
	found := workspaceID == ""
	for rows.Next() {
		var item messages.DocumentSummary
		var id *string
		if err := rows.Scan(&item.WorkspaceID, &id, &item.Title, &item.Version); err != nil {
			return nil, false, err
		}
		found = true
		if id != nil {
			item.ID = *id
			items = append(items, item)
		}
	}
	return items, found, rows.Err()
}

func (p *Postgres) GetDocument(ctx context.Context, id string) (messages.Document, bool, error) {
	var doc messages.Document
	err := p.pool.QueryRow(ctx, `
		SELECT d.id, d.workspace_id, d.title, d.version, d.content
		FROM documents d JOIN workspaces w ON w.id = d.workspace_id
		WHERE d.id = $1 AND w.owner_id = $2 AND d.deleted_at IS NULL`, id, demoUserID).
		Scan(&doc.ID, &doc.WorkspaceID, &doc.Title, &doc.Version, &doc.Content)
	if errors.Is(err, pgx.ErrNoRows) {
		return doc, false, nil
	}
	return doc, err == nil, err
}
