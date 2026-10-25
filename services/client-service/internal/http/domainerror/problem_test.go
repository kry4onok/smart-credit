package domainerror

import (
	"testing"

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
