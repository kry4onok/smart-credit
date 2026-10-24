// In-browser stand-in for auth, client-service and bank-service, built from api/*.yaml.
// Enabled only with VITE_API_MOCK=true (npm run dev:mock). It imitates the asynchronous
// conveyor with timers so both roles can be clicked through without backend services.
// The real decision logic lives in bank-service; the rules here are a simplified copy for the demo.
import type {
  AuditEntry,
  BankCase,
  BankStatus,
  ClientApplication,
  ClientStatus,
  CompanyProfile,
  CurrentUser,
  DocumentMetadata,
  DocumentType,
  InfoRequest,
  Offer,
  Product,
  ScoreResult,
  TimelineEntry,
} from "../api/types";
import { clientStatusLabel } from "../lib/labels";
import { uuid } from "../lib/util";

const STEP_MS = 1500;
const KEY = "smart-credit-mock-v1";

type User = CurrentUser & { password: string; orgId?: string };

const ORG = {
  bakery: "20000000-0000-4000-8000-0000000000a1",
  warehouse: "20000000-0000-4000-8000-0000000000a2",
  service: "20000000-0000-4000-8000-0000000000a3",
  workshop: "20000000-0000-4000-8000-0000000000a4",
  closing: "20000000-0000-4000-8000-0000000000a5",
};

function client(id: string, email: string, full_name: string, orgId: string, inn: string, name: string): User {
  return { id, email, full_name, role: "CLIENT", can_submit: true, password: "demo", orgId, organization: { id: orgId, inn, name } };
}

const users: User[] = [
  client("30000000-0000-4000-8000-0000000000a1", "bakery@demo.local", "Ирина Тестовая", ORG.bakery, "7707083893", "ООО «Северная пекарня»"),
  client("30000000-0000-4000-8000-0000000000a2", "warehouse@demo.local", "Алексей Тестовый", ORG.warehouse, "7810755500", "ООО «Балтийский склад»"),
  client("30000000-0000-4000-8000-0000000000a3", "service@demo.local", "Мария Тестовая", ORG.service, "5401010012", "ООО «Сибирский сервис»"),
  client("30000000-0000-4000-8000-0000000000a4", "workshop@demo.local", "Павел Тестовый", ORG.workshop, "6671160003", "ООО «Уральская мастерская»"),
  client("30000000-0000-4000-8000-0000000000a5", "closing@demo.local", "Олег Тестовый", ORG.closing, "1111111117", "ООО «Закрытие»"),
  { id: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", email: "analyst@demo.local", full_name: "Анна Аналитик", role: "ANALYST", can_submit: false, password: "demo", organization: null },
];

const rub = (rubles: number) => ({ amount_kopecks: Math.round(rubles * 100), currency: "RUB" as const });

const PRODUCTS: (Product & { maxShareBps: number; minAgeMonths: number; rates: Record<"A" | "B", number> })[] = [
  {
    id: "11111111-1111-4111-8111-111111111111",
    code: "EXPRESS",
    name: "Экспресс-оборотный",
    description: "Небольшая сумма на текущие расходы: закупки, зарплаты, кассовые разрывы. Решение обычно в день обращения.",
    purpose_tags: ["WORKING_CAPITAL"],
    min_amount: rub(100_000),
    max_amount: rub(2_000_000),
    min_term_months: 3,
    max_term_months: 12,
    indicative_rate_bps: 1900,
    required_document_types: ["BANK_STATEMENT"],
    version: 1,
    maxShareBps: 2500,
    minAgeMonths: 6,
    rates: { A: 1900, B: 2100 },
  },
  {
    id: "22222222-2222-4222-8222-222222222222",
    code: "DEVELOPMENT",
    name: "Кредит на развитие",
    description: "Для оборудования, новой точки или крупного проекта. Срок до трёх лет, нужен бизнес-план.",
    purpose_tags: ["WORKING_CAPITAL", "EQUIPMENT", "EXPANSION"],
    min_amount: rub(500_000),
    max_amount: rub(8_000_000),
    min_term_months: 6,
    max_term_months: 36,
    indicative_rate_bps: 1800,
    required_document_types: ["BANK_STATEMENT", "BUSINESS_PLAN"],
    version: 1,
    maxShareBps: 5000,
    minAgeMonths: 12,
    rates: { A: 1800, B: 2000 },
  },
];

const publicProduct = ({ maxShareBps: _a, minAgeMonths: _b, rates: _c, ...p }: (typeof PRODUCTS)[number]): Product => p;

// Synthetic bureau answers and scores, consistent with seed/personas.yaml.
type Persona = {
  checks: NonNullable<BankCase["checks"]>;
  score: number;
  grade: "A" | "B" | "C" | "D" | "E";
  factors: [number, number, number, number];
  askInfoFirst?: boolean;
};

const personas: Record<string, Persona> = {
  "7707083893": { checks: chk("ACTIVE", "2019-04-15", "10.71", 1_200_000, 0, 0), score: 850, grade: "A", factors: [50, 75, 25, 200] },
  "7810755500": { checks: chk("ACTIVE", "2016-09-20", "52.10", 3_500_000, 1, 12), score: 590, grade: "C", factors: [50, 25, -35, 50] },
  "5401010012": { checks: chk("ACTIVE", "2021-02-10", "43.21", 900_000, 0, 0), score: 720, grade: "B", factors: [25, 0, -5, 200], askInfoFirst: true },
  "6671160003": { checks: chk("ACTIVE", "2018-11-05", "25.62", 2_100_000, 0, 0), score: 760, grade: "B", factors: [50, 25, -15, 200] },
  "1111111117": { checks: chk("LIQUIDATING", "2017-03-01", "47.11", 4_000_000, 3, 95), score: 300, grade: "E", factors: [50, -100, -50, -100] },
};

function chk(company_status: "ACTIVE" | "LIQUIDATING", registered_at: string, okved: string, debt: number, overdue: number, maxDays: number) {
  return { company_status, registered_at, okved, total_debt: rub(debt), overdue_count_12m: overdue, max_overdue_days: maxDays, bureau_error: null };
}

const factorNames = ["business_age", "profit_margin", "bureau_debt_load", "payment_discipline"] as const;
const factorReason: Record<(typeof factorNames)[number], [string, string]> = {
  business_age: ["STABLE_BUSINESS", "SHORT_HISTORY"],
  profit_margin: ["HEALTHY_PROFIT_MARGIN", "LOW_PROFIT_MARGIN"],
  bureau_debt_load: ["MANAGEABLE_BUREAU_DEBT", "HIGH_BUREAU_DEBT"],
  payment_discipline: ["GOOD_PAYMENT_DISCIPLINE", "RECENT_OVERDUE"],
};

function scoreFor(p: Persona): ScoreResult {
  const contributions = factorNames.map((f, i) => ({ factor_name: f, points: p.factors[i], description: "" }));
  const reasons = contributions
    .filter((c) => c.points !== 0)
    .sort((a, b) => Math.abs(b.points) - Math.abs(a.points))
    .slice(0, 3)
    .map((c) => ({
      code: factorReason[c.factor_name][c.points > 0 ? 0 : 1] as ScoreResult["reasons"][number]["code"],
      description: "",
      direction: (c.points > 0 ? "DECREASES_RISK" : "INCREASES_RISK") as "DECREASES_RISK" | "INCREASES_RISK",
      contribution: Math.abs(c.points),
    }));
  return {
    assessment_status: "ASSESSED",
    score: p.score,
    grade: p.grade,
    reasons,
    model_version: "monotone-scorecard-v1",
    feature_schema_version: "borrower-features-v1",
    data_issues: [],
    factor_contributions: contributions,
    baseline_score: 500,
    as_of_date: new Date().toISOString().slice(0, 10),
  };
}

function personaFor(company: CompanyProfile): Persona {
  const known = personas[company.inn];
  if (known) return known;
  const age = monthsSince(company.registered_at);
  const margin = company.revenue_12m.amount_kopecks > 0 ? company.profit_12m.amount_kopecks / company.revenue_12m.amount_kopecks : 0;
  const factors: Persona["factors"] = [age >= 36 ? 50 : age >= 12 ? 25 : -50, margin >= 0.1 ? 75 : margin > 0 ? 0 : -100, company.obligations.amount_kopecks > company.revenue_12m.amount_kopecks / 2 ? -50 : 25, 150];
  const score = Math.max(0, Math.min(1000, 500 + factors.reduce((a, b) => a + b, 0)));
  const grade = score >= 800 ? "A" : score >= 650 ? "B" : score >= 500 ? "C" : score >= 350 ? "D" : "E";
  return { checks: chk("ACTIVE", company.registered_at, company.okved, company.obligations.amount_kopecks / 100, 0, 0), score, grade, factors };
}

function monthsSince(isoDate: string): number {
  const d = new Date(isoDate);
  const now = new Date();
  return (now.getFullYear() - d.getFullYear()) * 12 + now.getMonth() - d.getMonth();
}

type BankPart = {
  status: BankStatus;
  nextAt: number | null;
  version: number;
  assignee?: string;
  checks?: BankCase["checks"];
  score?: ScoreResult;
  triggered_rules: string[];
  recommendation?: BankCase["recommendation"];
  public_reason?: string;
  infoRounds: number;
  audit: AuditEntry[];
};

type ClientEvent = { type: "INFO" | "ACCEPT" | "DECLINE" | "SIGN" | "CANCEL"; at: number; reason?: string };

type MockApp = {
  orgId: string;
  app: Omit<ClientApplication, "status" | "stage_label" | "next_action" | "allowed_actions">;
  draft: boolean;
  bank?: BankPart;
  event?: ClientEvent;
  shownStatus?: ClientStatus;
  timeline: TimelineEntry[];
};

type State = {
  session: string | null;
  companies: Record<string, { version: number; profile: CompanyProfile }>;
  apps: MockApp[];
};

function load(): State {
  try {
    const raw = localStorage.getItem(KEY);
    if (raw) return JSON.parse(raw) as State;
  } catch {
    // Storage may be unavailable in a private window; start from the seed.
  }
  return seed();
}

function save() {
  try {
    localStorage.setItem(KEY, JSON.stringify(state));
  } catch {
    // Demo keeps working in memory.
  }
}

export function resetMock() {
  const session = state.session;
  state = seed();
  state.session = session;
  save();
}

function profile(inn: string, name: string, registered: string, okved: string, revenue: number, profit: number, obligations: number): CompanyProfile {
  return { inn, name, okved, registered_at: registered, revenue_12m: rub(revenue), profit_12m: rub(profit), obligations: rub(obligations) };
}

function seed(): State {
  const s: State = {
    session: null,
    companies: {
      [ORG.warehouse]: { version: 1, profile: profile("7810755500", "ООО «Балтийский склад»", "2016-09-20", "52.10", 15_000_000, 1_800_000, 3_500_000) },
      [ORG.service]: { version: 1, profile: profile("5401010012", "ООО «Сибирский сервис»", "2021-02-10", "43.21", 5_200_000, 420_000, 900_000) },
      [ORG.workshop]: { version: 1, profile: profile("6671160003", "ООО «Уральская мастерская»", "2018-11-05", "25.62", 11_000_000, 1_200_000, 2_100_000) },
      [ORG.closing]: { version: 1, profile: profile("1111111117", "ООО «Закрытие»", "2017-03-01", "47.11", 3_000_000, -200_000, 4_000_000) },
    },
    apps: [],
  };
  const now = Date.now();
  // A case already waiting for the analyst, so the queue is not empty on first login.
  const waiting = newApp(s, ORG.warehouse, PRODUCTS[0], new Date(now - 3 * 86400_000).toISOString());
  waiting.app.form = { amount: rub(1_800_000), term_months: 12, purpose: "WORKING_CAPITAL" };
  waiting.app.documents = [demoDoc("BANK_STATEMENT", "vypiska_2026.pdf", waiting.app.created_at)];
  waiting.draft = false;
  waiting.bank = { status: "SUBMITTED", nextAt: 0, version: 1, triggered_rules: [], infoRounds: 0, audit: [] };
  audit(waiting, "CASE_RECEIVED", "CLIENT", undefined, "SUBMITTED", "Заявка получена из client.events");
  while (autoStatuses.includes(waiting.bank.status)) {
    waiting.bank.nextAt = 0;
    step(waiting, now);
  }
  // A filled draft that exceeds the product limit relative to revenue: the analyst makes a counter offer.
  const large = newApp(s, ORG.workshop, PRODUCTS[1], new Date(now - 3600_000).toISOString());
  large.app.form = { amount: rub(7_000_000), term_months: 30, purpose: "EQUIPMENT" };
  large.app.documents = [demoDoc("BANK_STATEMENT", "vypiska.pdf", large.app.created_at), demoDoc("BUSINESS_PLAN", "plan_stanok.pdf", large.app.created_at)];
  return s;
}

function demoDoc(type: DocumentType, name: string, at: string): DocumentMetadata {
  return { id: uuid(), document_type: type, file_name: name, sha256: "0".repeat(64), uploaded_at: at, is_mock: true };
}

function newApp(s: State, orgId: string, product: Product, createdAt = new Date().toISOString()): MockApp {
  const m: MockApp = {
    orgId,
    draft: true,
    timeline: [],
    app: {
      id: uuid(),
      product_id: product.id,
      product_version: product.version,
      company: structuredClone(s.companies[orgId].profile),
      documents: [],
      created_at: createdAt,
      updated_at: createdAt,
      version: 1,
    },
  };
  s.apps.push(m);
  return m;
}

const productOf = (m: MockApp) => PRODUCTS.find((p) => p.id === m.app.product_id)!;

// ---------- the conveyor ----------

const autoStatuses: BankStatus[] = ["SUBMITTED", "VALIDATION", "ENRICHMENT", "SCORING", "DECISION", "SIGNING"];

function audit(m: MockApp, action: string, actor: AuditEntry["actor_type"], from: BankStatus | undefined, to: BankStatus | undefined, reason: string, actorId?: string) {
  m.bank!.audit.push({ id: uuid(), action, actor_type: actor, actor_id: actorId, from_status: from, to_status: to, reason, occurred_at: new Date().toISOString() });
}

function move(m: MockApp, to: BankStatus, reason: string, actor: AuditEntry["actor_type"] = "SYSTEM", actorId?: string) {
  const b = m.bank!;
  audit(m, "TRANSITION", actor, b.status, to, reason, actorId);
  b.status = to;
  b.version++;
  b.nextAt = autoStatuses.includes(to) ? Date.now() + STEP_MS : null;
  touch(m);
}

function touch(m: MockApp) {
  m.app.version++;
  m.app.updated_at = new Date().toISOString();
}

function step(m: MockApp, now: number) {
  const b = m.bank!;
  if (!autoStatuses.includes(b.status)) {
    b.nextAt = null;
    return false;
  }
  if (b.nextAt === null || b.nextAt > now) return false;
  const product = productOf(m);
  const persona = personaFor(m.app.company);
  switch (b.status) {
    case "SUBMITTED":
      move(m, "VALIDATION", "Дело создано, запущена проверка полноты");
      break;
    case "VALIDATION": {
      const missing = product.required_document_types.filter((t) => !m.app.documents.some((d) => d.document_type === t));
      if (missing.length || (persona.askInfoFirst && b.infoRounds === 0)) {
        b.infoRounds++;
        m.app.info_request = {
          id: uuid(),
          message: missing.length
            ? "Не хватает обязательных документов. Приложите их, чтобы банк продолжил рассмотрение."
            : "Уточните текущие кредиты и займы и приложите свежую выписку по счёту: в выписке за прошлый период нет последних двух месяцев.",
          required_fields: missing.length ? [] : ["COMPANY_OBLIGATIONS"],
          required_document_types: missing.length ? missing : ["BANK_STATEMENT"],
          created_at: new Date().toISOString(),
        };
        move(m, "NEEDS_INFO", "Неполные данные: создан запрос клиенту");
      } else move(m, "ENRICHMENT", "Обязательные поля и документы на месте");
      break;
    }
    case "ENRICHMENT":
      b.checks = persona.checks;
      audit(m, "BUREAU_CALL", "SYSTEM", undefined, undefined, "GetCompanyProfile: OK; GetCreditHistory: OK");
      move(m, "SCORING", "Получены ответы ЕГРЮЛ/ФНС и БКИ (моки)");
      break;
    case "SCORING":
      b.score = scoreFor(persona);
      move(m, "DECISION", `Score ${persona.score}, grade ${persona.grade}`);
      break;
    case "DECISION":
      decide(m, persona, product);
      break;
    case "SIGNING":
      move(m, "DISBURSED", "Демо-выдача после подписания (симуляция)");
      break;
  }
  return true;
}

function decide(m: MockApp, persona: Persona, product: (typeof PRODUCTS)[number]) {
  const b = m.bank!;
  const form = m.app.form!;
  if (persona.checks.company_status !== "ACTIVE" || persona.checks.max_overdue_days >= 90) {
    b.triggered_rules = ["STOP_FACTOR_COMPANY_STATUS"];
    b.recommendation = "REJECT";
    b.public_reason = "По данным государственного реестра компания находится в процессе ликвидации. Банк не может выдать кредит в этой ситуации.";
    return move(m, "REJECTED", "Стоп-фактор: статус компании LIQUIDATING");
  }
  if (persona.grade === "E") {
    b.recommendation = "REJECT";
    b.public_reason = "Сейчас банк не готов выдать кредит: финансовые показатели не соответствуют требованиям продукта.";
    return move(m, "REJECTED", "Grade E");
  }
  if (persona.grade === "C" || persona.grade === "D") {
    b.triggered_rules = ["GRADE_REQUIRES_REVIEW"];
    b.recommendation = "MANUAL_REVIEW";
    return move(m, "MANUAL_REVIEW", `Grade ${persona.grade}: нужен ручной разбор`);
  }
  const rules: string[] = [];
  if (form.amount.amount_kopecks * 10000 > m.app.company.revenue_12m.amount_kopecks * product.maxShareBps) rules.push("LARGE_REQUEST");
  if (monthsSince(persona.checks.registered_at ?? m.app.company.registered_at) < product.minAgeMonths) rules.push("COMPANY_TOO_YOUNG");
  if (persona.checks.max_overdue_days > 30) rules.push("OVERDUE_OVER_30");
  if (rules.length) {
    b.triggered_rules = rules;
    b.recommendation = "COUNTER";
    return move(m, "MANUAL_REVIEW", `Правила продукта не выполнены: ${rules.join(", ")}`);
  }
  b.recommendation = "APPROVE";
  m.app.offer = offer("OFFER", product, form.amount, form.term_months, product.rates[persona.grade], "Банк одобрил заявку на запрошенных условиях: у компании стабильная выручка и нет просрочек.");
  move(m, "OFFER", "Автооферта: правила продукта выполнены");
}

function offer(kind: Offer["kind"], product: Product, amount: Offer["amount"], term: number, rateBps: number, explanation: string): Offer {
  return {
    id: uuid(),
    kind,
    product_id: product.id,
    product_version: product.version,
    product_name: product.name,
    amount,
    term_months: term,
    rate_bps: rateBps,
    expires_at: new Date(Date.now() + 72 * 3600_000).toISOString(),
    explanation,
  };
}

// Client events reach the bank after a delay, as they would through the outbox and Kafka.
function deliver(m: MockApp, now: number) {
  const e = m.event;
  if (!e || e.at > now || !m.bank) return;
  m.event = undefined;
  const b = m.bank;
  if (e.type === "INFO" && b.status === "NEEDS_INFO") {
    m.app.info_request = undefined;
    b.score = undefined;
    move(m, "VALIDATION", "InfoProvided: клиент ответил на запрос", "CLIENT");
  } else if (e.type === "ACCEPT" && (b.status === "OFFER" || b.status === "COUNTER_OFFER")) {
    move(m, "OFFER_ACCEPTED", "OfferAccepted: клиент принял оферту", "CLIENT");
  } else if (e.type === "DECLINE") {
    b.public_reason = "Вы отказались от предложения банка. Заявка закрыта.";
    move(m, "CANCELLED", "OfferDeclined", "CLIENT");
  } else if (e.type === "SIGN" && b.status === "OFFER_ACCEPTED") {
    move(m, "SIGNING", "ContractSigned: демо-подпись", "CLIENT");
  } else if (e.type === "CANCEL") {
    b.public_reason = e.reason ? `Заявка отозвана: ${e.reason}` : "Заявка отозвана по вашему запросу.";
    move(m, "CANCELLED", "ApplicationCancelled", "CLIENT");
  } else touch(m);
}

function tick() {
  const now = Date.now();
  let changed = false;
  for (const m of state.apps) {
    if (m.event && m.event.at <= now) {
      deliver(m, now);
      changed = true;
    }
    if (m.bank) while (step(m, now)) changed = true;
    if (m.bank?.status && (m.bank.status === "OFFER" || m.bank.status === "COUNTER_OFFER") && m.app.offer && new Date(m.app.offer.expires_at).getTime() < now) {
      move(m, "EXPIRED", "Срок оферты истёк");
      changed = true;
    }
    changed = recordTimeline(m) || changed;
  }
  if (changed) save();
}

// ---------- client projection ----------

function clientStatus(m: MockApp): ClientStatus {
  if (m.draft) return "DRAFT";
  switch (m.bank!.status) {
    case "SUBMITTED":
      return "SUBMITTED";
    case "NEEDS_INFO":
      return "NEEDS_INFO";
    case "OFFER":
    case "COUNTER_OFFER":
      return "OFFER_AVAILABLE";
    case "OFFER_ACCEPTED":
    case "SIGNING":
      return "FORMALIZING";
    case "DISBURSED":
      return "DISBURSED";
    case "REJECTED":
      return "REJECTED";
    case "CANCELLED":
    case "EXPIRED":
      return "CLOSED";
    default:
      return "UNDER_REVIEW";
  }
}

const nextAction: Record<ClientStatus, string> = {
  DRAFT: "Заполните условия, приложите документы и отправьте заявку",
  SUBMITTED: "Банк получил заявку и начинает проверку",
  UNDER_REVIEW: "Банк проверяет данные и оценивает заявку. Обычно это занимает несколько минут",
  NEEDS_INFO: "Ответьте на вопрос банка, чтобы продолжить рассмотрение",
  OFFER_AVAILABLE: "Посмотрите условия и примите предложение до окончания срока",
  FORMALIZING: "Подпишите договор, чтобы банк выдал кредит",
  DISBURSED: "Кредит выдан, деньги на расчётном счёте",
  REJECTED: "Банк отказал в кредите. Причина указана ниже",
  CLOSED: "Заявка закрыта",
};

const pendingByEvent: Record<ClientEvent["type"], NonNullable<ClientApplication["pending_action"]>> = {
  INFO: "INFO_SENT",
  ACCEPT: "ACCEPT_PENDING",
  DECLINE: "DECLINE_PENDING",
  SIGN: "SIGN_PENDING",
  CANCEL: "CANCEL_PENDING",
};

function view(m: MockApp): ClientApplication {
  const status = clientStatus(m);
  const pending = m.event ? pendingByEvent[m.event.type] : undefined;
  const allowed: ClientApplication["allowed_actions"] = [];
  if (!pending) {
    if (status === "DRAFT") allowed.push("EDIT_DRAFT", "UPLOAD_DOCUMENT", "SUBMIT");
    if (status === "NEEDS_INFO") allowed.push("PROVIDE_INFO", "UPLOAD_DOCUMENT");
    if (status === "OFFER_AVAILABLE") allowed.push("ACCEPT_OFFER", "DECLINE_OFFER");
    if (m.bank?.status === "OFFER_ACCEPTED") allowed.push("SIGN");
    // As in client-service: cancellable until the contract is signed.
    if (["SUBMITTED", "UNDER_REVIEW", "NEEDS_INFO", "OFFER_AVAILABLE"].includes(status) || m.bank?.status === "OFFER_ACCEPTED") allowed.push("CANCEL");
  }
  // As in client-service: the offer is visible only while it awaits an answer.
  const showOffer = m.bank && ["OFFER", "COUNTER_OFFER"].includes(m.bank.status);
  const f = m.app.form;
  return {
    ...m.app,
    // client-service returns the form only when amount, term and purpose are all set.
    form: f?.amount?.amount_kopecks && f.term_months && f.purpose ? f : undefined,
    offer: showOffer ? m.app.offer : undefined,
    info_request: m.bank?.status === "NEEDS_INFO" ? m.app.info_request : undefined,
    public_reason: m.bank?.public_reason,
    status,
    stage_label: clientStatusLabel[status],
    next_action: nextAction[status],
    allowed_actions: allowed,
    pending_action: pending,
  };
}

function recordTimeline(m: MockApp): boolean {
  const status = clientStatus(m);
  if (m.shownStatus === status) return false;
  m.shownStatus = status;
  const text = status === "REJECTED" || status === "CLOSED" ? (m.bank?.public_reason ?? nextAction[status]) : status === "NEEDS_INFO" ? (m.app.info_request?.message ?? nextAction[status]) : nextAction[status];
  m.timeline.push({ id: uuid(), status, message: text, occurred_at: new Date().toISOString() });
  return true;
}

// ---------- bank projection ----------

function caseView(m: MockApp, me: User): BankCase {
  const b = m.bank!;
  const actions: BankCase["allowed_actions"] = ["VIEW"];
  if (b.status === "MANUAL_REVIEW" && !b.assignee) actions.unshift("TAKE");
  if (b.status === "MANUAL_REVIEW" && b.assignee === me.id) actions.unshift("DECIDE");
  const form = m.app.form!;
  return {
    id: m.app.id,
    status: b.status,
    product_id: m.app.product_id,
    product_name: productOf(m).name,
    product_version: m.app.product_version,
    company: m.app.company,
    application: { amount: form.amount, term_months: form.term_months, purpose: form.purpose, document_types: [...new Set(m.app.documents.map((d) => d.document_type))] },
    checks: b.checks,
    score: b.score,
    triggered_rules: b.triggered_rules,
    recommendation: b.recommendation,
    assignee_id: b.assignee,
    info_request: b.status === "NEEDS_INFO" ? m.app.info_request : undefined,
    offer: ["OFFER", "COUNTER_OFFER", "OFFER_ACCEPTED", "SIGNING", "DISBURSED"].includes(b.status) ? m.app.offer : undefined,
    public_reason: b.public_reason,
    allowed_actions: actions,
    audit_version: b.audit.length,
    created_at: m.app.created_at,
    updated_at: m.app.updated_at,
    version: b.version,
  };
}

// ---------- HTTP layer ----------

function json(status: number, body?: unknown, headers: Record<string, string> = {}): Response {
  return new Response(body === undefined ? null : JSON.stringify(body), { status, headers: { "Content-Type": "application/json", ...headers } });
}

function problem(status: number, code: string, detail: string): Response {
  return new Response(JSON.stringify({ type: "about:blank", title: detail, status, detail, code, trace_id: uuid() }), {
    status,
    headers: { "Content-Type": "application/problem+json" },
  });
}

class Fail extends Error {
  constructor(readonly response: Response) {
    super("mock failure");
  }
}

const fail = (status: number, code: string, detail: string): never => {
  throw new Fail(problem(status, code, detail));
};

function principal(request: Request): User {
  const token = /^Bearer mock\.(.+)$/.exec(request.headers.get("Authorization") ?? "")?.[1];
  const user = users.find((u) => u.id === token);
  if (!user || state.session !== user.id) fail(401, "UNAUTHORIZED", "Требуется действительная сессия.");
  return user!;
}

function requireClient(request: Request): User {
  const u = principal(request);
  if (u.role !== "CLIENT") fail(403, "FORBIDDEN", "Требуется роль CLIENT.");
  return u;
}

function requireAnalyst(request: Request): User {
  const u = principal(request);
  if (u.role === "CLIENT") fail(403, "FORBIDDEN", "Требуется роль ANALYST или ADMIN.");
  return u;
}

function ownApp(user: User, id: string): MockApp {
  const m = state.apps.find((a) => a.app.id === id && a.orgId === user.orgId);
  if (!m) fail(404, "NOT_FOUND", "Заявка не найдена.");
  return m!;
}

function bankCase(id: string): MockApp {
  const m = state.apps.find((a) => a.app.id === id && a.bank);
  if (!m) fail(404, "NOT_FOUND", "Дело не найдено.");
  return m!;
}

function checkVersion(request: Request, version: number) {
  if (request.headers.get("If-Match") !== `"${version}"`) fail(409, "VERSION_CONFLICT", "Данные успели измениться. Обновите страницу и повторите действие.");
}

const later = () => Date.now() + STEP_MS;

type Handler = (request: Request, params: string[]) => Promise<Response> | Response;
const A = "/api/client/applications/([0-9a-f-]{36})";
const B = "/api/backoffice/applications/([0-9a-f-]{36})";

const routes: [string, RegExp, Handler][] = [
  [
    "POST",
    /^\/api\/auth\/login$/,
    async (r) => {
      const { email, password } = (await r.json()) as { email: string; password: string };
      const u = users.find((x) => x.email === email.toLowerCase() && x.password === password);
      if (!u) return problem(401, "INVALID_CREDENTIALS", "Неверная почта или пароль.");
      state.session = u.id;
      return json(200, { access_token: `mock.${u.id}`, token_type: "Bearer", expires_in: 900 });
    },
  ],
  ["POST", /^\/api\/auth\/refresh$/, () => (state.session ? json(200, { access_token: `mock.${state.session}`, token_type: "Bearer", expires_in: 900 }) : problem(401, "UNAUTHORIZED", "Нет сессии."))],
  [
    "POST",
    /^\/api\/auth\/logout$/,
    () => {
      state.session = null;
      return json(204);
    },
  ],
  [
    "GET",
    /^\/api\/auth\/me$/,
    (r) => {
      const { password: _p, orgId: _o, ...me } = principal(r);
      return json(200, me);
    },
  ],

  [
    "GET",
    /^\/api\/client\/company$/,
    (r) => {
      const c = state.companies[requireClient(r).orgId!];
      return c ? json(200, c, { ETag: `"${c.version}"` }) : problem(404, "COMPANY_NOT_FOUND", "Профиль компании ещё не заполнен.");
    },
  ],
  [
    "PUT",
    /^\/api\/client\/company$/,
    async (r) => {
      const u = requireClient(r);
      const body = (await r.json()) as CompanyProfile;
      const existing = state.companies[u.orgId!];
      if (existing) checkVersion(r, existing.version);
      const next = { version: (existing?.version ?? 0) + 1, profile: body };
      state.companies[u.orgId!] = next;
      return json(200, next, { ETag: `"${next.version}"` });
    },
  ],
  [
    "GET",
    /^\/api\/client\/products$/,
    (r) => {
      requireClient(r);
      const purpose = new URL(r.url).searchParams.get("purpose");
      const items = PRODUCTS.filter((p) => !purpose || p.purpose_tags.includes(purpose as never)).map(publicProduct);
      return json(200, { items, catalog_version: 1 });
    },
  ],
  [
    "GET",
    /^\/api\/client\/products\/([0-9a-f-]{36})$/,
    (r, [id]) => {
      requireClient(r);
      const p = PRODUCTS.find((x) => x.id === id);
      return p ? json(200, publicProduct(p)) : problem(404, "NOT_FOUND", "Продукт не найден.");
    },
  ],
  [
    "POST",
    /^\/api\/client\/applications$/,
    async (r) => {
      const u = requireClient(r);
      const { product_id } = (await r.json()) as { product_id: string };
      const product = PRODUCTS.find((p) => p.id === product_id);
      if (!product) return problem(404, "PRODUCT_NOT_FOUND", "Продукт не найден.");
      if (!state.companies[u.orgId!]) return problem(422, "COMPANY_REQUIRED", "Сначала заполните сведения о компании.");
      return json(201, view(newApp(state, u.orgId!, product)));
    },
  ],
  [
    "GET",
    /^\/api\/client\/applications$/,
    (r) => {
      const u = requireClient(r);
      const status = new URL(r.url).searchParams.get("status");
      const items = state.apps
        .filter((m) => m.orgId === u.orgId)
        .map(view)
        .filter((a) => !status || a.status === status)
        .sort((a, b) => b.created_at.localeCompare(a.created_at))
        .map((a) => ({ id: a.id, product_id: a.product_id, status: a.status, next_action: a.next_action, amount: a.form?.amount, created_at: a.created_at }));
      return json(200, { items, next_cursor: null });
    },
  ],
  [
    "GET",
    new RegExp(`^${A}$`),
    (r, [id]) => {
      const a = view(ownApp(requireClient(r), id));
      return json(200, a, { ETag: `"${a.version}"` });
    },
  ],
  [
    "PATCH",
    new RegExp(`^${A}$`),
    async (r, [id]) => {
      const m = ownApp(requireClient(r), id);
      if (!m.draft) return problem(409, "NOT_DRAFT", "Заявка уже отправлена, её нельзя изменить.");
      checkVersion(r, m.app.version);
      const patch = (await r.json()) as Partial<NonNullable<ClientApplication["form"]>>;
      // A draft may be filled field by field; missing fields stay missing.
      const current = m.app.form ?? ({} as NonNullable<ClientApplication["form"]>);
      m.app.form = { ...current, ...patch } as ClientApplication["form"];
      touch(m);
      return json(200, view(m));
    },
  ],
  [
    "POST",
    new RegExp(`^${A}/documents$`),
    async (r, [id]) => {
      const m = ownApp(requireClient(r), id);
      const form = await r.formData();
      const type = form.get("document_type") as DocumentType;
      const file = form.get("file");
      if (!(file instanceof File)) return problem(400, "FILE_REQUIRED", "Файл не передан.");
      if (file.size > 5 * 1024 * 1024) return problem(422, "FILE_TOO_LARGE", "Файл больше 5 МБ.");
      const allowed = m.draft ? productOf(m).required_document_types : m.bank?.status === "NEEDS_INFO" ? (m.app.info_request?.required_document_types ?? []) : [];
      if (!allowed.includes(type)) return problem(422, "DOCUMENT_TYPE_NOT_ALLOWED", "Этот документ сейчас не требуется.");
      const doc: DocumentMetadata = { id: uuid(), document_type: type, file_name: file.name, sha256: await sha256(file), uploaded_at: new Date().toISOString(), is_mock: true };
      if (m.draft) m.app.documents = m.app.documents.filter((d) => d.document_type !== type);
      m.app.documents.push(doc);
      touch(m);
      return json(201, doc);
    },
  ],
  [
    "POST",
    new RegExp(`^${A}/submit$`),
    (r, [id]) => {
      const u = requireClient(r);
      const m = ownApp(u, id);
      if (!m.draft) return json(200, view(m));
      if (!u.can_submit) return problem(403, "SUBMIT_FORBIDDEN", "У вас нет права отправлять заявки.");
      const p = productOf(m);
      const f = m.app.form;
      if (!f?.amount?.amount_kopecks || !f.term_months || !f.purpose) return problem(422, "FORM_INCOMPLETE", "Заполните сумму, срок и цель кредита.");
      if (f.amount.amount_kopecks < p.min_amount.amount_kopecks || f.amount.amount_kopecks > p.max_amount.amount_kopecks) return problem(422, "AMOUNT_OUT_OF_RANGE", "Сумма вне диапазона продукта.");
      if (f.term_months < p.min_term_months || f.term_months > p.max_term_months) return problem(422, "TERM_OUT_OF_RANGE", "Срок вне диапазона продукта.");
      m.draft = false;
      m.bank = { status: "SUBMITTED", nextAt: later(), version: 1, triggered_rules: [], infoRounds: 0, audit: [] };
      audit(m, "CASE_RECEIVED", "CLIENT", undefined, "SUBMITTED", "ApplicationSubmitted получено из client.events", u.id);
      touch(m);
      recordTimeline(m);
      return json(200, view(m));
    },
  ],
  [
    "POST",
    new RegExp(`^${A}/info$`),
    async (r, [id]) => {
      const m = ownApp(requireClient(r), id);
      const body = (await r.json()) as { request_id: string; company?: CompanyProfile; fields?: Partial<NonNullable<ClientApplication["form"]>> };
      if (m.bank?.status !== "NEEDS_INFO" || m.app.info_request?.id !== body.request_id) return problem(409, "INFO_REQUEST_MISMATCH", "Запрос банка уже закрыт или изменился.");
      if (body.company) m.app.company = body.company;
      if (body.fields) m.app.form = { ...m.app.form!, ...body.fields };
      m.event = { type: "INFO", at: later() };
      touch(m);
      return json(202, { application_id: id, pending_action: "INFO_SENT" });
    },
  ],
  ...(
    [
      ["offer/accept", "ACCEPT", ["OFFER", "COUNTER_OFFER"]],
      ["offer/decline", "DECLINE", ["OFFER", "COUNTER_OFFER"]],
      ["sign", "SIGN", ["OFFER_ACCEPTED"]],
    ] as const
  ).map(
    ([path, type, statuses]): [string, RegExp, Handler] => [
      "POST",
      new RegExp(`^${A}/${path}$`),
      async (r, [id]) => {
        const m = ownApp(requireClient(r), id);
        const { offer_id } = (await r.json()) as { offer_id: string };
        if (!m.bank || !(statuses as readonly string[]).includes(m.bank.status) || m.app.offer?.id !== offer_id || m.event) {
          return problem(409, "OFFER_NOT_ACTIONABLE", "Предложение уже неактуально. Обновите страницу.");
        }
        m.event = { type, at: later() };
        touch(m);
        return json(202, { application_id: id, pending_action: pendingByEvent[type] });
      },
    ],
  ),
  [
    "POST",
    new RegExp(`^${A}/cancel$`),
    async (r, [id]) => {
      const m = ownApp(requireClient(r), id);
      const body = (await r.json().catch(() => ({}))) as { reason?: string };
      if (!m.bank || m.event || ["SIGNING", "DISBURSED", "REJECTED", "CANCELLED", "EXPIRED"].includes(m.bank.status)) {
        return problem(409, "CANNOT_CANCEL", "Заявку уже нельзя отозвать.");
      }
      m.event = { type: "CANCEL", at: later(), reason: body.reason };
      touch(m);
      return json(202, { application_id: id, pending_action: "CANCEL_PENDING" });
    },
  ],
  ["GET", new RegExp(`^${A}/timeline$`), (r, [id]) => json(200, { items: ownApp(requireClient(r), id).timeline })],

  [
    "GET",
    /^\/api\/backoffice\/applications$/,
    (r) => {
      const me = requireAnalyst(r);
      const q = new URL(r.url).searchParams;
      const status = q.get("status");
      const assignee = q.get("assignee");
      const items = state.apps
        .filter((m) => m.bank)
        .filter((m) => !status || m.bank!.status === status)
        .filter((m) => !assignee || (assignee === "me" ? m.bank!.assignee === me.id : assignee === "unassigned" ? !m.bank!.assignee : m.bank!.assignee === assignee))
        .sort((a, b) => Number(b.bank!.status === "MANUAL_REVIEW") - Number(a.bank!.status === "MANUAL_REVIEW") || a.app.created_at.localeCompare(b.app.created_at))
        .map((m) => ({
          id: m.app.id,
          status: m.bank!.status,
          product_id: m.app.product_id,
          requested_amount: m.app.form!.amount,
          company_name: m.app.company.name,
          assignee_id: m.bank!.assignee ?? null,
          created_at: m.app.created_at,
          version: m.bank!.version,
        }));
      return json(200, { items, next_cursor: null });
    },
  ],
  [
    "GET",
    new RegExp(`^${B}$`),
    (r, [id]) => {
      const c = caseView(bankCase(id), requireAnalyst(r));
      return json(200, c, { ETag: `"${c.version}"` });
    },
  ],
  [
    "POST",
    new RegExp(`^${B}/take$`),
    (r, [id]) => {
      const me = requireAnalyst(r);
      const m = bankCase(id);
      checkVersion(r, m.bank!.version);
      if (m.bank!.status !== "MANUAL_REVIEW") return problem(409, "NOT_MANUAL", "Дело не ждёт ручного решения.");
      if (m.bank!.assignee && m.bank!.assignee !== me.id) return problem(409, "ALREADY_ASSIGNED", "Дело уже взял другой аналитик.");
      m.bank!.assignee = me.id;
      m.bank!.version++;
      audit(m, "CASE_TAKEN", "ANALYST", undefined, undefined, "Дело взято в работу", me.id);
      return json(200, caseView(m, me));
    },
  ],
  [
    "POST",
    new RegExp(`^${B}/decision$`),
    async (r, [id]) => {
      const me = requireAnalyst(r);
      const m = bankCase(id);
      const b = m.bank!;
      checkVersion(r, b.version);
      if (b.status !== "MANUAL_REVIEW" || b.assignee !== me.id) return problem(409, "NOT_ASSIGNED", "Решение может принять только назначенный аналитик.");
      const d = (await r.json()) as {
        outcome: "APPROVE" | "COUNTER" | "REQUEST_INFO" | "REJECT";
        comment: string;
        public_reason?: string;
        offer?: { product_id: string; amount: Offer["amount"]; term_months: number; rate_bps: number };
        info_request?: Omit<InfoRequest, "id" | "created_at">;
      };
      if ((d.outcome === "REJECT" || d.outcome === "COUNTER") && !d.public_reason) return problem(422, "PUBLIC_REASON_REQUIRED", "Нужна причина для клиента.");
      if (d.outcome === "APPROVE" || d.outcome === "COUNTER") {
        const product = PRODUCTS.find((p) => p.id === d.offer?.product_id);
        if (!product || !d.offer) return problem(422, "PRODUCT_NOT_FOUND", "Продукт оферты не найден.");
        const o = d.offer;
        if (o.amount.amount_kopecks < product.min_amount.amount_kopecks || o.amount.amount_kopecks > product.max_amount.amount_kopecks) {
          return problem(422, "OFFER_AMOUNT_OUT_OF_RANGE", `Сумма вне диапазона продукта «${product.name}».`);
        }
        if (o.term_months < product.min_term_months || o.term_months > product.max_term_months) return problem(422, "OFFER_TERM_OUT_OF_RANGE", `Срок вне диапазона продукта «${product.name}».`);
        const missing = product.required_document_types.filter((t) => !m.app.documents.some((x) => x.document_type === t));
        if (missing.length) return problem(422, "DOCUMENTS_MISSING", "Для этого продукта у клиента нет нужных документов. Выберите «Запросить данные».");
        b.public_reason = d.public_reason;
        const kind = d.outcome === "APPROVE" && product.id === m.app.product_id ? "OFFER" : "COUNTER_OFFER";
        m.app.offer = offer(kind, product, o.amount, o.term_months, o.rate_bps, d.public_reason ?? "Банк одобрил заявку после проверки специалистом.");
        audit(m, "ANALYST_DECISION", "ANALYST", undefined, undefined, `${d.outcome}: ${d.comment}`, me.id);
        move(m, kind, "Решение аналитика", "ANALYST", me.id);
      } else if (d.outcome === "REQUEST_INFO") {
        b.public_reason = d.public_reason;
        m.app.info_request = { id: uuid(), created_at: new Date().toISOString(), ...d.info_request! };
        audit(m, "ANALYST_DECISION", "ANALYST", undefined, undefined, `REQUEST_INFO: ${d.comment}`, me.id);
        move(m, "NEEDS_INFO", "Аналитик запросил данные", "ANALYST", me.id);
      } else {
        b.public_reason = d.public_reason;
        audit(m, "ANALYST_DECISION", "ANALYST", undefined, undefined, `REJECT: ${d.comment}`, me.id);
        move(m, "REJECTED", "Отказ аналитика", "ANALYST", me.id);
      }
      b.assignee = undefined;
      recordTimeline(m);
      return json(200, caseView(m, me));
    },
  ],
  [
    "GET",
    new RegExp(`^${B}/audit$`),
    (r, [id]) => {
      requireAnalyst(r);
      return json(200, { items: bankCase(id).bank!.audit, next_cursor: null });
    },
  ],
  [
    "POST",
    new RegExp(`^${B}/summary$`),
    (r, [id]) => {
      const c = caseView(bankCase(id), requireAnalyst(r));
      const reasons = c.score?.reasons.map((x) => x.code).join(", ") || "нет";
      const text =
        `${c.company.name} просит ${(c.application.amount.amount_kopecks / 100).toLocaleString("ru-RU")} ₽ на ${c.application.term_months} мес. по продукту «${c.product_name}». ` +
        (c.score ? `Оценка риска: grade ${c.score.grade} (${c.score.score}), основные факторы: ${reasons}. ` : "Скоринг ещё не выполнен. ") +
        (c.triggered_rules?.length ? `Сработали правила: ${c.triggered_rules.join(", ")}.` : "Стоп-факторов не найдено.");
      return json(200, { text, source: "template" });
    },
  ],
];

async function sha256(file: File): Promise<string> {
  if (!crypto.subtle) return "0".repeat(64);
  const digest = await crypto.subtle.digest("SHA-256", await file.arrayBuffer());
  return [...new Uint8Array(digest)].map((b) => b.toString(16).padStart(2, "0")).join("");
}

// Initialised after all module constants: seed() relies on them.
let state: State = load();

export async function mockFetch(request: Request): Promise<Response> {
  const path = new URL(request.url).pathname;
  await new Promise((r) => setTimeout(r, 120));
  tick();
  for (const [method, pattern, handler] of routes) {
    const match = pattern.exec(path);
    if (match && method === request.method) {
      try {
        const response = await handler(request, match.slice(1));
        save();
        return response;
      } catch (e) {
        if (e instanceof Fail) return e.response;
        return problem(500, "MOCK_ERROR", String(e));
      }
    }
  }
  return problem(404, "NOT_FOUND", `Mock: ${request.method} ${path} не описан.`);
}
