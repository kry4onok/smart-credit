package receiverepository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/requirements"
	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/events/publicstate"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/infra/transaction"
	"github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/usecase/case/receive"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const incompleteInfoMessage = "Для продолжения рассмотрения дополните запрошенные данные."

func decodeRequirements(data []byte) ([]*commonpb.Requirement, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	items := make([]*commonpb.Requirement, 0, len(raw))
	for _, part := range raw {
		item := new(commonpb.Requirement)
		if err := protojson.Unmarshal(part, item); err != nil {
			return nil, err
		}
		if item.GetId() == "" {
			return nil, errors.New("requirement without ID")
		}
		items = append(items, item)
	}
	return items, nil
}

func encodeRequirements(items []*commonpb.Requirement) ([]byte, error) {
	raw := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		if item == nil {
			return nil, errors.New("nil requirement")
		}
		data, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(item)
		if err != nil {
			return nil, err
		}
		raw = append(raw, data)
	}
	return json.Marshal(raw)
}

func mergeDocumentSnapshots(stored, incoming []byte) ([]byte, []*commonpb.DocumentMetadata, bool, error) {
	var previous, next []receive.DocumentSnapshot
	if err := json.Unmarshal(stored, &previous); err != nil {
		return nil, nil, false, err
	}
	if err := json.Unmarshal(incoming, &next); err != nil {
		return nil, nil, false, err
	}
	if previous == nil {
		previous = []receive.DocumentSnapshot{}
	}
	index := map[string]receive.DocumentSnapshot{}
	for _, d := range previous {
		if d.DocumentID == "" {
			return nil, nil, false, errors.New("stored document without ID")
		}
		if old, ok := index[d.DocumentID]; ok && !reflect.DeepEqual(old, d) {
			return nil, nil, true, nil
		}
		index[d.DocumentID] = d
	}
	for _, d := range next {
		if d.DocumentID == "" {
			return nil, nil, false, errors.New("incoming document without ID")
		}
		if old, ok := index[d.DocumentID]; ok {
			if !reflect.DeepEqual(old, d) {
				return nil, nil, true, nil
			}
			continue
		}
		previous = append(previous, d)
		index[d.DocumentID] = d
	}
	merged, err := json.Marshal(previous)
	if err != nil {
		return nil, nil, false, err
	}
	documents := make([]*commonpb.DocumentMetadata, 0, len(previous))
	for _, d := range previous {
		value, ok := commonpb.DocumentType_value["DOCUMENT_TYPE_"+d.DocumentType]
		if !ok || value == 0 {
			return nil, nil, false, fmt.Errorf("unknown stored document type %q", d.DocumentType)
		}
		documents = append(documents, &commonpb.DocumentMetadata{DocumentId: d.DocumentID, DocumentType: commonpb.DocumentType(value), FileName: d.FileName, Sha256: d.SHA256, UploadedAt: timestamppb.New(d.UploadedAt), IsMock: d.IsMock, RequirementId: d.RequirementID, CoveragePeriod: d.CoveragePeriod})
	}
	return merged, documents, false, nil
}

func unmetRequest(request []*commonpb.Requirement, plan *commonpb.RequirementsPlan, answeredIDs []string) []*commonpb.Requirement {
	byID := map[string]*commonpb.Requirement{}
	for _, item := range plan.GetItems() {
		if item != nil {
			byID[item.GetId()] = item
		}
	}
	answered := map[string]bool{}
	for _, id := range answeredIDs {
		answered[id] = true
	}
	var missing []*commonpb.Requirement
	for _, old := range request {
		if old == nil {
			continue
		}
		now := byID[old.GetId()]
		if now == nil {
			continue
		}
		if now.GetStatus() != commonpb.RequirementStatus_REQUIREMENT_STATUS_PROVIDED || now.GetKind() == commonpb.RequirementKind_REQUIREMENT_KIND_FIELD && !answered[old.GetId()] {
			missing = append(missing, proto.Clone(now).(*commonpb.Requirement))
		}
	}
	return missing
}

func (r *Repository) loadRequirementsProduct(ctx context.Context, q transaction.Queryer, id uuid.UUID, version int64) (*commonpb.Product, error) {
	var policy *string
	var purposes []string
	var min, max int64
	var minTerm, maxTerm int32
	err := q.QueryRow(ctx, selectRequirementsProductQuery, id, version).Scan(&policy, &purposes, &min, &max, &minTerm, &maxTerm)
	if err != nil {
		return nil, err
	}
	p := &commonpb.Product{ProductId: id.String(), Version: version, MinAmount: &commonpb.Money{AmountKopecks: min, Currency: "RUB"}, MaxAmount: &commonpb.Money{AmountKopecks: max, Currency: "RUB"}, MinTermMonths: minTerm, MaxTermMonths: maxTerm}
	if policy != nil {
		p.RequirementsPolicyVersion = *policy
	}
	for _, tag := range purposes {
		value, ok := commonpb.PurposeTag_value["PURPOSE_TAG_"+tag]
		if !ok {
			return nil, fmt.Errorf("unknown purpose %q", tag)
		}
		p.PurposeTags = append(p.PurposeTags, commonpb.PurposeTag(value))
	}
	return p, nil
}

func (r *Repository) applyV2InfoProvided(ctx context.Context, q transaction.Queryer, input receive.Input, storedINN string, storedDocuments []byte, referenceDate time.Time, productID uuid.UUID, productVersion int64, requestJSON []byte) (bool, error) {
	if input.InvalidDocumentMetadata {
		return r.manualV2Info(ctx, q, input, "INVALID_DOCUMENT_COVERAGE", "Метаданные документа не позволяют проверить период")
	}
	if input.ProfileSchemaVersion != 2 {
		return r.manualV2Info(ctx, q, input, "INVALID_PROFILE", "Профиль v2 не передан в ответе")
	}
	if input.CompanyINN != storedINN {
		return r.manualV2Info(ctx, q, input, "PROFILE_IDENTITY_IMMUTABLE", "Идентификатор компании изменён в ответе")
	}
	request, err := decodeRequirements(requestJSON)
	if err != nil {
		return r.manualV2Info(ctx, q, input, "INVALID_INFO_REQUEST", "Сохранённый запрос повреждён")
	}
	merged, documents, conflict, err := mergeDocumentSnapshots(storedDocuments, input.DocumentsJSON)
	if err != nil {
		return false, fmt.Errorf("merge v2 documents: %w", err)
	}
	if conflict {
		return r.manualV2Info(ctx, q, input, "DOCUMENT_METADATA_CONFLICT", "Метаданные документа с тем же ID изменены")
	}
	company := new(commonpb.CompanySnapshot)
	if err := protojson.Unmarshal(input.CompanyJSON, company); err != nil {
		return false, fmt.Errorf("decode v2 company: %w", err)
	}
	form := new(commonpb.ApplicationForm)
	if err := protojson.Unmarshal(input.FormJSON, form); err != nil {
		return false, fmt.Errorf("decode v2 form: %w", err)
	}
	product, err := r.loadRequirementsProduct(ctx, q, productID, productVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return r.manualV2Info(ctx, q, input, "UNKNOWN_REQUIREMENTS_POLICY", "Версия продукта недоступна")
	}
	if err != nil {
		return false, fmt.Errorf("load pinned product: %w", err)
	}
	plan, buildErr := requirements.Build(requirements.Input{Company: company, Product: product, Form: form, Documents: documents, ReferenceDate: referenceDate})
	status, reason, publicReason := "VALIDATION", "", ""
	var pending []*commonpb.Requirement
	if buildErr != nil || plan == nil || plan.GetRoute() == commonpb.RequirementsRoute_REQUIREMENTS_ROUTE_MANUAL_REVIEW {
		status, reason, publicReason = "MANUAL_REVIEW", "PROFILE_V2_REQUIRES_REVIEW", "Профиль требует проверки сотрудником"
	} else {
		issues := requirements.ValidateResponse(request, plan, input.AnsweredRequirementIDs)
		for _, item := range issues {
			if item.Code == requirements.UnknownRequirement {
				status, reason, publicReason = "MANUAL_REVIEW", "INVALID_INFO_ANSWER", "Ответ содержит недопустимый ID требования"
				break
			}
		}
		if status == "VALIDATION" && len(issues) > 0 {
			pending = unmetRequest(request, plan, input.AnsweredRequirementIDs)
			if len(pending) > 0 {
				status, reason, publicReason = "NEEDS_INFO", "REQUIREMENTS_NOT_MET", incompleteInfoMessage
			}
		}
	}
	result, err := q.Exec(ctx, fulfillInfoRequestQuery, input.RequestID, input.ApplicationID)
	if err != nil {
		return false, fmt.Errorf("fulfill v2 info request: %w", err)
	}
	if result.RowsAffected() != 1 {
		return false, ErrInfoProvidedConflict
	}
	var version int64
	err = q.QueryRow(ctx, updateCaseFromV2InfoQuery, input.ApplicationID, input.CompanyINN, input.CompanyName, string(input.CompanyJSON), string(input.FormJSON), string(merged), input.AmountKopecks, input.TermMonths, input.PurposeTag, input.AggregateVersion, status, nullableReason(publicReason), reason).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrInfoProvidedConflict
	}
	if err != nil {
		return false, fmt.Errorf("update v2 case: %w", err)
	}
	if _, err := q.Exec(ctx, insertInfoProvidedAuditQuery, uuid.New(), input.ApplicationID, version, input.EventID.String(), input.RequestID.String()); err != nil {
		return false, fmt.Errorf("audit v2 info: %w", err)
	}
	if status == "VALIDATION" {
		return true, nil
	}
	now := time.Now().UTC()
	eventID := uuid.New()
	var payload []byte
	if status == "NEEDS_INFO" {
		newRequestID := uuid.New()
		data, err := encodeRequirements(pending)
		if err != nil {
			return false, err
		}
		if _, err := q.Exec(ctx, insertV2InfoRequestQuery, newRequestID, input.ApplicationID, incompleteInfoMessage, string(data)); err != nil {
			return false, fmt.Errorf("create replacement info request: %w", err)
		}
		payload, err = publicstate.NeedsInfo(eventID, input.ApplicationID, newRequestID, version, now, incompleteInfoMessage, nil, nil)
		if err != nil {
			return false, err
		}
		payload, err = publicstate.WithRequirements(payload, plan, pending)
		if err != nil {
			return false, err
		}
	} else {
		payload, err = publicstate.ManualReview(eventID, input.ApplicationID, version, now, publicReason)
		if err != nil {
			return false, err
		}
	}
	if _, err := q.Exec(ctx, insertOfferResponseOutboxQuery, eventID, input.ApplicationID.String(), version, payload); err != nil {
		return false, fmt.Errorf("publish v2 info outcome: %w", err)
	}
	return true, nil
}

func nullableReason(reason string) any {
	if reason == "" {
		return nil
	}
	return reason
}

func (r *Repository) manualV2Info(ctx context.Context, q transaction.Queryer, input receive.Input, reason, publicReason string) (bool, error) {
	result, err := q.Exec(ctx, fulfillInfoRequestQuery, input.RequestID, input.ApplicationID)
	if err != nil {
		return false, err
	}
	if result.RowsAffected() != 1 {
		return false, ErrInfoProvidedConflict
	}
	var version int64
	err = q.QueryRow(ctx, markV2InfoManualQuery, input.ApplicationID, input.AggregateVersion, publicReason, reason).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrInfoProvidedConflict
	}
	if err != nil {
		return false, err
	}
	if _, err := q.Exec(ctx, insertInfoProvidedAuditQuery, uuid.New(), input.ApplicationID, version, input.EventID.String(), input.RequestID.String()); err != nil {
		return false, err
	}
	eventID := uuid.New()
	payload, err := publicstate.ManualReview(eventID, input.ApplicationID, version, time.Now().UTC(), publicReason)
	if err != nil {
		return false, err
	}
	if _, err := q.Exec(ctx, insertOfferResponseOutboxQuery, eventID, input.ApplicationID.String(), version, payload); err != nil {
		return false, err
	}
	return true, nil
}
