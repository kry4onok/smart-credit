import type { Money, SignedMoney } from "../api/types";

const rubFmt = new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 0 });
const rubExact = new Intl.NumberFormat("ru-RU", { minimumFractionDigits: 0, maximumFractionDigits: 2 });

export function money(value: Money | SignedMoney | undefined, exact = false): string {
  if (!value) return "—";
  const rubles = value.amount_kopecks / 100;
  return `${(exact ? rubExact : rubFmt).format(rubles)} ₽`;
}

// Short form for headings: «3 млн ₽», «850 тыс. ₽».
export function moneyShort(value: Money | undefined): string {
  if (!value) return "—";
  const rubles = value.amount_kopecks / 100;
  if (Math.abs(rubles) >= 1_000_000) return `${trim(rubles / 1_000_000)} млн ₽`;
  if (Math.abs(rubles) >= 1_000) return `${trim(rubles / 1_000)} тыс. ₽`;
  return `${rubFmt.format(rubles)} ₽`;
}

function trim(n: number): string {
  return new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 1 }).format(n);
}

export function rub(amountKopecks: number): Money {
  return { amount_kopecks: amountKopecks, currency: "RUB" };
}

// Parses «1 250 000,50» into kopecks without floating point. Returns null if invalid.
export function parseRubles(input: string, allowNegative = false): number | null {
  // \s also covers the no-break spaces Intl uses as thousands separators.
  const cleaned = input.replace(/[\s₽]/g, "").replace(",", ".");
  const match = (allowNegative ? /^(-?)(\d+)(?:\.(\d{1,2}))?$/ : /^()(\d+)(?:\.(\d{1,2}))?$/).exec(cleaned);
  if (!match) return null;
  const [, sign, whole, fraction = ""] = match;
  const kopecks = Number(whole) * 100 + Number(fraction.padEnd(2, "0"));
  if (!Number.isSafeInteger(kopecks)) return null;
  return sign === "-" ? -kopecks : kopecks;
}

export function rublesInput(amountKopecks: number | undefined): string {
  if (amountKopecks === undefined) return "";
  const negative = amountKopecks < 0;
  const abs = Math.abs(amountKopecks);
  const whole = rubFmt.format(Math.floor(abs / 100));
  const fraction = abs % 100;
  return `${negative ? "-" : ""}${whole}${fraction ? "," + String(fraction).padStart(2, "0") : ""}`;
}

export function rate(bps: number | undefined): string {
  if (bps === undefined) return "—";
  return `${new Intl.NumberFormat("ru-RU", { maximumFractionDigits: 2 }).format(bps / 100)}%`;
}

export function months(n: number | undefined): string {
  if (n === undefined) return "—";
  const mod10 = n % 10;
  const mod100 = n % 100;
  const word = mod10 === 1 && mod100 !== 11 ? "месяц" : mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14) ? "месяца" : "месяцев";
  return `${n} ${word}`;
}

const dateFmt = new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "long", year: "numeric" });
const dateTimeFmt = new Intl.DateTimeFormat("ru-RU", { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });

export function date(value: string | undefined): string {
  return value ? dateFmt.format(new Date(value)) : "—";
}

export function dateTime(value: string | undefined): string {
  return value ? dateTimeFmt.format(new Date(value)) : "—";
}

export function age(value: string): string {
  const minutes = Math.max(0, Math.round((Date.now() - new Date(value).getTime()) / 60000));
  if (minutes < 60) return `${minutes} мин`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours} ч`;
  return `${Math.round(hours / 24)} дн`;
}

export function timeLeft(value: string): string {
  const ms = new Date(value).getTime() - Date.now();
  if (ms <= 0) return "срок истёк";
  const hours = Math.floor(ms / 3_600_000);
  if (hours >= 24) return `ещё ${Math.floor(hours / 24)} дн ${hours % 24} ч`;
  return `ещё ${hours} ч ${Math.floor((ms % 3_600_000) / 60000)} мин`;
}

// Indicative annuity payment for display only; the bank's contract is the source of truth.
export function monthlyPayment(amount: Money, termMonths: number, rateBps: number): Money {
  const principal = amount.amount_kopecks;
  const r = rateBps / 10000 / 12;
  const payment = r === 0 ? principal / termMonths : (principal * r) / (1 - Math.pow(1 + r, -termMonths));
  return rub(Math.round(payment));
}

export function shortId(id: string): string {
  return id.slice(0, 8);
}

// caseNumber is the human case number: the UUID tail, which also tells apart the seeded demo
// cases 10000000-…-000000000001, …0002 that share one prefix.
export function caseNumber(id: string): string {
  return id.replace(/-/g, "").slice(-8);
}
