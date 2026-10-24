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
};

export const documentHint: Record<DocumentType, string> = {
  BANK_STATEMENT: "За последние 12 месяцев, из любого банка",
  BUSINESS_PLAN: "Цель, план расходов и ожидаемый эффект",
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
};
