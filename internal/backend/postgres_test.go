package backend

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

type fakeRow struct {
	value bool
	err   error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	ptr, ok := dest[0].(*bool)
	if !ok {
		return errors.New("expected *bool scan target")
	}
	*ptr = r.value
	return nil
}

type fakeQueryer struct {
	lastQuery string
	lastArgs  []any
	value     bool
	err       error
}

func (f *fakeQueryer) QueryRowContext(_ context.Context, query string, args ...any) rowScanner {
	f.lastQuery = query
	f.lastArgs = append([]any(nil), args...)
	return fakeRow{value: f.value, err: f.err}
}

func (f *fakeQueryer) Close() error { return nil }

func TestPostgresContainsUsesExactExistsQuery(t *testing.T) {
	db := &fakeQueryer{value: true}
	lookup := NewPostgres(db, []string{"net", "com"})

	got, err := lookup.Contains(context.Background(), "com", "example")
	if err != nil {
		t.Fatalf("Contains() error = %v", err)
	}
	if !got {
		t.Fatal("Contains() = false, want true")
	}
	if db.lastQuery != existsQuery {
		t.Fatalf("query = %q, want %q", db.lastQuery, existsQuery)
	}
	if !reflect.DeepEqual(db.lastArgs, []any{"com", "example"}) {
		t.Fatalf("args = %#v, want [com example]", db.lastArgs)
	}
	if !reflect.DeepEqual(lookup.ZoneNames(), []string{"com", "net"}) {
		t.Fatalf("ZoneNames() = %#v, want [com net]", lookup.ZoneNames())
	}
}

func TestOpenPostgresRequiresDSN(t *testing.T) {
	if _, err := OpenPostgres("", []string{"com"}, func(string, string) (*sql.DB, error) { return nil, nil }); err == nil {
		t.Fatal("OpenPostgres() error = nil, want missing DSN error")
	}
}
