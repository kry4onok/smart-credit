package postgres

import (
	"context"
	"errors"
	"io"
	"net"
	"syscall"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/domain"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestOperationErrorClassifiesAvailability(t *testing.T) {
	for _, tc := range []struct {
		err         error
		unavailable bool
	}{
		{&net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}, true},
		{context.DeadlineExceeded, true}, {io.EOF, true}, {io.ErrUnexpectedEOF, true},
		{&pgconn.PgError{Code: "08006"}, true}, {&pgconn.PgError{Code: "57P01"}, true},
		{&pgconn.PgError{Code: "57P03"}, true}, {&pgconn.PgError{Code: "53300"}, true},
		{&pgconn.PgError{Code: "40001"}, true}, {&pgconn.PgError{Code: "40P01"}, true},
		{&pgconn.PgError{Code: "23514"}, false}, {&pgconn.PgError{Code: "42P01"}, false},
		{errors.New("private database error"), false},
	} {
		err := OperationError("test operation", tc.err)
		if errors.Is(err, domain.ErrUnavailable) != tc.unavailable || !errors.Is(err, tc.err) {
			t.Fatal("database failure incorrectly classified or original error lost")
		}
	}
}
