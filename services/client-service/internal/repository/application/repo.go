package application

import "strings"

const defaultDocumentStorageDir = ".data/client-documents"

type Repository struct {
	db         DB
	storageDir string
}

func New(db DB, storageDirs ...string) *Repository {
	storageDir := defaultDocumentStorageDir
	if len(storageDirs) > 0 && strings.TrimSpace(storageDirs[0]) != "" {
		storageDir = storageDirs[0]
	}
	return &Repository{db: db, storageDir: storageDir}
}
