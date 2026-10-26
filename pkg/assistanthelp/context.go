package assistanthelp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/assessment"
	pb "github.com/alfabank-hackathon-2026/smart-credit/proto/gen/go/assistant"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// View must be authorized and audience-filtered by the calling service.
type View struct {
	ApplicationID string
	Revision      string
	Status        string
	NextStep      string
	// Reason is the decision text already published to the client; never internal notes.
	Reason       string
	Pending      bool
	Public       *assessment.PublicResult
	Risk         *assessment.RiskResult
	Checks       *assessment.CheckResult
	Requirements []Requirement
}

type Requirement struct{ ID, Label, Reason string }

var statusLabels = map[string]string{
	"DRAFT": "черновик", "SUBMITTED": "отправлена", "UNDER_REVIEW": "на рассмотрении", "NEEDS_INFO": "нужны данные",
	"OFFER_AVAILABLE": "есть предложение банка", "FORMALIZING": "оформление", "DISBURSED": "деньги выданы",
	"REJECTED": "отказ", "CLOSED": "закрыта", "VALIDATION": "проверка полноты", "ENRICHMENT": "проверка источников",
	"SCORING": "скоринг", "DECISION": "решение по правилам", "MANUAL_REVIEW": "ручной разбор", "OFFER": "оферта",
	"COUNTER_OFFER": "контрпредложение", "OFFER_ACCEPTED": "оферта принята", "SIGNING": "подписание",
	"CANCELLED": "отменена", "EXPIRED": "срок предложения истёк",
}

// statusLabel keeps raw status codes out of text shown to people.
func statusLabel(status string) string {
	if label, ok := statusLabels[status]; ok {
		return "«" + label + "»"
	}
	return "статус уточняется"
}

// Context contains only approved statements. Forecasts are described as
// projections; the legacy Help evidence enum has no PROJECTED member.
func Context(v View, audience pb.Audience, now time.Time) *pb.HelpContext {
	c := &pb.HelpContext{SchemaVersion: 1, ApplicationId: v.ApplicationID, SourceRevision: v.Revision, Audience: audience, CatalogVersion: CatalogVersion, PolicyVersion: assessment.PolicyVersion, GeneratedAt: timestamppb.New(now), ValidUntil: timestamppb.New(now.Add(60 * time.Second)), Freshness: pb.Freshness_FRESHNESS_CURRENT}
	if v.Pending {
		c.Freshness = pb.Freshness_FRESHNESS_PENDING
	}
	add := func(id, label, value string, kind pb.TargetKind, topic pb.StatementTopic, evidence pb.EvidenceKind) {
		c.Targets = append(c.Targets, &pb.Target{Id: id, Kind: kind, Label: label})
		ref := "e:" + id
		c.Evidence = append(c.Evidence, &pb.Evidence{Id: ref, Kind: evidence, SourceRef: "snapshot:" + v.Revision, Label: label, ObservedAt: timestamppb.New(now), State: pb.EvidenceState_EVIDENCE_STATE_PROVIDED, IsMock: true})
		c.Statements = append(c.Statements, &pb.Statement{Id: "s:" + id, TargetId: id, Topic: topic, Priority: 50, EvidenceIds: []string{ref}, Variants: []*pb.StatementVariant{{Id: "concise", Text: value}, {Id: "detailed", Text: value}}})
	}
	next := v.NextStep
	if next == "" {
		switch {
		case audience == pb.Audience_AUDIENCE_ANALYST && v.Status == "MANUAL_REVIEW":
			next = "Проверьте оценку и доступные варианты, затем зафиксируйте решение по делу."
		case audience == pb.Audience_AUDIENCE_ANALYST:
			next = "Проверьте текущий статус и данные дела перед дальнейшим действием."
		case v.Status == "DISBURSED":
			next = "Выдача завершена; подробности доступны в истории заявки."
		case v.Status == "REJECTED":
			next = "Посмотрите опубликованное объяснение решения в карточке заявки."
		default:
			next = "Следите за обновлением статуса заявки."
		}
	}
	add("application", "Следующий шаг", next, pb.TargetKind_TARGET_KIND_PROFILE, pb.StatementTopic_STATEMENT_TOPIC_NEXT_STEP, pb.EvidenceKind_EVIDENCE_KIND_PROFILE)
	decision := fmt.Sprintf("Текущий статус заявки: %s.", statusLabel(v.Status))
	if v.Reason != "" {
		decision += " " + v.Reason
	}
	add("decision", "Статус заявки", decision, pb.TargetKind_TARGET_KIND_DECISION, pb.StatementTopic_STATEMENT_TOPIC_DECISION, pb.EvidenceKind_EVIDENCE_KIND_DECISION)
	for _, q := range v.Requirements[:min(30, len(v.Requirements))] {
		add("requirement:"+q.ID, q.Label, q.Label+". "+q.Reason, pb.TargetKind_TARGET_KIND_REQUIREMENT, pb.StatementTopic_STATEMENT_TOPIC_REQUIREMENT, pb.EvidenceKind_EVIDENCE_KIND_REQUIREMENT)
	}
	if v.Public != nil {
		r := v.Public
		if !r.ValidUntil.IsZero() && r.ValidUntil.Before(c.ValidUntil.AsTime()) {
			c.ValidUntil = timestamppb.New(r.ValidUntil)
		}
		if !c.ValidUntil.AsTime().After(now) {
			c.Freshness = pb.Freshness_FRESHNESS_STALE
		}
		c.CalculationId = &r.ID
		if r.Financial != nil {
			f := r.Financial
			add("cashflow", "Денежный поток", fmt.Sprintf("Расчёт опирается на денежный поток за период %s–%s. Прогноз повторяет сезонность без роста и не считается подтверждёнными поступлениями.", f.PeriodFrom, f.PeriodTo), pb.TargetKind_TARGET_KIND_FACTOR, pb.StatementTopic_STATEMENT_TOPIC_AFFORDABILITY, pb.EvidenceKind_EVIDENCE_KIND_FINANCIAL)
		}
		for _, o := range r.Candidates {
			text := fmt.Sprintf("Вариант «%s»: %s руб. на %d мес. под %s годовых. Платёж — %s руб. в месяц, всего — %s руб.; непокрытая часть потребности — %s руб. Это предварительный расчёт, условия подтверждает банк.", o.ProductName, money(o.Schedule.AmountKopecks), o.Schedule.TermMonths, percent(int64(o.Schedule.RateBPS)), money(o.Schedule.PaymentKopecks), money(o.Schedule.TotalPaymentKopecks), money(o.UncoveredNeedKopecks))
			add(o.ID, "Расчётный вариант", text, pb.TargetKind_TARGET_KIND_OPTION, pb.StatementTopic_STATEMENT_TOPIC_OPTION, pb.EvidenceKind_EVIDENCE_KIND_CALCULATION)
		}
		for _, o := range r.Candidates {
			if o.UncoveredNeedKopecks > 0 {
				text := fmt.Sprintf("Расчётный вариант покрывает не всю запрошенную сумму: непокрытая часть потребности составляет %s руб. Банк учитывает способность денежного потока выдерживать регулярный платёж; окончательные условия подтверждает сотрудник банка.", money(o.UncoveredNeedKopecks))
				add("amount_gap", "Почему сумма меньше", text, pb.TargetKind_TARGET_KIND_FACTOR, pb.StatementTopic_STATEMENT_TOPIC_AFFORDABILITY, pb.EvidenceKind_EVIDENCE_KIND_CALCULATION)
				break
			}
		}
		for i, ch := range r.Changes[:min(10, len(r.Changes))] {
			add(fmt.Sprintf("change:%d", i), "Изменение оценки", changeText(ch), pb.TargetKind_TARGET_KIND_CHANGE, pb.StatementTopic_STATEMENT_TOPIC_CHANGE, pb.EvidenceKind_EVIDENCE_KIND_CHANGE)
		}
	}
	if audience == pb.Audience_AUDIENCE_ANALYST && v.Risk != nil {
		for _, f := range v.Risk.Factors {
			add(f.ID, "Фактор риска", factorText(f), pb.TargetKind_TARGET_KIND_FACTOR, pb.StatementTopic_STATEMENT_TOPIC_SCORE, pb.EvidenceKind_EVIDENCE_KIND_SCORE)
		}
	}
	if audience == pb.Audience_AUDIENCE_ANALYST && v.Checks != nil {
		for i, source := range v.Checks.Sources[:min(20, len(v.Checks.Sources))] {
			id := fmt.Sprintf("source:%d", i)
			add(id, "Проверка источника", sourceText(source), pb.TargetKind_TARGET_KIND_FACTOR, pb.StatementTopic_STATEMENT_TOPIC_POLICY, pb.EvidenceKind_EVIDENCE_KIND_CHECK)
			e := c.Evidence[len(c.Evidence)-1]
			e.IsMock = source.Provenance.IsMock
			if source.Provenance.SourceRef != "" {
				e.SourceRef = source.Provenance.SourceRef
			}
			if !source.Provenance.ObservedAt.IsZero() {
				e.ObservedAt = timestamppb.New(source.Provenance.ObservedAt)
			}
			if source.Status == "UNAVAILABLE" {
				e.State = pb.EvidenceState_EVIDENCE_STATE_UNAVAILABLE
			}
			if source.Status == "AMBIGUOUS" || source.Match == "AMBIGUOUS" {
				e.State = pb.EvidenceState_EVIDENCE_STATE_AMBIGUOUS
			}
			if source.Status == "STALE" {
				e.State = pb.EvidenceState_EVIDENCE_STATE_UNAVAILABLE
			}
		}
	}
	// Navigation only: no action here accepts, signs, submits or disburses.
	for _, target := range c.Targets {
		kind := pb.ActionKind_ACTION_KIND_UNSPECIFIED
		switch target.Kind {
		case pb.TargetKind_TARGET_KIND_REQUIREMENT:
			kind = pb.ActionKind_ACTION_KIND_OPEN_REQUIREMENT
		case pb.TargetKind_TARGET_KIND_FACTOR:
			kind = pb.ActionKind_ACTION_KIND_OPEN_FACTOR
		case pb.TargetKind_TARGET_KIND_OPTION:
			kind = pb.ActionKind_ACTION_KIND_OPEN_OPTIONS
		case pb.TargetKind_TARGET_KIND_CHANGE:
			kind = pb.ActionKind_ACTION_KIND_OPEN_TIMELINE
		case pb.TargetKind_TARGET_KIND_PROFILE:
			kind = pb.ActionKind_ACTION_KIND_OPEN_PROFILE
		case pb.TargetKind_TARGET_KIND_DECISION:
			if v.Status == "OFFER_AVAILABLE" || v.Status == "OFFERED" || v.Status == "FORMALIZING" {
				kind = pb.ActionKind_ACTION_KIND_OPEN_OFFER
			} else {
				kind = pb.ActionKind_ACTION_KIND_OPEN_TIMELINE
			}
		}
		if kind != pb.ActionKind_ACTION_KIND_UNSPECIFIED {
			c.Actions = append(c.Actions, &pb.Action{Id: "open:" + target.Id, Kind: kind, TargetId: target.Id, Label: target.Label})
		}
	}
	limitation := "Данные и расчёты демонстрационные; условия и решение банка подтверждаются отдельно."
	if c.Freshness == pb.Freshness_FRESHNESS_STALE {
		limitation += " Срок актуальности оценки истёк; требуется новая проверка."
	} else if v.Pending {
		limitation += " Данные обновляются; дождитесь актуального статуса."
	}
	c.Statements = append(c.Statements, &pb.Statement{Id: "limitations", Topic: pb.StatementTopic_STATEMENT_TOPIC_DATA_QUALITY, Required: true, Priority: 100, Variants: []*pb.StatementVariant{{Id: "concise", Text: limitation}, {Id: "detailed", Text: limitation}}})
	// Hash the complete source view, never timestamps generated for this request.
	raw, _ := json.Marshal(struct {
		View     View
		Audience pb.Audience
		Catalog  string
	}{v, audience, CatalogVersion})
	sum := sha256.Sum256(raw)
	c.ContextId = hex.EncodeToString(sum[:])
	return c
}
