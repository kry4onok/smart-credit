package assessmentrepository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/assessment"
	"github.com/alfabank-hackathon-2026/smart-credit/pkg/creditcalc"
	"github.com/alfabank-hackathon-2026/smart-credit/pkg/requirements"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/events/publicstate"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/validation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
)

type Repository struct {
	db           transaction.Queryer
	transactions *transaction.Manager
}

func New(db transaction.Queryer, tx *transaction.Manager) *Repository { return &Repository{db, tx} }

func LoadInput(ctx context.Context, q transaction.Queryer, id uuid.UUID) (assessment.Input, error) {
	var in assessment.Input
	var sub, sources []byte
	var legal string
	err := q.QueryRow(ctx, `SELECT id::text,scoring_input_revision,assessment_generation,assessment_as_of,company_inn,COALESCE(company_snapshot->>'legal_form',''),product_id::text,requested_amount_kopecks,requested_term_months,purpose_tag,assessment_input,assessment_sources FROM cases WHERE id=$1`, id).Scan(&in.ApplicationID, &in.InputRevision, &in.Generation, &in.AsOf, &in.INN, &legal, &in.RequestedProductID, &in.RequestedAmount, &in.RequestedTerm, &in.Purpose, &sub, &sources)
	if err != nil {
		return in, err
	}
	if len(in.INN) == 12 {
		in.LegalForm = "IP"
	} else {
		in.LegalForm = "LLC"
	}
	_ = legal
	if err = json.Unmarshal(sub, &in.Submission); err != nil {
		return in, err
	}
	if len(sources) > 0 {
		if err = json.Unmarshal(sources, &in.Sources); err != nil {
			return in, err
		}
	}
	rows, err := q.Query(ctx, `SELECT p.id::text,p.version,p.name,p.purpose_tags,p.min_amount_kopecks,p.max_amount_kopecks,p.min_term_months,p.max_term_months,r.rate_grade_a_bps,r.rate_grade_b_bps FROM (SELECT DISTINCT ON(id) * FROM products ORDER BY id,version DESC) p JOIN product_rules r ON r.product_id=p.id AND r.product_version=p.version JOIN cases c ON c.id=$1 JOIN products requested ON requested.id=c.product_id AND requested.version=c.product_version WHERE p.is_active AND p.requirements_policy_version IS NOT DISTINCT FROM requested.requirements_policy_version AND p.required_document_types=requested.required_document_types ORDER BY p.id,p.version DESC`, id)
	if err != nil {
		return in, err
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var p assessment.Product
		if err = rows.Scan(&p.ID, &p.Version, &p.Name, &p.Purposes, &p.MinAmount, &p.MaxAmount, &p.MinTerm, &p.MaxTerm, &p.RateABPS, &p.RateBBPS); err != nil {
			return in, err
		}
		if !seen[p.ID] {
			in.Products = append(in.Products, p)
			seen[p.ID] = true
		}
	}
	return in, rows.Err()
}

func (r *Repository) RunOnce(ctx context.Context) (bool, error) {
	found := false
	err := r.transactions.WithinTransaction(ctx, func(ctx context.Context) error {
		q := transaction.QueryerFromContext(ctx, r.db)
		var id uuid.UUID
		var version int64
		err := q.QueryRow(ctx, `SELECT id,version FROM cases WHERE status='DECISION' ORDER BY created_at,id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &version)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		in, err := LoadInput(ctx, q, id)
		if err != nil {
			return err
		}
		result := assessment.Evaluate(in)
		missing, plan, err := unmetRequirements(ctx, q, id, in)
		if err != nil {
			return err
		}
		if result.Status != "REJECT" && len(missing) > 0 {
			result.Status = "NEEDS_INFO"
			result.Candidates = []assessment.Candidate{}
		}
		if result.Status == "NEEDS_INFO" && len(result.Issues) > 0 {
			missing = append(missing, &commonpb.Requirement{Id: "assessment_input", Code: "ASSESSMENT_INPUT", Label: "Структурированные финансовые данные", Reason: "Нужны помесячные потоки, резерв, концентрация и график действующих долгов", Kind: commonpb.RequirementKind_REQUIREMENT_KIND_FIELD, RequiredAt: commonpb.RequirementStage_REQUIREMENT_STAGE_ASSESSMENT, Status: commonpb.RequirementStatus_REQUIREMENT_STATUS_MISSING, FieldPath: "assessment_input"})
			plan.Items = append(plan.Items, missing[len(missing)-1])
		}
		var previous []byte
		err = q.QueryRow(ctx, `SELECT result FROM assessments WHERE case_id=$1 ORDER BY assessment_generation DESC LIMIT 1`, id).Scan(&previous)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if len(previous) > 0 {
			var old assessment.Result
			if err = json.Unmarshal(previous, &old); err != nil {
				return err
			}
			result.Changes = assessment.Diff(old, result)
		}
		inputJSON, err := json.Marshal(in)
		if err != nil {
			return err
		}
		resultJSON, err := json.Marshal(result)
		if err != nil {
			return err
		}
		publicJSON, err := json.Marshal(assessment.Public(result))
		if err != nil {
			return err
		}
		_, err = q.Exec(ctx, `INSERT INTO assessments(id,case_id,input_revision,assessment_generation,input,result,public_result,valid_until) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, result.ID, id, in.InputRevision, in.Generation, inputJSON, resultJSON, publicJSON, result.ValidUntil)
		if err != nil {
			return err
		}
		state := "MANUAL_REVIEW"
		recommendation := "MANUAL_REVIEW"
		reason := "Расчёт подготовлен. Условия подтверждает сотрудник банка."
		if result.Status == "REJECT" {
			state = "REJECTED"
			recommendation = "REJECT"
			reason = "Банк не может предложить кредит по действующим условиям продукта."
		}
		if result.Status == "NEEDS_INFO" {
			state = "NEEDS_INFO"
			recommendation = "REQUEST_INFO"
			reason = "Для продолжения дополните данные по указанным требованиям."
		}
		if result.Status == "NO_AFFORDABLE_OPTION" {
			reason = "В доступных условиях не найден вариант с посильными платежами."
		}
		if _, err = q.Exec(ctx, `UPDATE cases SET status=$2,recommendation=$3,public_reason=$4,triggered_rules=$5,version=version+1,audit_version=audit_version+1,updated_at=now(),lease_until=NULL,next_run_at=NULL WHERE id=$1`, id, state, recommendation, reason, []string{result.Status}); err != nil {
			return err
		}
		eventID := uuid.New()
		now := time.Now().UTC()
		var payload []byte
		switch state {
		case "NEEDS_INFO":
			requestID := uuid.New()
			raw := []json.RawMessage{}
			for _, item := range missing {
				data, e := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(item)
				if e != nil {
					return e
				}
				raw = append(raw, data)
			}
			data, e := json.Marshal(raw)
			if e != nil {
				return e
			}
			if _, err = q.Exec(ctx, `INSERT INTO info_requests(id,case_id,message,requirements) VALUES($1,$2,$3,$4)`, requestID, id, reason, data); err != nil {
				return err
			}
			payload, err = publicstate.NeedsInfo(eventID, id, requestID, version+1, now, reason, nil, nil)
			if err == nil {
				payload, err = publicstate.WithRequirements(payload, plan, missing)
			}
		case "REJECTED":
			payload, err = publicstate.Decision(eventID, id, version+1, now, "REJECT", reason, nil, nil)
		default:
			payload, err = publicstate.ManualReview(eventID, id, version+1, now, reason)
		}
		if err != nil {
			return err
		}
		if _, err = publicstate.Publish(ctx, q, eventID, id.String(), version+1, payload); err != nil {
			return err
		}
		_, err = q.Exec(ctx, `INSERT INTO audit_log(id,case_id,case_version,action,actor_type,reason,details) VALUES($1,$2,$3,'ASSESSMENT_COMPLETED','SYSTEM',$4,jsonb_build_object('assessment_id',$5::text,'assessment_generation',$6::bigint))`, uuid.New(), id, version+1, result.Status, result.ID, in.Generation)
		return err
	})
	return found, err
}

func unmetRequirements(ctx context.Context, q transaction.Queryer, id uuid.UUID, in assessment.Input) ([]*commonpb.Requirement, *commonpb.RequirementsPlan, error) {
	var snap validation.Snapshot
	var p validation.Product
	err := q.QueryRow(ctx, `SELECT c.company_snapshot,c.documents_snapshot,c.requirements_reference_date,c.requested_amount_kopecks,c.requested_term_months,c.purpose_tag,COALESCE(c.application_snapshot->>'purpose_description',''),COALESCE(p.requirements_policy_version,''),p.purpose_tags,p.required_document_types,p.min_amount_kopecks,p.max_amount_kopecks,p.min_term_months,p.max_term_months FROM cases c JOIN products p ON p.id=c.product_id AND p.version=c.product_version WHERE c.id=$1`, id).Scan(&snap.CompanyJSON, &snap.DocumentsJSON, &snap.ReferenceDate, &snap.AmountKopecks, &snap.TermMonths, &snap.PurposeTag, &snap.PurposeDescription, &p.PolicyVersion, &p.PurposeTags, &p.RequiredDocumentTypes, &p.MinAmountKopecks, &p.MaxAmountKopecks, &p.MinTermMonths, &p.MaxTermMonths)
	if err != nil {
		return nil, nil, err
	}
	p.Found = true
	if p.PolicyVersion == requirements.SourceFirstPolicyVersion {
		snap.ReferenceDate = &in.AsOf
	}
	checked := validation.Evaluate(snap, p)
	plan := checked.Plan
	if plan == nil {
		plan = &commonpb.RequirementsPlan{PolicyVersion: p.PolicyVersion, ReferenceDate: in.AsOf.Format(time.DateOnly)}
	}
	missing := []*commonpb.Requirement{}
	for _, item := range plan.Items {
		if item.Status == commonpb.RequirementStatus_REQUIREMENT_STATUS_PROVIDED || item.RequiredAt == commonpb.RequirementStage_REQUIREMENT_STAGE_ON_REQUEST {
			continue
		}
		for _, supplied := range in.Sources.SuppliedRequirements {
			if supplied.Code == item.Code && supplied.PeriodFrom <= item.GetPeriod().GetFrom() && supplied.PeriodTo >= item.GetPeriod().GetTo() && supplied.SourceRef != "" {
				item.Status = commonpb.RequirementStatus_REQUIREMENT_STATUS_PROVIDED
				item.Reason = "Сведения получены из источника: " + supplied.SourceRef
				break
			}
		}
		if item.Status == commonpb.RequirementStatus_REQUIREMENT_STATUS_MISSING {
			missing = append(missing, item)
		}
	}
	return missing, plan, nil
}

func CheckCurrentOffer(ctx context.Context, q transaction.Queryer, id, productID uuid.UUID, t creditcalc.Terms, now time.Time) (int64, error) {
	in, err := LoadInput(ctx, q, id)
	if err != nil {
		return 0, err
	}
	var raw []byte
	err = q.QueryRow(ctx, `SELECT result FROM assessments WHERE case_id=$1 AND assessment_generation=$2 AND input_revision=$3`, id, in.Generation, in.InputRevision).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, errors.New("ASSESSMENT_REQUIRED")
	}
	if err != nil {
		return 0, fmt.Errorf("load current assessment: %w", err)
	}
	var result assessment.Result
	if err = json.Unmarshal(raw, &result); err != nil {
		return 0, err
	}
	if !assessment.IsCurrent(result, in, now) {
		return 0, errors.New("ASSESSMENT_STALE")
	}
	allowed := false
	var productVersion int64
	for _, c := range result.Candidates {
		if c.ProductID == productID.String() && c.Schedule.Terms == t && c.ValidUntil.After(now) {
			allowed = true
			productVersion = c.ProductVersion
			break
		}
	}
	if !allowed {
		return 0, errors.New("CANDIDATE_NOT_FOUND")
	}
	_, err = assessment.CheckOffer(in, productID.String(), t)
	return productVersion, err
}

func (r *Repository) Recheck(ctx context.Context, id, actor, key uuid.UUID, version int64) (int64, error) {
	var generation int64
	err := r.transactions.WithinTransaction(ctx, func(ctx context.Context) error {
		q := transaction.QueryerFromContext(ctx, r.db)
		var oldVersion int64
		var oldActor uuid.UUID
		err := q.QueryRow(ctx, `SELECT actor_id,expected_version,generation FROM assessment_commands WHERE case_id=$1 AND idempotency_key=$2`, id, key).Scan(&oldActor, &oldVersion, &generation)
		if err == nil {
			if oldActor != actor || oldVersion != version {
				return errors.New("IDEMPOTENCY_CONFLICT")
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var current int64
		var state string
		var assignee *uuid.UUID
		if err = q.QueryRow(ctx, `SELECT version,status,assignee_id FROM cases WHERE id=$1 FOR UPDATE`, id).Scan(&current, &state, &assignee); err != nil {
			return err
		}
		// Re-read after acquiring the aggregate lock to handle concurrent retries.
		err = q.QueryRow(ctx, `SELECT actor_id,expected_version,generation FROM assessment_commands WHERE case_id=$1 AND idempotency_key=$2`, id, key).Scan(&oldActor, &oldVersion, &generation)
		if err == nil {
			if oldActor != actor || oldVersion != version {
				return errors.New("IDEMPOTENCY_CONFLICT")
			}
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if assignee == nil || *assignee != actor {
			return errors.New("FORBIDDEN")
		}
		if current != version || state != "MANUAL_REVIEW" && state != "NEEDS_INFO" {
			return errors.New("VERSION_CONFLICT")
		}
		if err = q.QueryRow(ctx, `UPDATE cases SET assessment_generation=assessment_generation+1,assessment_as_of=now(),assessment_sources=NULL,current_scoring_attempt_id=NULL,status='VALIDATION',recommendation=NULL,public_reason=NULL,triggered_rules='{}',lease_until=NULL,next_run_at=now(),version=version+1,audit_version=audit_version+1,updated_at=now() WHERE id=$1 RETURNING assessment_generation`, id).Scan(&generation); err != nil {
			return err
		}
		if _, err = q.Exec(ctx, `UPDATE info_requests SET status='CANCELLED' WHERE case_id=$1 AND status='OPEN'`, id); err != nil {
			return err
		}
		if _, err = q.Exec(ctx, `INSERT INTO assessment_commands(case_id,idempotency_key,actor_id,expected_version,generation) VALUES($1,$2,$3,$4,$5)`, id, key, actor, version, generation); err != nil {
			return err
		}
		eventID := uuid.New()
		payload := publicstate.UnderReview(eventID, id, id, version+1, time.Now().UTC())
		if _, err = publicstate.Publish(ctx, q, eventID, id.String(), version+1, payload); err != nil {
			return err
		}
		_, err = q.Exec(ctx, `INSERT INTO audit_log(id,case_id,case_version,action,actor_type,actor_id,details) VALUES($1,$2,$3,'ASSESSMENT_RECHECK','ANALYST',$4,jsonb_build_object('assessment_generation',$5::bigint))`, uuid.New(), id, version+1, actor, generation)
		return err
	})
	return generation, err
}
