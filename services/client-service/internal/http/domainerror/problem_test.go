package domainerror

import (
	"context"
	"testing"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/requirements"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
)

func TestValidationDetailPreservesMachineCode(t *testing.T) {
	for _, tc := range []struct {
		cause error
		code  string
	}{{domain.ErrInvalidApplication, "INVALID_APPLICATION"}, {domain.ErrInvalidInfoResponse, "INVALID_INFO_RESPONSE"}, {domain.ErrInvalidDocumentUpload, "INVALID_DOCUMENT"}} {
		problem := Map(t.Context(), domain.InvalidFields(tc.cause, "Укажите сумму кредита.", "Приложите выписку."))
		if problem.Status != 422 || problem.Code != tc.code || problem.Detail == nil || *problem.Detail != "Укажите сумму кредита. Приложите выписку." || problem.TraceId == "" {
			t.Fatal("validation response lost actionable detail or contract fields")
		}
	}
}

func TestProfileRuleAndConflictCodes(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{&requirements.RuleError{Code: requirements.InvalidTaxPeriod}, 422, "INVALID_TAX_PERIOD"},
		{domain.ErrProfileIdentityImmutable, 409, "PROFILE_IDENTITY_IMMUTABLE"},
		{domain.ErrStaleInfoRequest, 409, "STALE_INFO_REQUEST"},
		{&requirements.RuleError{Code: "INTERNAL_PRIVATE_CODE"}, 500, "INTERNAL_ERROR"},
	} {
		p := Map(context.Background(), tc.err)
		if p.Status != tc.status || p.Code != tc.code {
			t.Fatalf("problem=%+v", p)
		}
	}
}
