package company

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestFetchScopesQueryAndMapsMissingProfile(t *testing.T) {
	orgID := uuid.New()
	for _, tc := range []struct {
		name           string
		dbErr, wantErr error
	}{
		{"found", nil, nil}, {"missing", pgx.ErrNoRows, domain.ErrCompanyNotFound},
		{"failure", context.DeadlineExceeded, context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := New(dbMock{query: func(_ context.Context, query string, args ...any) pgx.Row {
				if !strings.Contains(query, "WHERE org_id = $1") || len(args) != 1 || args[0] != orgID {
					t.Fatalf("unscoped query: %s %v", query, args)
				}
				return rowMock{scan: func(dest ...any) error {
					if tc.dbErr != nil {
						return tc.dbErr
					}
					*dest[0].(*string) = "7707083893"
					*dest[1].(*string) = "Company"
					*dest[2].(*string) = "62.01"
					*dest[3].(*time.Time) = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
					*dest[4].(*int64) = 100
					*dest[5].(*int64) = -50
					*dest[6].(*int64) = 20
					*dest[7].(*int64) = 3
					return nil
				}}
			}})
			result, err := repo.Fetch(context.Background(), orgID)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v", err)
			}
			if err == nil && (result.Version != 3 || result.Profit12mKopecks != -50) {
				t.Fatalf("company = %+v", result)
			}
		})
	}
}
