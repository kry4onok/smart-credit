package document

const MaxDocumentSize = 5 * 1024 * 1024

func validDocumentType(value string) bool {
	switch value {
	case "BANK_STATEMENT", "BUSINESS_PLAN", "ACCOUNTING_REPORT", "MANAGEMENT_REPORT", "TAX_DECLARATION_USN", "TAX_DECLARATION_3NDFL", "INCOME_LEDGER", "PATENT", "RECEIVABLES_PAYABLES":
		return true
	default:
		return false
	}
}
