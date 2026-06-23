package memstore

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mayangzz/My-Agent/harness"
)

// Postgres 把每条消息作为一行 jsonb 存下来,落盘持久、可查询。默认后端。
type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	const method = "memstore.NewPostgres"
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("method=%s connect: %w", method, err)
	}
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS agent_messages (
			session    text   NOT NULL,
			seq        bigserial,
			data       jsonb  NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS idx_agent_messages_session ON agent_messages(session, seq);`); err != nil {
		pool.Close()
		return nil, fmt.Errorf("method=%s init schema: %w", method, err)
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Append(ctx context.Context, session string, msg harness.Message) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("method=Postgres.Append marshal: %w", err)
	}
	_, err = p.pool.Exec(ctx, `INSERT INTO agent_messages(session, data) VALUES ($1, $2::jsonb)`, session, string(b))
	if err != nil {
		return fmt.Errorf("method=Postgres.Append insert: %w", err)
	}
	return nil
}

func (p *Postgres) Load(ctx context.Context, session string) ([]harness.Message, error) {
	rows, err := p.pool.Query(ctx, `SELECT data FROM agent_messages WHERE session=$1 ORDER BY seq`, session)
	if err != nil {
		return nil, fmt.Errorf("method=Postgres.Load query: %w", err)
	}
	defer rows.Close()

	var out []harness.Message
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("method=Postgres.Load scan: %w", err)
		}
		var m harness.Message
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("method=Postgres.Load unmarshal: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (p *Postgres) Reset(ctx context.Context, session string) error {
	_, err := p.pool.Exec(ctx, `DELETE FROM agent_messages WHERE session=$1`, session)
	if err != nil {
		return fmt.Errorf("method=Postgres.Reset delete: %w", err)
	}
	return nil
}
