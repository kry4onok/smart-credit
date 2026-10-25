package documenthandler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/generated"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/domainerror"
	authmiddleware "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/middleware/auth"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"
	getcase "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/get"
)

const maxDocumentSize = 5 * 1024 * 1024

type CaseReader interface {
	Execute(context.Context, getcase.Input) (getcase.Output, error)
}

type Handler struct {
	cases   CaseReader
	client  *http.Client
	baseURL string
}

func New(cases CaseReader, baseURL string, timeout time.Duration) *Handler {
	return &Handler{cases: cases, client: &http.Client{Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}, baseURL: strings.TrimRight(baseURL, "/")}
}

func (h *Handler) Handle(ctx context.Context, request generated.GetCaseDocumentContentRequestObject) (generated.GetCaseDocumentContentResponseObject, error) {
	userID, hasUser := authmiddleware.UserID(ctx)
	token, hasToken := authmiddleware.AccessToken(ctx)
	if !hasUser || !hasToken {
		return nil, domainerror.Forbidden("FORBIDDEN", errors.New("требуется доступ сотрудника"))
	}
	caseItem, err := h.cases.Execute(ctx, getcase.Input{CaseID: request.ApplicationId, CurrentUserID: userID})
	if errors.Is(err, getcase.ErrNotFound) {
		return nil, domainerror.NotFound("CASE_NOT_FOUND", err)
	}
	if err != nil {
		return nil, fmt.Errorf("load case for document: %w", err)
	}
	var document *model.DocumentMetadata
	for index := range caseItem.Case.Documents {
		if caseItem.Case.Documents[index].ID == request.DocumentId {
			document = &caseItem.Case.Documents[index]
			break
		}
	}
	if document == nil {
		return nil, domainerror.NotFound("DOCUMENT_NOT_FOUND", errors.New("документ отсутствует в деле"))
	}
	path, err := url.JoinPath(h.baseURL, "internal/client/applications", request.ApplicationId.String(),
		"documents", request.DocumentId.String(), "content")
	if err != nil {
		return nil, fmt.Errorf("build client document URL: %w", err)
	}
	upstreamRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("create client document request: %w", err)
	}
	upstreamRequest.Header.Set("Authorization", "Bearer "+token)
	upstreamResponse, err := h.client.Do(upstreamRequest)
	if err != nil {
		return nil, unavailable()
	}
	defer func() { _ = upstreamResponse.Body.Close() }()
	if upstreamResponse.StatusCode != http.StatusOK || upstreamResponse.ContentLength > maxDocumentSize {
		return nil, unavailable()
	}
	content, err := io.ReadAll(io.LimitReader(upstreamResponse.Body, maxDocumentSize+1))
	if err != nil || len(content) > maxDocumentSize {
		return nil, unavailable()
	}
	checksum := sha256.Sum256(content)
	if hex.EncodeToString(checksum[:]) != document.SHA256 {
		return nil, unavailable()
	}
	return documentResponse{content: content, fileName: document.FileName}, nil
}

func unavailable() *domainerror.Error {
	return &domainerror.Error{Status: http.StatusBadGateway, Code: "DOCUMENT_UNAVAILABLE",
		Title: "Документ недоступен", Detail: "Не удалось получить исходный файл."}
}

type documentResponse struct {
	content  []byte
	fileName string
}

func (r documentResponse) VisitGetCaseDocumentContentResponse(w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": r.fileName}))
	w.Header().Set("Content-Length", fmt.Sprint(len(r.content)))
	w.WriteHeader(http.StatusOK)
	_, err := io.Copy(w, bytes.NewReader(r.content))
	return err
}
