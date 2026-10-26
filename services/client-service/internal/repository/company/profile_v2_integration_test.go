package company

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/profile"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/testutil"
	"github.com/google/uuid"
)

func TestV2NullableFinancialsAndIdentity(t *testing.T) {
	db := testutil.Postgres(t)
	repo := New(db)
	org := uuid.New()
	c := model.Company{INN: "990000000115", Name: "ИП Демо", OKVED: "47.11", RegisteredAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), ProfileDetails: model.ProfileDetails{ProfileSchemaVersion: profile.Ptr(int32(2)), LegalForm: profile.Ptr("IP")}}
	saved, err := repo.Create(t.Context(), org, c)
	if err != nil {
		t.Fatal(err)
	}
	for _, amount := range []*int64{profile.Ptr(int64(0)), profile.Ptr(int64(-100)), nil} {
		saved.Profit12mKopecks = amount
		saved, err = repo.Update(t.Context(), org, saved, saved.Version)
		if err != nil {
			t.Fatal(err)
		}
		loaded, err := repo.Fetch(t.Context(), org)
		if err != nil {
			t.Fatal(err)
		}
		if amount == nil {
			if loaded.Profit12mKopecks != nil {
				t.Fatal("missing became zero")
			}
		} else if loaded.Profit12mKopecks == nil || *loaded.Profit12mKopecks != *amount {
			t.Fatal("zero or loss changed")
		}
	}
	altered := saved
	altered.INN = "500100732259"
	if _, err := repo.Update(t.Context(), org, altered, saved.Version); !errors.Is(err, domain.ErrProfileIdentityImmutable) {
		t.Fatalf("identity=%v", err)
	}
	if _, err := repo.Update(t.Context(), org, saved, 1); !errors.Is(err, domain.ErrCompanyVersionConflict) {
		t.Fatalf("version=%v", err)
	}
	if _, err := repo.Fetch(t.Context(), uuid.New()); !errors.Is(err, domain.ErrCompanyNotFound) {
		t.Fatalf("foreign org=%v", err)
	}
}
func TestProfileMigrationPreservesLegacyAndRejectsLossyDown(t *testing.T) {
	db := testutil.Postgres(t)
	migration, err := os.ReadFile("../../../migrations/00002_profile_requirements.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, down, ok := strings.Cut(string(migration), "-- +goose Down")
	if !ok {
		t.Fatal("missing down")
	}
	if _, err := db.Exec(t.Context(), down); err != nil {
		t.Fatal(err)
	}
	org := uuid.New()
	if _, err := db.Exec(t.Context(), `INSERT INTO companies(org_id,inn,name,okved,registered_at,revenue_12m_kopecks,profit_12m_kopecks,obligations_kopecks) VALUES($1,'990000000115','Legacy','47.11','2020-01-01',0,-100,0)`, org); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(t.Context(), up); err != nil {
		t.Fatal(err)
	}
	repo := New(db)
	c, err := repo.Fetch(t.Context(), org)
	if err != nil {
		t.Fatal(err)
	}
	if c.ProfileSchemaVersion != nil || c.LegalForm != nil || c.Profit12mKopecks == nil || *c.Profit12mKopecks != -100 {
		t.Fatal("migration invented metadata or lost values")
	}
	c.ProfileSchemaVersion = profile.Ptr(int32(2))
	c.Profit12mKopecks = nil
	if _, err := repo.Update(t.Context(), org, c, c.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(t.Context(), down); err == nil {
		t.Fatal("lossy rollback accepted")
	}
	c, err = repo.Fetch(t.Context(), org)
	if err != nil || c.Profit12mKopecks != nil || c.ProfileSchemaVersion == nil {
		t.Fatalf("rollback changed data %v", err)
	}
}
