package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/domain"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const documentIdempotencyRoutePrefix = "POST /client/applications/"

func (r *Repository) AddDocument(ctx context.Context, params model.AddApplicationDocumentParams) (model.ApplicationDocument, error) {
	contentHash := sha256.Sum256(params.Content)
	idempotencyRoute := documentIdempotencyRoutePrefix + params.ApplicationID.String() + "/documents"
	requestHash := sha256.Sum256([]byte(strings.Join([]string{
		params.ApplicationID.String(), params.DocumentType, params.FileName, hex.EncodeToString(contentHash[:]),
	}, ":")))

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return model.ApplicationDocument{}, fmt.Errorf("begin document upload: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var reserved int
	err = tx.QueryRow(ctx, `
		INSERT INTO idempotency_keys (
			actor_id, route, idempotency_key, request_hash, response_status,
			response_body, application_id, expires_at
		) VALUES ($1, $2, $3, $4, 201, '{}'::jsonb, $5, now() + interval '24 hours')
		ON CONFLICT (actor_id, route, idempotency_key) DO NOTHING
		RETURNING 1`, params.UserID, idempotencyRoute, params.IdempotencyKey, requestHash[:], params.ApplicationID).Scan(&reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		var storedHash []byte
		var responseBody []byte
		err = tx.QueryRow(ctx, `
			SELECT request_hash, response_body
			FROM idempotency_keys
			WHERE actor_id = $1 AND route = $2 AND idempotency_key = $3
			FOR UPDATE`, params.UserID, idempotencyRoute, params.IdempotencyKey).Scan(&storedHash, &responseBody)
		if err != nil {
			return model.ApplicationDocument{}, fmt.Errorf("load document idempotency record: %w", err)
		}
		if !bytesEqual(storedHash, requestHash[:]) {
			return model.ApplicationDocument{}, domain.ErrIdempotencyConflict
		}
		var document model.ApplicationDocument
		if err := json.Unmarshal(responseBody, &document); err != nil {
			return model.ApplicationDocument{}, fmt.Errorf("decode document idempotency response: %w", err)
		}
		if document.ID == uuid.Nil {
			return model.ApplicationDocument{}, errors.New("decode document idempotency response: empty document id")
		}
		return document, nil
	}
	if err != nil {
		return model.ApplicationDocument{}, fmt.Errorf("reserve document idempotency key: %w", err)
	}

	var status string
	var productDocumentTypes []string
	var infoDocumentTypes []string
	err = tx.QueryRow(ctx, `
		SELECT a.status,
		       COALESCE(ARRAY(
			       SELECT jsonb_array_elements_text(a.product_snapshot->'required_document_types')
		       ), '{}'::text[]),
		       COALESCE((
			       SELECT ir.required_document_types
			       FROM info_requests ir
			       WHERE ir.application_id = a.id AND ir.is_active
			       ORDER BY ir.created_at DESC, ir.id DESC
			       LIMIT 1
		       ), '{}'::text[])
		FROM applications a
		WHERE a.id = $1 AND a.org_id = $2
		FOR UPDATE`, params.ApplicationID, params.OrgID).Scan(&status, &productDocumentTypes, &infoDocumentTypes)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ApplicationDocument{}, domain.ErrApplicationNotFound
	}
	if err != nil {
		return model.ApplicationDocument{}, fmt.Errorf("load application for document upload: %w", err)
	}
	if status != "DRAFT" && status != "NEEDS_INFO" {
		return model.ApplicationDocument{}, domain.ErrApplicationDocumentState
	}
	if !contains(productDocumentTypes, params.DocumentType) && !contains(infoDocumentTypes, params.DocumentType) {
		return model.ApplicationDocument{}, domain.ErrApplicationDocumentType
	}

	documentID := uuid.New()
	storageKey := filepath.ToSlash(filepath.Join(params.ApplicationID.String(), documentID.String()))
	storagePath := filepath.Join(r.storageDir, filepath.FromSlash(storageKey))
	if err := writeDocument(storagePath, params.Content); err != nil {
		return model.ApplicationDocument{}, fmt.Errorf("store document: %w", err)
	}
	removeStoredDocument := true
	defer func() {
		if removeStoredDocument {
			_ = os.Remove(storagePath)
		}
	}()

	document := model.ApplicationDocument{
		ID: documentID, DocumentType: params.DocumentType, FileName: params.FileName,
		Sha256: hex.EncodeToString(contentHash[:]), IsMock: true, StorageKey: storageKey,
		SizeBytes: int64(len(params.Content)),
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO application_documents (
			id, application_id, document_type, file_name, storage_key,
			size_bytes, sha256, is_mock, uploaded_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, true, $8)
		RETURNING uploaded_at`, document.ID, params.ApplicationID, document.DocumentType,
		document.FileName, document.StorageKey, document.SizeBytes, document.Sha256, params.UserID).Scan(&document.UploadedAt); err != nil {
		return model.ApplicationDocument{}, fmt.Errorf("save document metadata: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE applications
		SET version = version + 1, updated_at = now()
		WHERE id = $1 AND org_id = $2`, params.ApplicationID, params.OrgID); err != nil {
		return model.ApplicationDocument{}, fmt.Errorf("update application after document upload: %w", err)
	}
	responseBody, err := json.Marshal(document)
	if err != nil {
		return model.ApplicationDocument{}, fmt.Errorf("encode document idempotency response: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE idempotency_keys
		SET response_body = $4, application_id = $5
		WHERE actor_id = $1 AND route = $2 AND idempotency_key = $3`,
		params.UserID, idempotencyRoute, params.IdempotencyKey, responseBody, params.ApplicationID); err != nil {
		return model.ApplicationDocument{}, fmt.Errorf("save document idempotency response: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return model.ApplicationDocument{}, fmt.Errorf("commit document upload: %w", err)
	}
	removeStoredDocument = false
	return document, nil
}

func writeDocument(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".document-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func bytesEqual(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
