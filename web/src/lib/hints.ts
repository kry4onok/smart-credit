// Early hints for the draft: what in the entered numbers may make the bank lower the amount or
// send the application to a specialist. They use only the client's own figures and the thresholds
// published in docs/ASSESSMENT_V2.md. The bank's assessment remains the only decision.

export type Level = "good" | "warn" | "bad" | "info";

export type Hint = {
  id: string;
  level: Level;
  title: string;
  text: string;
  // Position on a 0..1 scale for the indicator bar, when the hint has a measurable value.
  meter?: { value: number; good: number; label: string };
  fix?: { label: string; amountKopecks?: number; termMonths?: number };
};

export type HintInput = {
  amountKopecks: number | undefined;
  termMonths: number | undefined;
  rateBps: number | undefined;
  profitKopecks: number | undefined;
  revenueKopecks: number | undefined;
  obligationsKopecks: number | undefined;
  periodMonths: number;
  registeredAt: string | undefined;
  maxPaymentKopecks?: number | null;
  minTerm?: number;
  maxTerm?: number;
  minAmountKopecks?: number;
  today?: Date;
};

// Preliminary profit comparison only; the bank separately evaluates cash flow, existing debt and stress.
export const COVERAGE_MIN = 1.3;
const ROUND_TO = 100_000_00; // round suggestions down to 100 000 ₽

export function annuity(amountKopecks: number, termMonths: number, rateBps: number): number {
  const r = rateBps / 120000;
  if (r === 0) return amountKopecks / termMonths;
  return (amountKopecks * r) / (1 - Math.pow(1 + r, -termMonths));
}

// The largest amount whose annuity payment fits into `payment`.
export function principalFor(paymentKopecks: number, termMonths: number, rateBps: number): number {
  const r = rateBps / 120000;
  if (r === 0) return paymentKopecks * termMonths;
  return (paymentKopecks * (1 - Math.pow(1 + r, -termMonths))) / r;
}

export function monthsBetween(from: string, to: Date): number {
  const d = new Date(from);
  return (to.getUTCFullYear() - d.getUTCFullYear()) * 12 + (to.getUTCMonth() - d.getUTCMonth()) - (to.getUTCDate() < d.getUTCDate() ? 1 : 0);
}

const rubles = (k: number) => `${Math.round(k / 100).toLocaleString("ru-RU")} ₽`;
const roundDown = (k: number) => Math.floor(k / ROUND_TO) * ROUND_TO;

export function loanHints(i: HintInput): Hint[] {
  const hints: Hint[] = [];
  const today = i.today ?? new Date();

  if (i.registeredAt) {
    const age = monthsBetween(i.registeredAt, today);
    if (age < 12) {
      hints.push({
        id: "age",
        level: "bad",
        title: "Бизнесу меньше года",
        text: `С регистрации прошло ${Math.max(age, 0)} мес. По политике банка нужно не меньше 12 месяцев деятельности — заявку, скорее всего, отклонят.`,
      });
    } else {
      hints.push({ id: "age", level: "good", title: "Бизнес работает больше года", text: "Срок деятельности подходит под требования банка." });
    }
  }

  if (i.profitKopecks === undefined || i.revenueKopecks === undefined) {
    hints.push({
      id: "financials",
      level: "info",
      title: "Нет данных о выручке и прибыли",
      text: "Укажите их в сведениях о бизнесе — кабинет подскажет комфортную сумму. Без них банк возьмёт данные из выписки или попросит документы.",
    });
  }

  if (i.profitKopecks !== undefined && i.profitKopecks <= 0) {
    hints.push({
      id: "loss",
      level: "bad",
      title: "Бизнес не показывает прибыль",
      text: "Заявленная прибыль не положительная. Для решения банк отдельно проверит денежный поток и действующие долги.",
    });
  }

  const ready = i.amountKopecks && i.termMonths && i.rateBps !== undefined;
  if (ready && i.profitKopecks !== undefined && i.profitKopecks > 0) {
    const payment = annuity(i.amountKopecks!, i.termMonths!, i.rateBps!);
    const monthlyFlow = i.profitKopecks / Math.max(i.periodMonths, 1);
    const coverage = monthlyFlow / payment;
    const level: Level = coverage >= COVERAGE_MIN ? "good" : coverage >= 1 ? "warn" : "bad";
    const meter = { value: Math.min(coverage / 3, 1), good: COVERAGE_MIN / 3, label: `×${coverage.toLocaleString("ru-RU", { maximumFractionDigits: 2 })}` };
    if (level === "good") {
      hints.push({
        id: "coverage",
        level,
        title: "Платёж ниже заявленной прибыли",
        text: `Ежемесячная прибыль ≈ ${rubles(monthlyFlow)} покрывает платёж ≈ ${rubles(payment)} с запасом ×${coverage.toLocaleString("ru-RU", { maximumFractionDigits: 1 })}. Это предварительное сравнение: банк проверит денежный поток, действующие долги и стрессовый сценарий.`,
        meter,
      });
    } else {
      const comfortable = roundDown(principalFor(monthlyFlow / COVERAGE_MIN, i.termMonths!, i.rateBps!));
      let longer: number | undefined;
      for (let n = i.termMonths! + 1; n <= (i.maxTerm ?? 60); n++) {
        if (monthlyFlow / annuity(i.amountKopecks!, n, i.rateBps!) >= COVERAGE_MIN) {
          longer = n;
          break;
        }
      }
      const options = [
        comfortable >= (i.minAmountKopecks ?? 0) && comfortable > 0 ? `сумма до ${rubles(comfortable)}` : null,
        longer ? `срок ${longer} мес.` : null,
      ].filter(Boolean);
      hints.push({
        id: "coverage",
        level,
        title: level === "bad" ? "Платёж больше прибыли" : "Платёж почти без запаса",
        text:
          `Платёж ≈ ${rubles(payment)} в месяц, а прибыль ≈ ${rubles(monthlyFlow)}. Для ориентира используем запас ×1,3 по прибыли; банковский расчёт выполняется по денежному потоку с учётом других долгов и стресса. Меньшую сумму банк предложит, только если указать минимальную сумму в разделе «Если всю сумму дать не получится»; иначе уменьшите сумму или увеличьте срок сами.` +
          (options.length ? ` Для предварительного сравнения: ${options.join(" или ")}.` : ""),
        meter,
        fix:
          comfortable > 0 && comfortable >= (i.minAmountKopecks ?? 0)
            ? { label: `Взять ${rubles(comfortable)}`, amountKopecks: comfortable }
            : longer
              ? { label: `Срок ${longer} мес.`, termMonths: longer }
              : undefined,
      });
    }
    if (i.maxPaymentKopecks && payment > i.maxPaymentKopecks) {
      hints.push({
        id: "preferred-payment",
        level: "warn",
        title: "Платёж выше желаемого",
        text: `Вы указали комфортный платёж до ${rubles(i.maxPaymentKopecks)}, а при этих условиях он ≈ ${rubles(payment)}. Увеличьте срок или уменьшите сумму.`,
      });
    }
  }

  if (i.amountKopecks && i.revenueKopecks && i.revenueKopecks > 0) {
    const share = i.amountKopecks / ((i.revenueKopecks * 12) / Math.max(i.periodMonths, 1));
    if (share > 0.5) {
      hints.push({
        id: "revenue",
        level: "warn",
        title: "Сумма — больше половины годовой выручки",
        text: `Кредит составляет ${Math.round(share * 100)}% годовой выручки. Такой запрос банк обычно рассматривает вручную.`,
        meter: { value: Math.min(share, 1), good: 0.5, label: `${Math.round(share * 100)}%` },
      });
    }
  }

  if (i.obligationsKopecks && i.revenueKopecks && i.obligationsKopecks > i.revenueKopecks / 2) {
    hints.push({
      id: "debt",
      level: "warn",
      title: "Высокая текущая долговая нагрузка",
      text: "Текущие кредиты больше половины выручки за период. Банк учтёт их платежи при расчёте доступной суммы.",
    });
  }

  const order: Record<Level, number> = { bad: 0, warn: 1, info: 2, good: 3 };
  return hints.sort((a, b) => order[a.level] - order[b.level]);
}
