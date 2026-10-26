package assistanthelp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/alfabank-hackathon-2026/smart-credit/pkg/assessment"
)

var factorLabels = map[string]string{
	"payment_discipline":     "Платёжная дисциплина",
	"operating_margin":       "Операционная рентабельность",
	"customer_concentration": "Концентрация выручки по клиентам",
}

var sourceLabels = map[string]string{
	"REGISTRY":       "реестру компаний",
	"CREDIT_HISTORY": "кредитной истории",
	"BANKRUPTCY":     "реестру банкротств",
	"ARBITRATION":    "арбитражным делам",
	"ENFORCEMENT":    "исполнительным производствам",
}

var roleLabels = map[string]string{
	"BORROWER": "компании",
	"DIRECTOR": "руководителя",
	"OWNER":    "владельца",
}

var changeLabels = map[string]string{
	"risk.score":                           "Баллы скоринга",
	"status":                               "Статус оценки",
	"financial.operating_margin_bps":       "Операционная рентабельность",
	"financial.initial_cash_kopecks":       "Начальный остаток денежных средств",
	"financial.period_to":                  "Последний месяц расчёта",
	"candidates.count":                     "Число расчётных вариантов",
	"candidates.preferred.amount_kopecks":  "Сумма предпочтительного варианта",
	"candidates.preferred.term_months":     "Срок предпочтительного варианта",
	"candidates.preferred.payment_kopecks": "Платёж по предпочтительному варианту",
}

func factorText(f assessment.Factor) string {
	label := factorLabels[f.ID]
	if label == "" {
		label = "Показатель скоринга"
	}
	value := fmt.Sprint(f.Value)
	switch f.Unit {
	case "BASIS_POINTS":
		value = percent(int64(f.Value))
	case "DAYS":
		value += " дн."
	}
	return fmt.Sprintf("%s — %s; вклад в экспертную оценку %d из %d баллов за период %s–%s. Это не вероятность дефолта.", label, value, f.Points, f.MaxPoints, f.PeriodFrom, f.PeriodTo)
}

func sourceText(s assessment.Source) string {
	kind := sourceLabels[s.Kind]
	if kind == "" {
		kind = "внешнему источнику"
	}
	role := roleLabels[s.Role]
	if role == "" {
		role = "заявителя"
	}
	result := "результат требует проверки аналитиком."
	switch {
	case s.Status == "UNAVAILABLE":
		result = "источник недоступен; нарушение не подтверждено."
	case s.Status == "STALE":
		result = "данные устарели; нужна повторная проверка."
	case s.Status == "AMBIGUOUS" || s.Match == "AMBIGUOUS":
		result = "совпадение неоднозначно; нужна проверка."
	case s.Conclusion == "BLOCK":
		result = "выявлен сигнал, который требует проверки аналитиком."
	case s.Conclusion == "REVIEW":
		result = "результат требует проверки аналитиком."
	case s.Status == "NOT_FOUND":
		result = "запись не найдена; ограничений по этому источнику не выявлено."
	case s.Status == "SUCCESS" && s.Conclusion == "CLEAR":
		result = "данные получены; ограничений по этому источнику не выявлено."
	}
	return fmt.Sprintf("Проверка по %s для %s: %s", kind, role, result)
}

func changeText(ch assessment.Change) string {
	if ch.Unit == "VERSION" {
		if ch.Path == "policy_version" {
			return "Обновилась версия банковских правил для расчёта."
		}
		return "Обновилась версия скоринговой модели."
	}
	label := changeLabels[ch.Path]
	if label == "" {
		return "Один из показателей оценки изменился; подробности доступны в истории расчётов."
	}
	return fmt.Sprintf("%s: было %s, стало %s.", label, changeValue(ch.Previous, ch.Unit), changeValue(ch.Current, ch.Unit))
}

func changeValue(value any, unit string) string {
	switch unit {
	case "KOPECKS":
		if n, ok := integer(value); ok {
			return money(n) + " руб."
		}
	case "BASIS_POINTS":
		if n, ok := integer(value); ok {
			return percent(n)
		}
	case "MONTHS":
		return fmt.Sprint(value) + " мес."
	case "POINTS":
		return fmt.Sprint(value) + " баллов"
	case "STATUS":
		return statusLabel(fmt.Sprint(value))
	}
	return fmt.Sprint(value)
}

func integer(value any) (int64, bool) {
	switch n := value.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), n == float64(int64(n))
	case json.Number:
		parsed, err := n.Int64()
		return parsed, err == nil
	}
	return 0, false
}

func percent(bps int64) string {
	return strings.ReplaceAll(fmt.Sprintf("%.2f%%", float64(bps)/100), ".", ",")
}

func money(kopecks int64) string {
	sign := ""
	if kopecks < 0 {
		sign = "−"
		kopecks = -kopecks
	}
	roubles := strconv.FormatInt(kopecks/100, 10)
	groups := []string{}
	for len(roubles) > 3 {
		groups = append([]string{roubles[len(roubles)-3:]}, groups...)
		roubles = roubles[:len(roubles)-3]
	}
	groups = append([]string{roubles}, groups...)
	return fmt.Sprintf("%s%s,%02d", sign, strings.Join(groups, " "), kopecks%100)
}
