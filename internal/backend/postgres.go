package backend

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const existsQuery = `
SELECT EXISTS (
  SELECT 1
  FROM dm.zone_records
  WHERE zone_file = $1
    AND name = $2
);`

type rowScanner interface {
	Scan(dest ...any) error
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) rowScanner
	Close() error
}

type sqlQueryer struct {
	db *sql.DB
}

func (q sqlQueryer) QueryRowContext(ctx context.Context, query string, args ...any) rowScanner {
	return q.db.QueryRowContext(ctx, query, args...)
}

func (q sqlQueryer) Close() error {
	return q.db.Close()
}

// Postgres uses exact-match zone_file + name lookups against Domain Miner data.
type Postgres struct {
	db    queryer
	zones []string
}

// OpenPostgres opens a postgres-backed lookup using the provided DSN.
func OpenPostgres(dsn string, zones []string, opener func(driverName, dsn string) (*sql.DB, error)) (*Postgres, error) {
	if dsn == "" {
		return nil, fmt.Errorf("postgres backend requires a DSN")
	}
	if opener == nil {
		opener = sql.Open
	}
	db, err := opener("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres backend: %w", err)
	}
	return NewPostgres(sqlQueryer{db: db}, zones), nil
}

// NewPostgres creates a postgres-backed lookup from an injected query layer.
func NewPostgres(db queryer, zones []string) *Postgres {
	names := append([]string(nil), zones...)
	sort.Strings(names)
	return &Postgres{db: db, zones: names}
}

func (p *Postgres) ZoneNames() []string {
	if p == nil {
		return nil
	}
	return append([]string(nil), p.zones...)
}

func (p *Postgres) Contains(ctx context.Context, zone, stem string) (bool, error) {
	if p == nil || p.db == nil {
		return false, fmt.Errorf("postgres backend is not initialized")
	}
	var exists bool
	if err := p.db.QueryRowContext(ctx, existsQuery, zone, stem).Scan(&exists); err != nil {
		return false, fmt.Errorf("postgres exact lookup for %s/%s: %w", zone, stem, err)
	}
	return exists, nil
}

func (p *Postgres) Close() error {
	if p == nil || p.db == nil {
		return nil
	}
	return p.db.Close()
}
