package summary

import "github.com/alfabank-hackathon-2026/smart-credit/services/bank-service/internal/model"


var purposeDescriptions = map[string]string{
	"WORKING_CAPITAL": "Пополнение оборотных средств",
	"EQUIPMENT":       "Покупка оборудования",
	"EXPANSION":       "Развитие бизнеса",
}

var statusDescriptions = map[model.CaseStatus]string{
	model.CaseStatusSubmitted:     "Заявка получена",
	model.CaseStatusValidation:    "Проверка полноты данных",
	model.CaseStatusNeedsInfo:     "Запрошены дополнительные данные",
	model.CaseStatusEnrichment:    "Проверка внешних источников",
	model.CaseStatusScoring:       "Расчёт скоринговой оценки",
	model.CaseStatusDecision:      "Применение банковских правил",
	model.CaseStatusManualReview:  "Ручное рассмотрение",
	model.CaseStatusOffer:         "Сформирована оферта",
	model.CaseStatusCounterOffer:  "Подготовлено встречное предложение",
	model.CaseStatusOfferAccepted: "Оферта принята клиентом",
	model.CaseStatusSigning:       "Подписание документов",
	model.CaseStatusDisbursed:     "Выдача завершена",
	model.CaseStatusRejected:      "Принято решение об отказе",
	model.CaseStatusCancelled:     "Заявка отменена",
	model.CaseStatusExpired:       "Срок действия истёк",
}


var ruleDescriptions = map[string]string{
	"UNKNOWN_PRODUCT":             "Продукт заявки не найден",
	"VALIDATION_INVALID_SNAPSHOT": "Данные компании или документов не удалось проверить",
	"MISSING_REQUIRED_DATA":       "Не хватает обязательных данных или документов",
	"PRODUCT_RULES_UNAVAILABLE":   "Правила продукта недоступны",
	"BUREAU_RESULT_UNAVAILABLE":   "Результат внешней проверки недоступен",
	"SCORE_RESULT_UNAVAILABLE":    "Результат скоринга недоступен",
	"COMPANY_LIQUIDATION":         "Компания ликвидируется или ликвидирована",
	"OVERDUE_90_DAYS":             "По данным кредитной истории просрочка составляет не менее 90 дней",
	"COMPANY_STATUS_REVIEW":       "Статус компании требует ручной проверки",
	"GRADE_E":                     "Скоринговая категория E",
	"GRADE_REVIEW":                "Скоринговая категория требует ручной проверки",
	"GRADE_UNKNOWN":               "Скоринговая категория не определена",
	"PRODUCT_RANGE_REVIEW":        "Сумма или срок вне диапазона продукта",
	"COMPANY_FINANCIALS_REVIEW":   "Выручка компании требует ручной проверки",
	"COMPANY_AGE_REVIEW":          "Возраст компании требует ручной проверки",
	"AMOUNT_REVENUE_REVIEW":       "Сумма превышает допустимую долю выручки",
	"OVERDUE_PRODUCT_REVIEW":      "Просрочка превышает порог продукта",
	"AUTO_OFFER":                  "По действующим правилам сформирована оферта",
	"REVIEW_REQUIRED":             "Необходим ручной разбор; конкретная причина этим кодом не указана",
	"LARGE_REQUEST":               "Запрошенная сумма требует дополнительной проверки",
}
