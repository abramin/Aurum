package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Executor abstracts database operations that work with both pool and transaction.
// This follows the Qonto pattern of defining an interface with shared methods
// between *sql.DB and *sql.Tx (in our case, *pgxpool.Pool and pgx.Tx).
//
// See: https://medium.com/qonto-way/transactions-in-go-hexagonal-architecture-f12c7a817a61
type Executor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Verify that both pgxpool.Pool and pgx.Tx implement Executor.
var (
	_ Executor = (*pgxpool.Pool)(nil)
	_ Executor = (pgx.Tx)(nil)
)
