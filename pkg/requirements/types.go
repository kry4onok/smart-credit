package requirements

import (
	"time"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
)

type Input struct {
	Company       *commonpb.CompanySnapshot
	Product       *commonpb.Product
	Form          *commonpb.ApplicationForm
	Documents     []*commonpb.DocumentMetadata
	ReferenceDate time.Time
}

type Issue struct{ Path, Code, Message string }

const (
	InvalidProfile            = "INVALID_PROFILE"
	LegalFormINNMismatch      = "LEGAL_FORM_INN_MISMATCH"
	InvalidTaxPeriod          = "INVALID_TAX_PERIOD"
	UnsupportedTaxCombination = "UNSUPPORTED_TAX_COMBINATION"
	RequirementsNotMet        = "REQUIREMENTS_NOT_MET"
	InvalidDocumentCoverage   = "INVALID_DOCUMENT_COVERAGE"
	UnknownRequirement        = "UNKNOWN_REQUIREMENT"
)
