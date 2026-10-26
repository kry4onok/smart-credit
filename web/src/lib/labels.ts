import type { BankStatus, ClientStatus, DocumentType, InfoField, PurposeTag, ScoreReason } from "../api/types";

export const purposeLabel: Record<PurposeTag, string> = {
  WORKING_CAPITAL: "Пополнить оборотные средства",
  EQUIPMENT: "Купить оборудование",
  EXPANSION: "Развить бизнес",
};

export const purposeHint: Record<PurposeTag, string> = {
  WORKING_CAPITAL: "Закупка товара, зарплаты, аренда, кассовые разрывы",
  EQUIPMENT: "Станки, транспорт, техника для работы",
  EXPANSION: "Новая точка, направление или крупный проект",
};

export const documentLabel: Record<DocumentType, string> = {
  BANK_STATEMENT: "Выписка по расчётному счёту",
  BUSINESS_PLAN: "Бизнес-план",
  ACCOUNTING_REPORT: "Бухгалтерская отчётность",
  MANAGEMENT_REPORT: "Управленческий отчёт",
  TAX_DECLARATION_USN: "Декларация УСН",
  TAX_DECLARATION_3NDFL: "Декларация 3-НДФЛ",
  INCOME_LEDGER: "Книга учёта доходов и расходов",
  PATENT: "Патент",
  RECEIVABLES_PAYABLES: "Дебиторская и кредиторская задолженность",
};

export const documentHint: Record<DocumentType, string> = {
  BANK_STATEMENT: "За последние 12 месяцев, из любого банка",
  BUSINESS_PLAN: "Цель, план расходов и ожидаемый эффект",
  ACCOUNTING_REPORT: "Баланс и отчёт о финансовых результатах за указанный период",
  MANAGEMENT_REPORT: "Доходы, расходы и прибыль за указанный период",
  TAX_DECLARATION_USN: "За год, указанный в запросе банка",
  TAX_DECLARATION_3NDFL: "Доходы предпринимателя за указанный год",
  INCOME_LEDGER: "Книга учёта, соответствующая налоговому режиму",
  PATENT: "Выданный патент или комплект действующих патентов",
  RECEIVABLES_PAYABLES: "Расшифровка на указанную дату",
};

export const infoFieldLabel: Record<InfoField, string> = {
  COMPANY_REVENUE_12M: "Выручка за 12 месяцев",
  COMPANY_PROFIT_12M: "Прибыль за 12 месяцев",
  COMPANY_OBLIGATIONS: "Текущие кредиты и займы",
  APPLICATION_AMOUNT: "Сумма кредита",
  APPLICATION_TERM_MONTHS: "Срок кредита",
  APPLICATION_PURPOSE: "Цель кредита",
};

export const clientStatusLabel: Record<ClientStatus, string> = {
  DRAFT: "Черновик",
  SUBMITTED: "Отправлена",
  UNDER_REVIEW: "На рассмотрении",
  NEEDS_INFO: "Нужны данные",
  OFFER_AVAILABLE: "Есть предложение",
  FORMALIZING: "Оформление",
  DISBURSED: "Деньги выданы",
  REJECTED: "Отказ",
  CLOSED: "Закрыта",
};

export type Tone = "neutral" | "progress" | "attention" | "success" | "danger" | "muted";

export const clientStatusTone: Record<ClientStatus, Tone> = {
  DRAFT: "neutral",
  SUBMITTED: "progress",
  UNDER_REVIEW: "progress",
  NEEDS_INFO: "attention",
  OFFER_AVAILABLE: "success",
  FORMALIZING: "success",
  DISBURSED: "success",
  REJECTED: "danger",
  CLOSED: "muted",
};

export const bankStatusLabel: Record<BankStatus, string> = {
  SUBMITTED: "Получена",
  VALIDATION: "Проверка полноты",
  NEEDS_INFO: "Запрос данных",
  ENRICHMENT: "Внешние источники",
  SCORING: "Скоринг",
  DECISION: "Решение по правилам",
  MANUAL_REVIEW: "Ручной разбор",
  OFFER: "Оферта",
  COUNTER_OFFER: "Контрпредложение",
  OFFER_ACCEPTED: "Оферта принята",
  SIGNING: "Подписание",
  DISBURSED: "Выдан",
  REJECTED: "Отказ",
  CANCELLED: "Отменена",
  EXPIRED: "Истекла",
};

export const bankStatusTone: Record<BankStatus, Tone> = {
  SUBMITTED: "progress",
  VALIDATION: "progress",
  NEEDS_INFO: "attention",
  ENRICHMENT: "progress",
  SCORING: "progress",
  DECISION: "progress",
  MANUAL_REVIEW: "attention",
  OFFER: "success",
  COUNTER_OFFER: "success",
  OFFER_ACCEPTED: "success",
  SIGNING: "success",
  DISBURSED: "success",
  REJECTED: "danger",
  CANCELLED: "muted",
  EXPIRED: "muted",
};

export const reasonLabel: Record<ScoreReason["code"], string> = {
  SHORT_HISTORY: "Короткая история бизнеса",
  LOW_PROFIT_MARGIN: "Низкая рентабельность",
  HIGH_OBLIGATIONS: "Высокие заявленные обязательства",
  HIGH_BUREAU_DEBT: "Высокий долг по данным БКИ",
  RECENT_OVERDUE: "Недавние просрочки",
  LARGE_REQUEST: "Крупный запрос относительно выручки",
  STABLE_BUSINESS: "Стабильный бизнес",
  NO_OVERDUE: "Нет просрочек",
  HEALTHY_PROFIT_MARGIN: "Здоровая рентабельность",
  MANAGEABLE_BUREAU_DEBT: "Умеренный долг по данным БКИ",
  GOOD_PAYMENT_DISCIPLINE: "Хорошая платёжная дисциплина",
};

export const factorLabel: Record<string, string> = {
  business_age: "Возраст бизнеса",
  profit_margin: "Рентабельность",
  bureau_debt_load: "Долговая нагрузка (БКИ)",
  payment_discipline: "Платёжная дисциплина",
};

export const companyStatusLabel: Record<string, string> = {
  UNKNOWN: "Нет данных",
  ACTIVE: "Действующая",
  LIQUIDATING: "В процессе ликвидации",
  LIQUIDATED: "Ликвидирована",
};

export const recommendationLabel: Record<string, string> = {
  APPROVE: "Одобрить",
  COUNTER: "Контрпредложение",
  REQUEST_INFO: "Запросить данные",
  MANUAL_REVIEW: "Нужен ручной разбор",
  REJECT: "Отказать",
};

export const actorLabel: Record<string, string> = {
  SYSTEM: "Система",
  ANALYST: "Аналитик",
  CLIENT: "Клиент",
};

// Decision-engine rule codes from bank-service; unknown codes are shown as is.
export const ruleLabel: Record<string, string> = {
  AUTO_OFFER: "Автоматическая оферта",
  AMOUNT_REVENUE_REVIEW: "Сумма велика относительно выручки",
  COMPANY_AGE_REVIEW: "Компания моложе порога продукта",
  COMPANY_FINANCIALS_REVIEW: "Нужна проверка финансовых показателей",
  COMPANY_LIQUIDATION: "Компания ликвидируется",
  COMPANY_STATUS_REVIEW: "Статус компании требует проверки",
  GRADE_E: "Категория риска E",
  GRADE_REVIEW: "Категория риска требует ручного разбора",
  GRADE_UNKNOWN: "Категория риска не определена",
  OVERDUE_PRODUCT_REVIEW: "Просрочки выше порога продукта",
  PRODUCT_RANGE_REVIEW: "Сумма или срок вне диапазона продукта",
  PRODUCT_RULES_UNAVAILABLE: "Правила продукта недоступны",
  MISSING_REQUIRED_DATA: "Не хватает обязательных данных",
  BUREAU_TIMEOUT: "БКИ не ответило вовремя",
  BUREAU_UNAVAILABLE: "БКИ недоступно",
  BUREAU_RESULT_UNAVAILABLE: "Нет результата проверки БКИ",
  SCORE_RESULT_UNAVAILABLE: "Нет результата скоринга",
  SCORING_UNAVAILABLE: "Скоринг недоступен",
  SCORING_DEADLINE: "Скоринг не ответил вовремя",
  SCORING_INSUFFICIENT_DATA: "Скорингу не хватило данных",
  SCORING_UNSUPPORTED_INPUT: "Данные вне области модели",
  UNKNOWN_PRODUCT: "Неизвестный продукт",
  REVIEW_REQUIRED: "Нужен ручной разбор",
  READY_FOR_CONFIRMATION: "Вариант ждёт подтверждения менеджером",
  NO_AFFORDABLE_OPTION: "Нет доступного варианта по денежному потоку",
  LARGE_REQUEST: "Крупный запрос",
};

// Audit actions from bank-service; transitions are labelled by their statuses instead.
export const auditActionLabel: Record<string, string> = {
  APPLICATION_SUBMITTED: "Заявка получена",
  INFO_PROVIDED: "Клиент ответил на запрос",
  VALIDATION_COMPLETED: "Проверка полноты завершена",
  ENRICHMENT_COMPLETED: "Внешние источники проверены",
  SCORING_COMPLETED: "Скоринг выполнен",
  DECISION_COMPLETED: "Решение по правилам",
  DECISION: "Решение аналитика",
  TAKE: "Взято в работу",
  CASE_TAKEN: "Взято в работу",
  CASE_RECEIVED: "Заявка получена",
  ANALYST_DECISION: "Решение аналитика",
  BUREAU_CALL: "Запрос в БКИ",
  ASSESSMENT_COMPLETED: "Оценка денежного потока выполнена",
  ASSESSMENT_RECHECK: "Запущен пересчёт оценки",
  CLIENT_EVENT_SKIPPED: "Действие клиента устарело и не применено",
  REQUEST_INFO: "Запрошены данные",
  INFO_REQUESTED: "Запрошены данные",
  OFFER_ACCEPTED: "Клиент принял оферту",
  OFFER_DECLINED: "Клиент отказался от оферты",
  OFFER_EXPIRED: "Срок оферты истёк",
  CONTRACT_SIGNED: "Договор подписан",
  DEMO_DISBURSED: "Деньги выданы (демо)",
  APPLICATION_CANCELLED: "Клиент отозвал заявку",
};

// --- Profile v2: legal form, tax regimes, requirements ---

export const legalFormLabel: Record<"IP" | "LLC", string> = {
  IP: "Индивидуальный предприниматель",
  LLC: "ООО",
};

export const taxRegimeLabel: Record<string, string> = {
  OSNO: "ОСНО",
  USN_INCOME: "УСН «Доходы»",
  USN_INCOME_EXPENSE: "УСН «Доходы минус расходы»",
  PSN: "Патент (ПСН)",
  AUSN_INCOME: "АУСН «Доходы»",
  AUSN_INCOME_EXPENSE: "АУСН «Доходы минус расходы»",
  ESHN: "ЕСХН",
  NPD: "Самозанятость (НПД)",
  OTHER: "Другой режим",
  UNKNOWN: "Не знаю",
};

// Regimes available to each legal form: a patent and NPD exist only for sole proprietors.
export const taxRegimesFor: Record<"IP" | "LLC", string[]> = {
  IP: ["USN_INCOME", "USN_INCOME_EXPENSE", "PSN", "OSNO", "AUSN_INCOME", "AUSN_INCOME_EXPENSE", "ESHN", "NPD", "UNKNOWN"],
  LLC: ["USN_INCOME", "USN_INCOME_EXPENSE", "OSNO", "AUSN_INCOME", "AUSN_INCOME_EXPENSE", "ESHN", "UNKNOWN"],
};

export const financialBasisLabel: Record<string, string> = {
  MANAGEMENT: "Управленческий учёт",
  ACCOUNTING: "Бухгалтерская отчётность",
  TAX: "Налоговая отчётность",
  UNKNOWN: "Не знаю",
};

export const vatStatusLabel: Record<string, string> = {
  UNKNOWN: "Не знаю",
  EXEMPT: "Без НДС",
  PAYER: "Плательщик НДС",
};

export const requiredAtLabel: Record<string, string> = {
  SUBMIT: "Нужно до отправки",
  ASSESSMENT: "Понадобится для оценки",
  ON_REQUEST: "Банк может запросить",
};

// Reasons why the requirements engine sends a draft to a manual route.
export const planReasonLabel: Record<string, string> = {
  DEBT_DATE_STALE: "Сведения о долгах указаны на старую дату",
  FINANCIAL_BASIS_UNSUPPORTED: "Источник финансовых данных требует проверки",
  FINANCIAL_PERIOD_UNSUPPORTED: "Финансовый период не подходит для автоматической оценки",
  TAX_HISTORY_INCOMPLETE: "История налоговых режимов неполная",
  TAX_REGIME_REQUIRES_REVIEW: "Налоговый режим требует проверки специалистом",
  UNSUPPORTED_TAX_COMBINATION: "Такое сочетание налоговых режимов проверяется вручную",
  LEGAL_FORM_INN_MISMATCH: "Форма бизнеса не совпадает с ИНН",
  INVALID_TAX_PERIOD: "Периоды налоговых режимов заполнены с ошибкой",
};

// --- Assessment v2 ---

export const assessmentStatusLabel: Record<string, string> = {
  READY_FOR_CONFIRMATION: "Банк рассчитал доступные условия",
  NO_AFFORDABLE_OPTION: "Подходящих условий не нашлось",
  NEEDS_INFO: "Для оценки нужны данные",
  MANUAL_REVIEW: "Нужна проверка специалиста",
  REJECT: "Кредит недоступен по политике банка",
};

export const priorityLabel: Record<string, { title: string; hint: string }> = {
  KEEP_AMOUNT: { title: "Получить всю сумму", hint: "Срок и платёж подберём под запрошенную сумму" },
  LOW_PAYMENT: { title: "Меньший платёж", hint: "Важно, чтобы ежемесячная нагрузка была ниже" },
  LOW_TOTAL_COST: { title: "Меньше переплата", hint: "Короче срок — меньше процентов в сумме" },
};

export const v2FactorLabel: Record<string, string> = {
  payment_discipline: "Платёжная дисциплина",
  operating_margin: "Запас денежного потока",
  customer_concentration: "Зависимость от крупнейшего покупателя",
};

export const sourceKindLabel: Record<string, string> = {
  REGISTRY: "ЕГРЮЛ/ЕГРИП",
  CREDIT_HISTORY: "Бюро кредитных историй",
  BANKRUPTCY: "Банкротства (ЕФРСБ)",
  ARBITRATION: "Арбитражные дела",
  ENFORCEMENT: "Исполнительные производства (ФССП)",
};

export const subjectRoleLabel: Record<string, string> = {
  BORROWER: "Заёмщик",
  DIRECTOR: "Руководитель",
  OWNER: "Владелец",
};

export const sourceStatusLabel: Record<string, string> = {
  SUCCESS: "Ответ получен",
  NOT_FOUND: "Записей нет",
  UNAVAILABLE: "Источник недоступен",
  STALE: "Данные устарели",
  AMBIGUOUS: "Неоднозначное совпадение",
};

export const conclusionLabel: Record<string, string> = {
  CLEAR: "Чисто",
  REVIEW: "Проверить",
  BLOCK: "Стоп-фактор",
};

export const checkCodeLabel: Record<string, string> = {
  COMPANY_LIQUIDATION: "Компания ликвидируется",
  HISTORICAL_DPD_90_12M: "Просрочка 90+ дней за год",
  CURRENT_OVERDUE: "Есть текущая просрочка",
  ACTIVITY_UNDER_12_MONTHS: "Бизнесу меньше 12 месяцев",
  RELATED_OR_SOURCE_REVIEW: "Связанные лица или источники требуют проверки",
  SOURCE_UNAVAILABLE: "Источник недоступен",
  SOURCE_STALE: "Данные источника устарели",
  SOURCE_CONFLICT: "Источники противоречат друг другу",
  SUBJECT_MATCH_AMBIGUOUS: "Неоднозначное совпадение по лицу",
  CREDIT_HISTORY_INCOMPLETE: "Кредитная история неполная",
  CREDIT_HISTORY_CONFLICT: "Противоречия в кредитной истории",
  COMPANY_STATUS_UNKNOWN: "Статус компании неизвестен",
  REGISTRATION_UNKNOWN: "Дата регистрации неизвестна",
};

export const freshnessLabel: Record<string, string> = {
  CURRENT: "Актуально",
  PENDING: "Обновляется",
  STALE: "Устарело",
};
