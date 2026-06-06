package profile

import (
	"context"
	"database/sql"
	"fmt"
)

// X3 T4 #18: RLS project_id isolation.
//
// The 018 migration enables PostgreSQL Row-Level Security on the four
// project_id-scoped tables and defines policies that read
// `current_setting('app.current_project_id', true)::uuid` per connection.
//
// `RLSConn` is a thin wrapper around *sql.DB that lets callers execute a
// statement with a project_id pulled from the context. The wrapper starts a
// transaction, sets the GUC, runs the caller's query, and (for read queries)
// rolls the transaction back so the connection returns to the pool. lib/pq
// buffers the result rows client-side before QueryContext returns, so the
// caller can Scan them safely after the rollback.
//
// Context helpers:
//   - WithProjectID(ctx, projectID)  — stash a project id in the context
//   - ProjectIDFromContext(ctx)      — read it back ("" if absent)
//
// Usage:
//   ctx := profile.WithProjectID(r.Context(), projectID)
//   rows, err := of.RLSDB.QueryContext(ctx, "SELECT id FROM pipeline")
//   defer rows.Close()
//   ...
//   err := of.RLSDB.ExecContext(ctx, "INSERT INTO pipeline ...")
//
// For multi-statement transactions:
//   tx, err := of.RLSDB.Begin(ctx, nil)
//   ...
//   tx.Commit()

// contextKey is unexported to prevent collisions with other packages.
type contextKey int

const projectIDKey contextKey = iota

// WithProjectID returns a copy of ctx that carries projectID. Callers should
// pass the returned context to any query that should be filtered by RLS.
func WithProjectID(ctx context.Context, projectID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, projectIDKey, projectID)
}

// ProjectIDFromContext returns the project id stored in ctx, or "" if none.
func ProjectIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(projectIDKey).(string)
	return v
}

// RLSConn wraps a *sql.DB and provides helpers that run SET LOCAL
// app.current_project_id on every connection acquired from the pool.
type RLSConn struct {
	db *sql.DB
}

// NewRLSConn returns a wrapper around db that applies RLS scoping based on
// the project id found in the caller's context.
func NewRLSConn(db *sql.DB) *RLSConn {
	return &RLSConn{db: db}
}

// DB returns the underlying *sql.DB. Callers that cannot use the RLS-aware
// helpers (for example, code that operates on the migration DSN role and
// needs to bypass RLS) can use it directly.
func (r *RLSConn) DB() *sql.DB {
	return r.db
}

// applyProjectID runs SET LOCAL app.current_project_id = '...' on tx when
// projectID is non-empty. When projectID is empty the policy evaluates to
// NULL and rows are filtered out (because the project_id columns are NOT
// NULL on the protected tables); callers who want unrestricted access
// should use DB() directly.
func applyProjectID(ctx context.Context, tx *sql.Tx, projectID string) error {
	if projectID == "" {
		return nil
	}
	stmt := fmt.Sprintf("SET LOCAL app.current_project_id = %s", quoteLiteral(projectID))
	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("rls: set project_id: %w", err)
	}
	return nil
}

// quoteLiteral escapes a SQL string literal. The projectID is always a uuid
// or short text and contains no special characters in practice; we still
// double-quote single quotes defensively.
func quoteLiteral(s string) string {
	out := make([]byte, 0, len(s)+2)
	out = append(out, '\'')
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			out = append(out, '\'', '\'')
		} else {
			out = append(out, s[i])
		}
	}
	out = append(out, '\'')
	return string(out)
}

// Begin starts a transaction with the RLS project id applied. The returned
// *sql.Tx has SET LOCAL already executed, so any subsequent query in the
// same transaction sees the policy. Callers must Commit or Rollback.
func (r *RLSConn) Begin(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if r.db == nil {
		return nil, fmt.Errorf("rls: nil db")
	}
	tx, err := r.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	if err := applyProjectID(ctx, tx, ProjectIDFromContext(ctx)); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return tx, nil
}

// ExecContext runs a single statement inside a transaction with RLS applied,
// then commits. Returns sql.Result and any error.
func (r *RLSConn) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	tx, err := r.Begin(ctx, nil)
	if err != nil {
		return nil, err
	}
	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return res, nil
}

// QueryContext runs a SELECT inside a transaction with RLS applied. lib/pq
// buffers all result rows client-side before QueryContext returns, so we
// can safely roll the transaction back here — the caller can Scan the
// returned *sql.Rows to completion. The caller must Close the rows.
func (r *RLSConn) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	tx, err := r.Begin(ctx, nil)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	// Release the connection back to the pool. The rows have already been
	// buffered; closing rows later is a no-op.
	_ = tx.Rollback()
	return rows, nil
}
