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
	Pending       bool
	Public        *assessment.PublicResult
	Risk          *assessment.RiskResult
	Requirements  []Requirement
}

type Requirement struct{ ID, Label, Reason string }

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
		next = "Откройте карточку заявки, чтобы увидеть текущий этап."
	}
	add("application", "Следующий шаг", next, pb.TargetKind_TARGET_KIND_PROFILE, pb.StatementTopic_STATEMENT_TOPIC_NEXT_STEP, pb.EvidenceKind_EVIDENCE_KIND_PROFILE)
	add("decision", "Статус заявки", fmt.Sprintf("Текущий статус заявки: %s. %s", v.Status, next), pb.TargetKind_TARGET_KIND_DECISION, pb.StatementTopic_STATEMENT_TOPIC_DECISION, pb.EvidenceKind_EVIDENCE_KIND_DECISION)
	for _, q := range v.Requirements[:min(30, len(v.Requirements))] {
		add("requirement:"+q.ID, q.Label, q.Label+". "+q.Reason, pb.TargetKind_TARGET_KIND_REQUIREMENT, pb.StatementTopic_STATEMENT_TOPIC_REQUIREMENT, pb.EvidenceKind_EVIDENCE_KIND_REQUIREMENT)
	}
	if v.Public != nil {
		r := v.Public
		c.CalculationId = &r.ID
		if r.Financial != nil {
			f := r.Financial
			add("cashflow", "Денежный поток", fmt.Sprintf("Расчёт использует денежный поток за %s–%s. Прогноз повторяет сезонность без роста; базовый и стрессовый сценарии являются прогнозом, а не подтверждёнными поступлениями.", f.PeriodFrom, f.PeriodTo), pb.TargetKind_TARGET_KIND_FACTOR, pb.StatementTopic_STATEMENT_TOPIC_AFFORDABILITY, pb.EvidenceKind_EVIDENCE_KIND_FINANCIAL)
		}
		for _, o := range r.Candidates {
			text := fmt.Sprintf("%s: сумма %s руб., срок %d мес., ставка %.2f%% годовых, регулярный платёж %s руб., всего выплат %s руб., непокрытая потребность %s руб. Последний платёж смотрите в графике. Это расчётный вариант; условия должен подтвердить сотрудник банка.", o.ProductName, money(o.Schedule.AmountKopecks), o.Schedule.TermMonths, float64(o.Schedule.RateBPS)/100, money(o.Schedule.PaymentKopecks), money(o.Schedule.TotalPaymentKopecks), money(o.UncoveredNeedKopecks))
			add(o.ID, "Расчётный вариант", text, pb.TargetKind_TARGET_KIND_OPTION, pb.StatementTopic_STATEMENT_TOPIC_OPTION, pb.EvidenceKind_EVIDENCE_KIND_CALCULATION)
		}
		for i, ch := range r.Changes[:min(10, len(r.Changes))] {
			add(fmt.Sprintf("change:%d", i), "Изменение оценки", fmt.Sprintf("%s: было %v, стало %v; единица %s.", ch.Path, ch.Previous, ch.Current, ch.Unit), pb.TargetKind_TARGET_KIND_CHANGE, pb.StatementTopic_STATEMENT_TOPIC_CHANGE, pb.EvidenceKind_EVIDENCE_KIND_CHANGE)
		}
	}
	if audience == pb.Audience_AUDIENCE_ANALYST && v.Risk != nil {
		for _, f := range v.Risk.Factors {
			add(f.ID, "Фактор риска", fmt.Sprintf("%s: значение %d (%s), вклад %d из %d баллов. Период %s–%s. Это экспертная оценка, не вероятность дефолта.", f.ID, f.Value, f.Unit, f.Points, f.MaxPoints, f.PeriodFrom, f.PeriodTo), pb.TargetKind_TARGET_KIND_FACTOR, pb.StatementTopic_STATEMENT_TOPIC_SCORE, pb.EvidenceKind_EVIDENCE_KIND_SCORE)
		}
	}
	limitation := "Демонстрационные сведения синтетические. Расчёт не гарантирует одобрение или выдачу кредита."
	if v.Pending {
		limitation += " Выполняется обновление заявки; дождитесь подтверждения нового состояния."
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

func money(n int64) string { return fmt.Sprintf("%d.%02d", n/100, n%100) }
