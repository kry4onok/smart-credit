package model

import "github.com/google/uuid"

type GetDocumentContentParams struct {
	ApplicationID uuid.UUID
	DocumentID    uuid.UUID
}

// DocumentContent contains no filesystem keys or paths.
type DocumentContent struct {
	FileName    string
	ContentType string
	Content     []byte
}
