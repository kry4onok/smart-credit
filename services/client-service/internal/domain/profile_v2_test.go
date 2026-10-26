package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/requirements"
	fixtures "github.com/alfabank-hackathon-2026/smart-credit/seed/profile-requirements"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/profile"
)

func TestV2FixtureValidation(t *testing.T) {
	data, err := fixtures.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range data.Cases {
		t.Run(s.ID, func(t *testing.T) {
			ref, err := time.Parse(time.DateOnly, s.ReferenceDate)
			if err != nil {
				t.Fatal(err)
			}
			err = ValidateCompanyAt(profile.FromHTTP(s.Profile), ref)
			if s.Expected.ValidationError == nil {
				if err != nil {
					t.Fatal(err)
				}
			} else {
				var r *requirements.RuleError
				if !errors.As(err, &r) || r.Code != *s.Expected.ValidationError {
					t.Fatalf("error=%v want=%s", err, *s.Expected.ValidationError)
				}
			}
		})
	}
}
