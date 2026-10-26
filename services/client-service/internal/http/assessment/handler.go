package assessmenthandler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/assessment"
	"github.com/alfabank-hackathon-2026/smart-credit/pkg/assistanthelp"
	"github.com/alfabank-hackathon-2026/smart-credit/pkg/httpjson"
	pb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/assistant"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/client-service/internal/http/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"
)

type Handler struct {
	db        *pgxpool.Pool
	assistant string
}

func New(db *pgxpool.Pool, target string) *Handler { return &Handler{db, target} }
func (h *Handler) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 4 || parts[0] != "client" || parts[1] != "applications" || (parts[3] != "assessment-input" && parts[3] != "assessment" && parts[3] != "help") {
			next.ServeHTTP(w, r)
			return
		}
		principal, ok := middleware.Client(r.Context())
		if !ok {
			httpjson.Problem(w, 401, "UNAUTHORIZED")
			return
		}
		id, err := uuid.Parse(parts[2])
		if err != nil {
			httpjson.Problem(w, 400, "INVALID_APPLICATION_ID")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		if parts[3] == "help" {
			assistanthelp.ServeHTTP(w, r, h.assistant, pb.Audience_AUDIENCE_CLIENT, func(ctx context.Context) (assistanthelp.View, error) {
				_, v, e := h.load(ctx, id, principal.OrgID)
				return v, e
			})
			return
		}
		if parts[3] == "assessment-input" {
			h.input(w, r, id, principal.OrgID)
			return
		}
		if r.Method != http.MethodGet {
			httpjson.Problem(w, 405, "METHOD_NOT_ALLOWED")
			return
		}
		s, _, err := h.load(r.Context(), id, principal.OrgID)
		if err != nil {
			problem(w, err)
			return
		}
		w.Header().Set("ETag", fmt.Sprintf(`"%d"`, s.Version))
		httpjson.Write(w, 200, s)
	})
}

type Snapshot struct {
	Version     int64                    `json:"version"`
	BankVersion int64                    `json:"bank_event_version"`
	Status      string                   `json:"application_status"`
	Pending     *string                  `json:"pending_action"`
	Freshness   string                   `json:"freshness"`
	Assessment  *assessment.PublicResult `json:"assessment"`
}

func (h *Handler) load(ctx context.Context, id, org uuid.UUID) (Snapshot, assistanthelp.View, error) {
	var s Snapshot
	var raw []byte
	var next, reason string
	err := h.db.QueryRow(ctx, `SELECT version,bank_event_version,status,pending_action,next_action,COALESCE(public_reason,''),public_assessment FROM applications WHERE id=$1 AND org_id=$2`, id, org).Scan(&s.Version, &s.BankVersion, &s.Status, &s.Pending, &next, &reason, &raw)
	if err != nil {
		return s, assistanthelp.View{}, err
	}
	s.Freshness = "PENDING"
	if len(raw) > 0 {
		if err = json.Unmarshal(raw, &s.Assessment); err != nil {
			return s, assistanthelp.View{}, err
		}
		s.Freshness = "CURRENT"
		if s.Assessment != nil && !s.Assessment.ValidUntil.After(time.Now()) {
			s.Freshness = "STALE"
		}
	}
	if s.Pending != nil {
		s.Freshness = "PENDING"
	}
	v := assistanthelp.View{ApplicationID: id.String(), Revision: fmt.Sprintf("client:%d:%d", s.Version, s.BankVersion), Status: s.Status, NextStep: next, Reason: reason, Pending: s.Freshness != "CURRENT", Public: s.Assessment}
	rows, err := h.db.Query(ctx, `SELECT requirements FROM info_requests WHERE application_id=$1 AND is_active`, id)
	if err != nil {
		return s, v, err
	}
	defer rows.Close()
	for rows.Next() {
		var data []byte
		if err = rows.Scan(&data); err != nil {
			return s, v, err
		}
		var list []json.RawMessage
		if err = json.Unmarshal(data, &list); err != nil {
			return s, v, err
		}
		for _, b := range list {
			var q commonpb.Requirement
			if err = protojson.Unmarshal(b, &q); err != nil {
				return s, v, err
			}
			v.Requirements = append(v.Requirements, assistanthelp.Requirement{ID: q.Id, Label: q.Label, Reason: q.Reason})
		}
	}
	return s, v, rows.Err()
}

func (h *Handler) input(w http.ResponseWriter, r *http.Request, id, org uuid.UUID) {
	if r.Method != http.MethodGet && r.Method != http.MethodPut {
		httpjson.Problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	var version, amount int64
	var raw json.RawMessage
	err := h.db.QueryRow(r.Context(), `SELECT version,COALESCE(requested_amount_kopecks,0),assessment_input FROM applications WHERE id=$1 AND org_id=$2`, id, org).Scan(&version, &amount, &raw)
	if err != nil {
		problem(w, err)
		return
	}
	if r.Method == http.MethodPut {
		expected, e := httpjson.Version(r)
		if e != nil {
			httpjson.Problem(w, 400, "INVALID_IF_MATCH")
			return
		}
		var sub assessment.Submission
		if httpjson.Read(w, r, &sub) != nil {
			httpjson.Problem(w, 400, "INVALID_ASSESSMENT_INPUT")
			return
		}
		if e = assessment.ValidateSubmission(sub, amount); e != nil {
			httpjson.Problem(w, 422, e.Error())
			return
		}
		raw, e = json.Marshal(sub)
		if e != nil {
			httpjson.Problem(w, 500, "INTERNAL_ERROR")
			return
		}
		err = h.db.QueryRow(r.Context(), `UPDATE applications SET assessment_input=$4,public_assessment=NULL,version=version+1,updated_at=now() WHERE id=$1 AND org_id=$2 AND version=$3 AND status IN ('DRAFT','NEEDS_INFO') AND pending_action IS NULL RETURNING version`, id, org, expected, raw).Scan(&version)
		if errors.Is(err, pgx.ErrNoRows) {
			httpjson.Problem(w, 409, "APPLICATION_VERSION_OR_STATE_CONFLICT")
			return
		}
		if err != nil {
			problem(w, err)
			return
		}
	}
	w.Header().Set("ETag", fmt.Sprintf(`"%d"`, version))
	httpjson.Write(w, 200, map[string]any{"version": version, "assessment_input": raw})
}

func problem(w http.ResponseWriter, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		httpjson.Problem(w, 404, "APPLICATION_NOT_FOUND")
	} else {
		httpjson.Problem(w, 500, "INTERNAL_ERROR")
	}
}
