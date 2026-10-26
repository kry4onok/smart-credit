package assistanthelp

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/httpjson"
	pb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/assistant"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
)

type SelectionRequest struct {
	ContextID      string   `json:"context_id"`
	SourceRevision string   `json:"source_revision"`
	CatalogVersion string   `json:"catalog_version"`
	Mode           string   `json:"mode"`
	Detail         string   `json:"detail"`
	TargetIDs      []string `json:"target_ids"`
	Question       string   `json:"question"`
}

// ServeHTTP reloads the authorized snapshot after inference. No response based
// on an obsolete snapshot is delivered, even if the model answered successfully.
func ServeHTTP(w http.ResponseWriter, r *http.Request, target string, audience pb.Audience, load func(context.Context) (View, error)) {
	v, err := load(r.Context())
	if err != nil {
		httpjson.Problem(w, 404, "APPLICATION_NOT_FOUND")
		return
	}
	c := Context(v, audience, time.Now().UTC())
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		data, e := (protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}).Marshal(c)
		if e != nil {
			httpjson.Problem(w, 500, "INTERNAL_ERROR")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
		return
	}
	if r.Method != http.MethodPost {
		httpjson.Problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	var input SelectionRequest
	if httpjson.Read(w, r, &input) != nil {
		httpjson.Problem(w, 400, "INVALID_HELP_REQUEST")
		return
	}
	if input.CatalogVersion != CatalogVersion {
		httpjson.Problem(w, 409, "UNSUPPORTED_CATALOG_VERSION")
		return
	}
	if input.ContextID != c.ContextId || input.SourceRevision != c.SourceRevision {
		httpjson.Problem(w, 409, "CONTEXT_STALE")
		return
	}
	req := &pb.HelpRequest{RequestId: uuid.NewString(), Context: c, Mode: pb.HelpMode(pb.HelpMode_value["HELP_MODE_"+input.Mode]), Detail: pb.Detail(pb.Detail_value["DETAIL_"+input.Detail]), Locale: "ru-RU", TargetIds: input.TargetIDs, Question: input.Question}
	if Validate(req, time.Now().UTC()) != nil {
		httpjson.Problem(w, 400, "INVALID_HELP_REQUEST")
		return
	}
	answer, err := Call(r.Context(), target, req)
	if err != nil {
		code := "HELP_INCOMPATIBLE"
		if errors.Is(err, ErrExpired) {
			code = "CONTEXT_STALE"
		}
		if errors.Is(err, ErrIncompatible) {
			code = "UNSUPPORTED_CATALOG_VERSION"
		}
		httpjson.Problem(w, 409, code)
		return
	}
	latest, err := load(r.Context())
	if err != nil {
		httpjson.Problem(w, 404, "APPLICATION_NOT_FOUND")
		return
	}
	if Context(latest, audience, time.Now().UTC()).ContextId != c.ContextId || !c.ValidUntil.AsTime().After(time.Now()) {
		httpjson.Problem(w, 409, "CONTEXT_STALE")
		return
	}
	data, err := (protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}).Marshal(answer)
	if err != nil {
		httpjson.Problem(w, 500, "INTERNAL_ERROR")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}
