package transaction

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type beginnerStub struct {
	tx       pgx.Tx
	err      error
	beginCnt int
}

func (s *beginnerStub) Begin(context.Context) (pgx.Tx, error) {
	s.beginCnt++
	return s.tx, s.err
}

type txStub struct {
	pgx.Tx
	commitErr          error
	rollbackErr        error
	commitCnt          int
	rollbackCnt        int
	execCnt            int
	closed             bool
	rollbackContextErr error
}

func (s *txStub) Commit(context.Context) error {
	s.commitCnt++
	if s.commitErr == nil {
		s.closed = true
	}
	return s.commitErr
}

func (s *txStub) Rollback(ctx context.Context) error {
	s.rollbackCnt++
	s.rollbackContextErr = ctx.Err()
	if s.closed {
		return pgx.ErrTxClosed
	}
	if s.rollbackErr == nil {
		s.closed = true
	}
	return s.rollbackErr
}

func (s *txStub) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	s.execCnt++
	return pgconn.CommandTag{}, nil
}

type queryerStub struct {
	Queryer
}

func TestWithinTransactionCommitsAndRoutesQueries(t *testing.T) {
	tx := &txStub{}
	beginner := &beginnerStub{tx: tx}
	manager := New(beginner)
	fallback := &queryerStub{}
	if got := QueryerFromContext(context.Background(), fallback); got != fallback {
		t.Fatalf("queryer outside transaction = %T, want fallback", got)
	}

	err := manager.WithinTransaction(context.Background(), func(ctx context.Context) error {
		queryer := QueryerFromContext(ctx, fallback)
		if queryer != tx {
			t.Fatalf("queryer inside transaction = %T, want transaction", queryer)
		}
		_, execErr := queryer.Exec(ctx, "INSERT INTO cases ...")
		return execErr
	})
	if err != nil {
		t.Fatalf("WithinTransaction() error = %v", err)
	}
	if beginner.beginCnt != 1 || tx.execCnt != 1 || tx.commitCnt != 1 || tx.rollbackCnt != 0 {
		t.Fatalf("begin=%d exec=%d commit=%d rollback=%d, want 1/1/1/0",
			beginner.beginCnt, tx.execCnt, tx.commitCnt, tx.rollbackCnt)
	}
}

func TestWithinTransactionRollsBackOnCallbackError(t *testing.T) {
	tx := &txStub{}
	manager := New(&beginnerStub{tx: tx})
	wantErr := errors.New("write failed")

	err := manager.WithinTransaction(context.Background(), func(context.Context) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) || tx.commitCnt != 0 || tx.rollbackCnt != 1 {
		t.Fatalf("error=%v commit=%d rollback=%d", err, tx.commitCnt, tx.rollbackCnt)
	}
}

func TestWithinTransactionReportsBeginAndCommitErrors(t *testing.T) {
	beginErr := errors.New("begin failed")
	beginner := &beginnerStub{err: beginErr}
	err := New(beginner).WithinTransaction(context.Background(), func(context.Context) error {
		t.Fatal("callback must not run when begin fails")
		return nil
	})
	if !errors.Is(err, beginErr) {
		t.Fatalf("begin error = %v", err)
	}

	commitErr := errors.New("commit failed")
	tx := &txStub{commitErr: commitErr}
	err = New(&beginnerStub{tx: tx}).WithinTransaction(context.Background(), func(context.Context) error {
		return nil
	})
	if !errors.Is(err, commitErr) || tx.commitCnt != 1 || tx.rollbackCnt != 1 {
		t.Fatalf("commit error=%v commit=%d rollback=%d", err, tx.commitCnt, tx.rollbackCnt)
	}
}

func TestWithinTransactionJoinsRollbackError(t *testing.T) {
	callbackErr := errors.New("callback failed")
	rollbackErr := errors.New("rollback failed")
	tx := &txStub{rollbackErr: rollbackErr}
	err := New(&beginnerStub{tx: tx}).WithinTransaction(context.Background(), func(context.Context) error {
		return callbackErr
	})
	if !errors.Is(err, callbackErr) || !errors.Is(err, rollbackErr) {
		t.Fatalf("error = %v, want callback and rollback errors", err)
	}
}

func TestWithinTransactionRejectsNesting(t *testing.T) {
	tx := &txStub{}
	beginner := &beginnerStub{tx: tx}
	manager := New(beginner)
	err := manager.WithinTransaction(context.Background(), func(ctx context.Context) error {
		nestedErr := manager.WithinTransaction(ctx, func(context.Context) error {
			t.Fatal("nested callback must not run")
			return nil
		})
		if !errors.Is(nestedErr, ErrNested) {
			t.Fatalf("nested error = %v, want ErrNested", nestedErr)
		}
		return nil
	})
	if err != nil || beginner.beginCnt != 1 {
		t.Fatalf("error=%v begin=%d, want one transaction", err, beginner.beginCnt)
	}
}

func TestWithinTransactionRollsBackAfterCancellation(t *testing.T) {
	tx := &txStub{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := New(&beginnerStub{tx: tx}).WithinTransaction(ctx, func(context.Context) error {
		cancel()
		return context.Canceled
	})
	if !errors.Is(err, context.Canceled) || tx.rollbackCnt != 1 || tx.rollbackContextErr != nil {
		t.Fatalf("error=%v rollback=%d rollback context error=%v",
			err, tx.rollbackCnt, tx.rollbackContextErr)
	}
}

func TestWithinTransactionRollsBackOnPanic(t *testing.T) {
	tx := &txStub{}
	defer func() {
		if recovered := recover(); recovered != "callback panic" {
			t.Fatalf("recovered = %v, want callback panic", recovered)
		}
		if tx.commitCnt != 0 || tx.rollbackCnt != 1 {
			t.Fatalf("commit=%d rollback=%d, want 0/1", tx.commitCnt, tx.rollbackCnt)
		}
	}()

	_ = New(&beginnerStub{tx: tx}).WithinTransaction(context.Background(), func(context.Context) error {
		panic("callback panic")
	})
}
