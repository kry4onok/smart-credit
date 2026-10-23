package document

const MaxDocumentSize = 5 * 1024 * 1024

func validDocumentType(value string) bool {
	switch value {
	case "BANK_STATEMENT", "BUSINESS_PLAN":
		return true
	default:
		return false
	}
}
