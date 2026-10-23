package domain

import "errors"

var (
	ErrCompanyNotFound            = errors.New("company not found")
	ErrCompanyAlreadyExists       = errors.New("company already exists")
	ErrCompanyVersionConflict     = errors.New("company version conflict")
	ErrInvalidCompanyProfile      = errors.New("invalid company profile")
	ErrInvalidIfMatch             = errors.New("invalid if-match")
	ErrProductNotFound            = errors.New("product not found")
	ErrApplicationNotFound        = errors.New("application not found")
	ErrApplicationVersionConflict = errors.New("application version conflict")
	ErrApplicationNotDraft        = errors.New("application is not a draft")
	ErrInvalidApplicationPatch    = errors.New("invalid application patch")
	ErrApplicationDocumentState   = errors.New("application does not accept documents")
	ErrApplicationDocumentType    = errors.New("document type is not allowed")
	ErrInvalidDocumentUpload      = errors.New("invalid document upload")
	ErrIdempotencyConflict        = errors.New("idempotency conflict")
	ErrInvalidApplication         = errors.New("invalid application")
)
