package postgres

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/alfabank-hackathon-2026/smart-credit/services/auth/internal/domain"
	"github.com/jackc/pgx/v5/pgconn"
)

// OperationError marks unavailable connections and retryable database failures
// without exposing driver-specific error types to the HTTP layer.
func OperationError(operation string, err error) error {
	var network net.Error
	var database *pgconn.PgError
	unavailable := errors.As(err, &network) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
	if errors.As(err, &database) {
		unavailable = strings.HasPrefix(database.Code, "08")
		switch database.Code {
		case "40001", "40P01", "53300", "57P01", "57P02", "57P03":
			unavailable = true
		}
	}
	if unavailable {
		err = errors.Join(domain.ErrUnavailable, err)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
