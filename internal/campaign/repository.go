package campaign

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/puddle/v2"
	db "pulsebid/gen/campaigndb"
)

// Repository bounds queries and owns read/write transaction boundaries.
type Repository interface {
	Read(context.Context, func(context.Context, *db.Queries) error) error
	Write(context.Context, func(context.Context, *db.Queries) error) error
}

// Store adapts pgxpool and generated queries to the domain repository.
type Store struct{ pool *pgxpool.Pool }

// NewStore uses a caller-owned connection pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Read runs a bounded read against the pool.
func (s *Store) Read(ctx context.Context, fn func(context.Context, *db.Queries) error) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return databaseError(fn(ctx, db.New(s.pool)))
}

// Write serializes read-modify-write operations with the queries' row locks.
func (s *Store) Write(ctx context.Context, fn func(context.Context, *db.Queries) error) (result error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return databaseError(err)
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		err := tx.Rollback(cleanup)
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			result = errors.Join(result, databaseError(err))
		}
	}()
	if err = fn(ctx, db.New(tx)); err != nil {
		return databaseError(err)
	}
	return databaseError(tx.Commit(ctx))
}

func databaseError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "23503":
			return fmt.Errorf("%w: %s", ErrConflict, pgErr.Code)
		case "23514", "23502", "22P02":
			return fmt.Errorf("%w: stored constraint", ErrValidation)
		}
		if strings.HasPrefix(pgErr.Code, "08") || strings.HasPrefix(pgErr.Code, "53") || strings.HasPrefix(pgErr.Code, "57") || pgErr.Code == "40P01" || pgErr.Code == "55P03" {
			return fmt.Errorf("%w: %s", ErrUnavailable, pgErr.Code)
		}
	}
	var netErr net.Error
	var connectErr *pgconn.ConnectError
	if errors.As(err, &netErr) || errors.As(err, &connectErr) || pgconn.SafeToRetry(err) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, puddle.ErrClosedPool) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return err
}
