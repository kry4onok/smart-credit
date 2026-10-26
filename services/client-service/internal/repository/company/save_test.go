package company

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	profileconv "github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/profile"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestCreateCompanyMapsConflict(t *testing.T) {
	orgID := uuid.New()
	profile := testCompany()
	for _, tc := range []struct {
		name    string
		scanErr error
		wantErr error
	}{
		{"created", nil, nil},
		{"already exists", pgx.ErrNoRows, domain.ErrCompanyAlreadyExists},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := New(dbMock{query: func(_ context.Context, query string, args ...any) pgx.Row {
				if !strings.Contains(query, "ON CONFLICT (org_id) DO NOTHING") || args[0] != orgID {
					t.Fatalf("unexpected insert query: %s", query)
				}
				return rowMock{scan: func(dest ...any) error {
					if tc.scanErr != nil {
						return tc.scanErr
					}
					fillCompany(dest, profile, 1)
					return nil
				}}
			}})
			result, err := repo.Create(context.Background(), orgID, profile)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if err == nil && result.Version != 1 {
				t.Fatalf("version = %d", result.Version)
			}
		})
	}
}

func TestUpdateCompanyUsesExpectedVersion(t *testing.T) {
	orgID := uuid.New()
	profile := testCompany()
	repo := New(dbMock{query: func(_ context.Context, query string, args ...any) pgx.Row {
		if !strings.Contains(query, "WHERE org_id = $1 AND version = $9") || args[0] != orgID || args[8] != int64(4) {
			t.Fatalf("unexpected update query: %s args=%v", query, args)
		}
		return rowMock{scan: func(dest ...any) error {
			fillCompany(dest, profile, 5)
			return nil
		}}
	}})
	result, err := repo.Update(context.Background(), orgID, profile, 4)
	if err != nil || result.Version != 5 {
		t.Fatalf("result = %+v, err = %v", result, err)
	}
}

func testCompany() model.Company {
	return model.Company{INN: "7707083893", Name: "Демо", OKVED: "62.01", RegisteredAt: time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC), Revenue12mKopecks: profileconv.Ptr(int64(100)), Profit12mKopecks: profileconv.Ptr(int64(-10)), ObligationsKopecks: profileconv.Ptr(int64(20))}
}

func fillCompany(dest []any, profile model.Company, version int64) {
	*dest[0].(*string) = profile.INN
	*dest[1].(*string) = profile.Name
	*dest[2].(*string) = profile.OKVED
	*dest[3].(*time.Time) = profile.RegisteredAt
	*dest[4].(**int64) = profile.Revenue12mKopecks
	*dest[5].(**int64) = profile.Profit12mKopecks
	*dest[6].(**int64) = profile.ObligationsKopecks
	*dest[7].(*int64) = version
}
