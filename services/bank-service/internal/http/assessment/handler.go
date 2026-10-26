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
	auth "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/http/middleware/auth"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	repository "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/repository/assessment"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
)

type Handler struct {
	db        transaction.Queryer
	repo      *repository.Repository
	assistant string
}

func New(db transaction.Queryer, tx *transaction.Manager, target string) *Handler {
	return &Handler{db, repository.New(db, tx), target}
}

func (h *Handler) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) != 4 || parts[0] != "backoffice" || parts[1] != "applications" || (parts[3] != "assessment" && parts[3] != "assessments" && parts[3] != "recheck" && parts[3] != "help") {
			next.ServeHTTP(w, r)
			return
		}
		if _, ok := auth.UserID(r.Context()); !ok {
			httpjson.Problem(w, 401, "UNAUTHORIZED")
			return
		}
		id, err := uuid.Parse(parts[2])
		if err != nil {
			httpjson.Problem(w, 400, "INVALID_APPLICATION_ID")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		switch parts[3] {
		case "help":
			assistanthelp.ServeHTTP(w, r, h.assistant, pb.Audience_AUDIENCE_ANALYST, func(ctx context.Context) (assistanthelp.View, error) { _, v, e := h.load(ctx, id); return v, e })
		case "recheck":
			h.recheck(w, r, id)
		case "assessments":
			h.history(w, r, id)
		default:
			if r.Method != http.MethodGet {
				httpjson.Problem(w, 405, "METHOD_NOT_ALLOWED")
				return
			}
			data, _, e := h.load(r.Context(), id)
			if e != nil {
				h.problem(w, e)
				return
			}
			w.Header().Set("ETag", fmt.Sprintf(`"%d"`, data.Version))
			httpjson.Write(w, 200, data)
		}
	})
}

type Snapshot struct {
	Version       int64              `json:"version"`
	InputRevision int64              `json:"input_revision"`
	Generation    int64              `json:"assessment_generation"`
	Status        string             `json:"case_status"`
	Freshness     string             `json:"freshness"`
	Assessment    *assessment.Result `json:"assessment"`
	Baseline      json.RawMessage    `json:"baseline_v1"`
}

func (h *Handler) load(ctx context.Context, id uuid.UUID) (Snapshot, assistanthelp.View, error) {
	var s Snapshot
	var raw []byte
	var reason string
	err := h.db.QueryRow(ctx, `SELECT c.version,c.scoring_input_revision,c.assessment_generation,c.status,COALESCE(c.public_reason,''),a.result,CASE WHEN b.id IS NULL THEN NULL ELSE jsonb_build_object('attempt_id',b.id,'status',b.status,'input_revision',b.input_revision,'assessment_generation',b.assessment_generation,'as_of_date',b.as_of_date,'response',b.response,'error_code',b.error_code) END FROM cases c LEFT JOIN assessments a ON a.case_id=c.id AND a.input_revision=c.scoring_input_revision AND a.assessment_generation=c.assessment_generation LEFT JOIN scoring_attempts b ON b.id=c.current_scoring_attempt_id WHERE c.id=$1`, id).Scan(&s.Version, &s.InputRevision, &s.Generation, &s.Status, &reason, &raw, &s.Baseline)
	if err != nil {
		return s, assistanthelp.View{}, err
	}
	s.Freshness = "PENDING"
	if len(raw) > 0 {
		var result assessment.Result
		if err = json.Unmarshal(raw, &result); err != nil {
			return s, assistanthelp.View{}, err
		}
		s.Assessment = &result
		s.Freshness = "CURRENT"
		if !result.ValidUntil.After(time.Now()) {
			s.Freshness = "STALE"
		}
	}
	v := assistanthelp.View{ApplicationID: id.String(), Revision: fmt.Sprintf("bank:%d:%d:%d", s.Version, s.InputRevision, s.Generation), Status: s.Status, NextStep: reason, Pending: s.Freshness != "CURRENT"}
	if s.Assessment != nil {
		p := assessment.Public(*s.Assessment)
		p.Changes = s.Assessment.Changes
		v.Public = &p
		v.Risk = &s.Assessment.Risk
	}
	rows, err := h.db.Query(ctx, `SELECT requirements FROM info_requests WHERE case_id=$1 AND status='OPEN'`, id)
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

func (h *Handler) recheck(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	if r.Method != http.MethodPost {
		httpjson.Problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	actor, ok := auth.AnalystID(r.Context())
	if !ok {
		httpjson.Problem(w, 403, "ANALYST_REQUIRED")
		return
	}
	v, err := httpjson.Version(r)
	if err != nil {
		httpjson.Problem(w, 400, "INVALID_IF_MATCH")
		return
	}
	key, err := uuid.Parse(r.Header.Get("Idempotency-Key"))
	if err != nil || key == uuid.Nil {
		httpjson.Problem(w, 400, "INVALID_IDEMPOTENCY_KEY")
		return
	}
	generation, err := h.repo.Recheck(r.Context(), id, actor, key, v)
	if err != nil {
		switch err.Error() {
		case "FORBIDDEN":
			httpjson.Problem(w, 403, "CASE_NOT_ASSIGNED")
		case "VERSION_CONFLICT", "IDEMPOTENCY_CONFLICT":
			httpjson.Problem(w, 409, err.Error())
		default:
			h.problem(w, err)
		}
		return
	}
	httpjson.Write(w, 202, map[string]any{"assessment_generation": generation})
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	if r.Method != http.MethodGet {
		httpjson.Problem(w, 405, "METHOD_NOT_ALLOWED")
		return
	}
	var exists bool
	if err := h.db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM cases WHERE id=$1)`, id).Scan(&exists); err != nil {
		h.problem(w, err)
		return
	}
	if !exists {
		httpjson.Problem(w, 404, "APPLICATION_NOT_FOUND")
		return
	}
	rows, err := h.db.Query(r.Context(), `SELECT result FROM assessments WHERE case_id=$1 ORDER BY assessment_generation DESC LIMIT 20`, id)
	if err != nil {
		h.problem(w, err)
		return
	}
	defer rows.Close()
	items := []json.RawMessage{}
	for rows.Next() {
		var raw json.RawMessage
		if err = rows.Scan(&raw); err != nil {
			h.problem(w, err)
			return
		}
		items = append(items, raw)
	}
	if err = rows.Err(); err != nil {
		h.problem(w, err)
		return
	}
	httpjson.Write(w, 200, map[string]any{"items": items, "limit": 20})
}

func (h *Handler) problem(w http.ResponseWriter, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		httpjson.Problem(w, 404, "APPLICATION_NOT_FOUND")
	} else {
		httpjson.Problem(w, 500, "INTERNAL_ERROR")
	}
}
