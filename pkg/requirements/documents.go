package requirements

import (
	"sort"
	"strings"

	commonpb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/common"
	"google.golang.org/protobuf/proto"
)

func MatchDocuments(req *commonpb.Requirement, documents []*commonpb.DocumentMetadata) []string {
	if req == nil || req.GetKind() != commonpb.RequirementKind_REQUIREMENT_KIND_DOCUMENT || req.GetPeriod() == nil {
		return nil
	}
	type match struct {
		id    string
		nanos int64
	}
	var matches []match
	for _, d := range documents {
		if d == nil || d.GetDocumentId() == "" || d.GetRequirementId() != req.GetId() || d.GetCoveragePeriod() == nil {
			continue
		}
		allowed := false
		for _, typ := range req.GetAcceptedDocumentTypes() {
			if d.GetDocumentType() == typ {
				allowed = true
				break
			}
		}
		if !allowed {
			continue
		}
		from, ok1 := day(d.GetCoveragePeriod().GetFrom())
		to, ok2 := day(d.GetCoveragePeriod().GetTo())
		needFrom, ok3 := day(req.GetPeriod().GetFrom())
		needTo, ok4 := day(req.GetPeriod().GetTo())
		if !ok1 || !ok2 || !ok3 || !ok4 || from.After(needFrom) || to.Before(needTo) {
			continue
		}
		var nanos int64
		if d.GetUploadedAt() != nil {
			nanos = d.GetUploadedAt().AsTime().UnixNano()
		}
		matches = append(matches, match{d.GetDocumentId(), nanos})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].nanos == matches[j].nanos {
			return matches[i].id > matches[j].id
		}
		return matches[i].nanos > matches[j].nanos
	})
	ids := make([]string, 0, len(matches))
	seen := map[string]bool{}
	for _, m := range matches {
		if !seen[m.id] {
			ids = append(ids, m.id)
			seen[m.id] = true
		}
	}
	return ids
}

func SelectRequest(plan *commonpb.RequirementsPlan, ids []string) ([]*commonpb.Requirement, error) {
	if plan == nil || len(ids) == 0 {
		return nil, &RuleError{Code: UnknownRequirement, Issues: []Issue{issue("requirement_ids", UnknownRequirement, "Выберите требования")}}
	}
	index := map[string]*commonpb.Requirement{}
	for _, r := range plan.GetItems() {
		if r != nil {
			index[r.GetId()] = r
		}
	}
	seen := map[string]bool{}
	selected := make([]*commonpb.Requirement, 0, len(ids))
	for _, id := range ids {
		r := index[id]
		if id == "" || seen[id] || r == nil || r.GetKind() == commonpb.RequirementKind_REQUIREMENT_KIND_DOCUMENT && r.GetStatus() != commonpb.RequirementStatus_REQUIREMENT_STATUS_MISSING {
			return nil, &RuleError{Code: UnknownRequirement, Issues: []Issue{issue("requirement_ids", UnknownRequirement, "Неизвестное, повторное или уже выполненное требование: "+id)}}
		}
		seen[id] = true
		selected = append(selected, proto.Clone(r).(*commonpb.Requirement))
	}
	return selected, nil
}

func ValidateResponse(request []*commonpb.Requirement, current *commonpb.RequirementsPlan, answeredFieldIDs []string) []Issue {
	if current == nil {
		return []Issue{issue("requirements_plan", RequirementsNotMet, "Не удалось пересчитать требования")}
	}
	currentByID := map[string]*commonpb.Requirement{}
	if current != nil {
		for _, r := range current.GetItems() {
			if r != nil {
				currentByID[r.GetId()] = r
			}
		}
	}
	requested := map[string]*commonpb.Requirement{}
	for _, r := range request {
		if r != nil {
			requested[r.GetId()] = r
		}
	}
	var issues []Issue
	answered := map[string]bool{}
	for _, id := range answeredFieldIDs {
		r := requested[id]
		if id == "" || answered[id] || r == nil || r.GetKind() != commonpb.RequirementKind_REQUIREMENT_KIND_FIELD {
			issues = append(issues, issue("answered_requirement_ids", UnknownRequirement, "Недопустимый ID поля: "+id))
		}
		answered[id] = true
	}
	for _, r := range request {
		if r == nil {
			continue
		}
		now := currentByID[r.GetId()]
		if now == nil {
			continue
		}
		if r.GetKind() == commonpb.RequirementKind_REQUIREMENT_KIND_FIELD {
			if !answered[r.GetId()] || now.GetStatus() != commonpb.RequirementStatus_REQUIREMENT_STATUS_PROVIDED {
				issues = append(issues, issue(strings.TrimSpace(r.GetFieldPath()), RequirementsNotMet, "На запрошенное поле нужен явный ответ"))
			}
		} else if now.GetStatus() != commonpb.RequirementStatus_REQUIREMENT_STATUS_PROVIDED {
			issues = append(issues, issue(r.GetId(), RequirementsNotMet, "Запрошенный документ не предоставлен"))
		}
	}
	return issues
}
