package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) GetDocumentContent(ctx context.Context, params model.GetDocumentContentParams) (model.DocumentContent, error) {
	var document model.DocumentContent
	var storageKey string
	var sizeBytes int64
	err := r.db.QueryRow(ctx, `
		SELECT d.file_name, d.storage_key, d.size_bytes
		FROM application_documents d
		JOIN applications a ON a.id = d.application_id
		WHERE d.id = $2 AND d.application_id = $1
		  AND a.submitted_at IS NOT NULL AND a.status <> 'DRAFT'`,
		params.ApplicationID, params.DocumentID).Scan(&document.FileName, &storageKey, &sizeBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.DocumentContent{}, domain.ErrDocumentNotFound
	}
	if err != nil {
		return model.DocumentContent{}, fmt.Errorf("fetch document content metadata: %w", err)
	}
	// Uploads always use application_id/document_id. Do not accept an arbitrary
	// filesystem path from a damaged metadata record.
	expectedKey := filepath.ToSlash(filepath.Join(params.ApplicationID.String(), params.DocumentID.String()))
	if storageKey != expectedKey {
		return model.DocumentContent{}, domain.ErrDocumentNotFound
	}
	root, err := os.OpenRoot(r.storageDir)
	if errors.Is(err, os.ErrNotExist) {
		return model.DocumentContent{}, domain.ErrDocumentNotFound
	}
	if err != nil {
		return model.DocumentContent{}, fmt.Errorf("open document storage: %w", err)
	}
	defer func() { _ = root.Close() }()
	name := filepath.FromSlash(storageKey)
	info, err := root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) || (err == nil && !info.Mode().IsRegular()) {
		return model.DocumentContent{}, domain.ErrDocumentNotFound
	}
	if err != nil {
		return model.DocumentContent{}, fmt.Errorf("stat document content: %w", err)
	}
	// OpenRoot prevents escaping the volume through symlinks in parent paths.
	file, err := root.Open(name)
	if errors.Is(err, os.ErrNotExist) {
		return model.DocumentContent{}, domain.ErrDocumentNotFound
	}
	if err != nil {
		return model.DocumentContent{}, fmt.Errorf("open document content: %w", err)
	}
	defer func() { _ = file.Close() }()
	document.Content, err = io.ReadAll(io.LimitReader(file, sizeBytes+1))
	if err != nil {
		return model.DocumentContent{}, fmt.Errorf("read document content: %w", err)
	}
	if int64(len(document.Content)) != sizeBytes {
		return model.DocumentContent{}, errors.New("document content size does not match metadata")
	}
	return document, nil
}
