import { useEffect, useState, type FormEvent } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { clientApi, isMock } from "../../api/http";
import { ApiError, call } from "../../api/problem";
import type { CompanyProfile, LegalForm, TaxRegime } from "../../api/types";
import { useSession } from "../../auth/session";
import { MoneyInput } from "../../components/inputs";
import { Choices, ErrorNote, Field, Loading, useAction, useTitle } from "../../components/ui";
import { demoCompany, demoProfile } from "../../lib/demoCompanies";
import { rub } from "../../lib/format";
import { registryLinks } from "../../lib/external";
import { financialBasisLabel, legalFormLabel, taxRegimeLabel, taxRegimesFor, vatStatusLabel } from "../../lib/labels";
import { etag, isValidInn, useIntent } from "../../lib/util";
import { createDraft, loadProducts, prefillFromParams } from "./products";

type Basis = NonNullable<CompanyProfile["financial_basis"]>;
type Vat = "UNKNOWN" | "EXEMPT" | "PAYER";

type Draft = {
  legal_form: LegalForm | undefined;
  inn: string;
  name: string;
  okved: string;
  registered_at: string;
  regimes: TaxRegime[];
  regime_since: string;
  vat_status: Vat;
  financial_basis: Basis;
  period_from: string;
  period_to: string;
  obligations_as_of: string;
  revenue: number | null | undefined;
  profit: number | null | undefined;
  obligations: number | null | undefined;
};

type Errors = Partial<Record<keyof Draft, string>>;

const today = () => new Date().toISOString().slice(0, 10);

// The last 12 complete calendar months: the window the bank uses for cash-flow assessment.
function lastFullYear(): { from: string; to: string } {
  const now = new Date();
  const end = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 0));
  const start = new Date(Date.UTC(end.getUTCFullYear(), end.getUTCMonth() - 11, 1));
  return { from: start.toISOString().slice(0, 10), to: end.toISOString().slice(0, 10) };
}

// INN length fixes the legal form: 10 digits for a company, 12 for a sole proprietor.
export function formForInn(inn: string): LegalForm | undefined {
  return inn.length === 10 ? "LLC" : inn.length === 12 ? "IP" : undefined;
}

function fromProfile(p: CompanyProfile): Draft {
  const period = p.financial_period ?? lastFullYear();
  const tax = p.tax_periods?.[p.tax_periods.length - 1];
  return {
    legal_form: p.legal_form ?? formForInn(p.inn),
    inn: p.inn,
    name: p.name,
    okved: p.okved,
    registered_at: p.registered_at,
    regimes: tax?.regimes ?? [],
    regime_since: tax?.from ?? p.registered_at,
    vat_status: (tax?.vat_status as Vat | undefined) ?? "UNKNOWN",
    financial_basis: p.financial_basis ?? "MANAGEMENT",
    period_from: period.from,
    period_to: period.to,
    obligations_as_of: p.obligations_as_of ?? today(),
    revenue: p.revenue_12m?.amount_kopecks,
    profit: p.profit_12m?.amount_kopecks,
    obligations: p.obligations?.amount_kopecks,
  };
}

function validate(d: Draft): Errors {
  const e: Errors = {};
  if (!isValidInn(d.inn)) e.inn = "ИНН не прошёл проверку контрольной суммы.";
  else if (d.legal_form && formForInn(d.inn) !== d.legal_form)
    e.inn = d.legal_form === "IP" ? "У предпринимателя ИНН из 12 цифр." : "У ООО ИНН из 10 цифр.";
  if (!d.legal_form) e.legal_form = "Выберите форму бизнеса.";
  if (!d.name.trim()) e.name = d.legal_form === "IP" ? "Укажите ФИО предпринимателя, как в ЕГРИП." : "Укажите название, как в ЕГРЮЛ.";
  if (!/^\d{2}(\.\d{1,2}){0,2}$/.test(d.okved.trim())) e.okved = "Код вида деятельности в формате 10.71 или 47.";
  if (!d.registered_at) e.registered_at = "Укажите дату регистрации.";
  else if (d.registered_at > today()) e.registered_at = "Дата регистрации не может быть в будущем.";
  if (!d.regimes.length) e.regimes = "Отметьте хотя бы один налоговый режим или «Не знаю».";
  if (!d.regime_since) e.regime_since = "Укажите, с какой даты действует режим.";
  else if (d.regime_since > today()) e.regime_since = "Дата не может быть в будущем.";
  if (!d.period_from || !d.period_to || d.period_from >= d.period_to) e.period_to = "Конец периода должен быть позже начала.";
  if (!d.obligations_as_of || d.obligations_as_of > today()) e.obligations_as_of = "Укажите дату не позже сегодняшней.";
  if (d.revenue === null) e.revenue = "Введите сумму числом или оставьте поле пустым.";
  if (d.profit === null) e.profit = "Введите сумму числом или оставьте поле пустым.";
  if (d.obligations === null) e.obligations = "Введите сумму числом или оставьте поле пустым.";
  return e;
}

function toProfile(d: Draft): CompanyProfile {
  return {
    inn: d.inn,
    name: d.name.trim(),
    okved: d.okved.trim(),
    registered_at: d.registered_at,
    profile_schema_version: 2,
    legal_form: d.legal_form,
    tax_periods: [{ from: d.regime_since, regimes: d.regimes, vat_status: d.vat_status }],
    financial_period: { from: d.period_from, to: d.period_to },
    financial_basis: d.financial_basis,
    obligations_as_of: d.obligations_as_of,
    // Unknown amounts stay absent: the bank then uses its sources or asks for documents.
    ...(d.revenue != null ? { revenue_12m: rub(d.revenue) } : {}),
    ...(d.profit != null ? { profit_12m: rub(d.profit) } : {}),
    ...(d.obligations != null ? { obligations: rub(d.obligations) } : {}),
  };
}

const formCopy: Record<LegalForm, { hint: string; namePlaceholder: string; nameLabel: string; registered: string; note: string }> = {
  IP: {
    hint: "ИНН из 12 цифр, учёт по налоговому режиму, баланс не нужен",
    namePlaceholder: "ИП Иванов Иван Иванович",
    nameLabel: "ФИО предпринимателя",
    registered: "Дата регистрации ИП",
    note: "Для ИП банк смотрит на налоговый режим: при патенте попросит патент, при УСН — декларацию или книгу учёта доходов.",
  },
  LLC: {
    hint: "ИНН из 10 цифр, бухгалтерская отчётность, проверка руководителя и учредителей",
    namePlaceholder: "ООО «Северная пекарня»",
    nameLabel: "Название компании",
    registered: "Дата регистрации компании",
    note: "Для ООО банк дополнительно проверяет руководителя и владельцев, а годовую отчётность может взять из ГИР БО.",
  },
};

export function CompanyPage() {
  const { user } = useSession();
  const [params] = useSearchParams();
  const productId = params.get("product");
  const navigate = useNavigate();
  const [draft, setDraft] = useState<Draft | null>(null);
  const [version, setVersion] = useState<number | null>(null);
  const [errors, setErrors] = useState<Errors>({});
  const [loadError, setLoadError] = useState<unknown>(null);
  const [saved, setSaved] = useState(false);
  const save = useAction();
  const draftIntent = useIntent();
  useTitle("Сведения о бизнесе");

  useEffect(() => {
    call(clientApi.GET("/client/company"))
      .then((c) => {
        setDraft(fromProfile(c.profile));
        setVersion(c.version);
      })
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) {
          const org = user?.organization;
          const period = lastFullYear();
          const inn = org?.inn ?? "";
          setDraft({
            legal_form: formForInn(inn),
            inn,
            name: org?.name ?? "",
            okved: "",
            registered_at: "",
            regimes: [],
            regime_since: "",
            vat_status: "UNKNOWN",
            financial_basis: "MANAGEMENT",
            period_from: period.from,
            period_to: period.to,
            obligations_as_of: today(),
            revenue: undefined,
            profit: undefined,
            obligations: undefined,
          });
        } else setLoadError(e);
      });
  }, [user]);

  if (loadError) return <ErrorNote error={loadError} />;
  if (!draft) return <Loading />;

  const set = <K extends keyof Draft>(key: K, value: Draft[K]) => {
    setDraft((d) => (d ? { ...d, [key]: value } : d));
    setSaved(false);
    if (errors[key]) setErrors({ ...errors, [key]: undefined });
  };

  // The INN of a saved profile cannot change, and it decides which legal form is possible.
  const scenario = isMock ? undefined : demoCompany(user?.organization?.inn);
  const innLocked = version !== null;
  const innForm = formForInn(draft.inn);
  const form = draft.legal_form;
  const copy = form ? formCopy[form] : null;

  function chooseForm(next: LegalForm) {
    setDraft((d) => (d ? { ...d, legal_form: next, regimes: d.regimes.filter((r) => taxRegimesFor[next].includes(r)) } : d));
    setErrors((e) => ({ ...e, legal_form: undefined }));
  }

  function toggleRegime(regime: TaxRegime) {
    const has = draft!.regimes.includes(regime);
    // «Не знаю» excludes concrete regimes and the other way round.
    const next = has ? draft!.regimes.filter((r) => r !== regime) : regime === "UNKNOWN" ? ["UNKNOWN" as TaxRegime] : [...draft!.regimes.filter((r) => r !== "UNKNOWN"), regime];
    set("regimes", next);
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!draft) return;
    const found = validate(draft);
    setErrors(found);
    if (Object.keys(found).length) return;
    const result = await save.run(async () => {
      const company = await call(
        clientApi.PUT("/client/company", { params: { header: version ? { "If-Match": etag(version) } : {} }, body: toProfile(draft) }),
      );
      setVersion(company.version);
      if (!productId) return null;
      const product = (await loadProducts()).find((p) => p.id === productId);
      if (!product) throw new ApiError(404, "Продукт больше не доступен. Выберите другой в каталоге.");
      return draftIntent.send((key) => createDraft(product, prefillFromParams(params), key));
    });
    if (result) navigate(`/cabinet/applications/${result.id}`);
    else if (result === null) setSaved(true);
  }

  return (
    <form className="narrow" onSubmit={submit} noValidate>
      <div className="page-head">
        <h1>Сведения о бизнесе</h1>
      </div>
      <p className="lead">
        Заполните один раз — данные подставятся в каждую новую заявку. От формы бизнеса зависит, какие документы попросит банк.
      </p>

      {scenario && version === null && (
        <section className="panel">
          <h2>Данные компании</h2>
          <p>Можно подставить сведения о {scenario.name}. Проверьте их перед сохранением.</p>
          {version === null && (
            <button type="button" className="btn btn-secondary" onClick={() => {
              setDraft(fromProfile(demoProfile(scenario)));
              setErrors({});
              setSaved(false);
            }}>Подставить данные компании</button>
          )}
        </section>
      )}

      <fieldset className="panel">
        <legend>Форма бизнеса</legend>
        <Choices
          name="legal_form"
          className="form-choice"
          value={form}
          onChange={(v) => chooseForm(v)}
          options={(["IP", "LLC"] as LegalForm[]).map((f) => ({
            value: f,
            label: legalFormLabel[f],
            hint: innForm && innForm !== f ? "Не подходит к ИНН вашей организации" : formCopy[f].hint,
          }))}
          optionClassName={(v, on) => `form-option${on ? " on" : ""}${innForm && innForm !== v ? " form-option-off" : ""}`}
        />
        {errors.legal_form && <p className="field-error">{errors.legal_form}</p>}
        {copy && <p className="field-hint form-note">{copy.note}</p>}
      </fieldset>

      {form && copy && (
        <>
          <fieldset className="panel form-grid">
            <legend>Реквизиты</legend>
            <Field label="ИНН" htmlFor="inn" error={errors.inn} hint={innLocked ? "ИНН сохранённого профиля не меняется" : form === "IP" ? "12 цифр" : "10 цифр"}>
              <input
                id="inn"
                inputMode="numeric"
                maxLength={form === "IP" ? 12 : 10}
                value={draft.inn}
                readOnly={innLocked}
                onChange={(e) => set("inn", e.target.value.replace(/\D/g, ""))}
                aria-invalid={!!errors.inn}
              />
            </Field>
            <Field label={copy.nameLabel} htmlFor="name" error={errors.name}>
              <input id="name" placeholder={copy.namePlaceholder} value={draft.name} onChange={(e) => set("name", e.target.value)} aria-invalid={!!errors.name} />
            </Field>
            <Field label="Основной ОКВЭД" htmlFor="okved" error={errors.okved} hint="Например, 10.71 — производство хлеба">
              <input id="okved" value={draft.okved} onChange={(e) => set("okved", e.target.value)} aria-invalid={!!errors.okved} />
            </Field>
            <Field label={copy.registered} htmlFor="registered_at" error={errors.registered_at}>
              <input id="registered_at" type="date" max={today()} value={draft.registered_at} onChange={(e) => set("registered_at", e.target.value)} aria-invalid={!!errors.registered_at} />
            </Field>
            <p className="field-hint registry-links">
              Сверить реквизиты:{" "}
              {registryLinks.map((l, i) => (
                <span key={l.url}>
                  {i > 0 && ", "}
                  <a href={l.url} target="_blank" rel="noreferrer noopener">
                    {l.label}
                  </a>
                </span>
              ))}
            </p>
          </fieldset>

          <fieldset className="panel form-grid">
            <legend>Налоги</legend>
            <Field label="Налоговый режим сейчас" group error={errors.regimes} hint={form === "IP" ? "Можно отметить несколько, например УСН и патент" : undefined}>
              <div className="regimes">
                {taxRegimesFor[form].map((r) => (
                  <label key={r} className={`check regime${draft.regimes.includes(r as TaxRegime) ? " on" : ""}`}>
                    <input type="checkbox" checked={draft.regimes.includes(r as TaxRegime)} onChange={() => toggleRegime(r as TaxRegime)} />
                    {taxRegimeLabel[r]}
                  </label>
                ))}
              </div>
            </Field>
            <Field label="Режим действует с" htmlFor="regime_since" error={errors.regime_since}>
              <input id="regime_since" type="date" max={today()} value={draft.regime_since} onChange={(e) => set("regime_since", e.target.value)} aria-invalid={!!errors.regime_since} />
            </Field>
            <Field label="НДС" htmlFor="vat">
              <select id="vat" value={draft.vat_status} onChange={(e) => set("vat_status", e.target.value as Vat)}>
                {Object.entries(vatStatusLabel).map(([k, v]) => (
                  <option key={k} value={k}>
                    {v}
                  </option>
                ))}
              </select>
            </Field>
          </fieldset>

          <fieldset className="panel form-grid">
            <legend>Финансы</legend>
            <p className="field-hint form-grid-wide">
              Укажите, если знаете. Пустое поле не считается нулём: банк возьмёт данные из выписки или попросит документ. По этим цифрам кабинет заранее
              подскажет, какая сумма будет комфортной.
            </p>
            <Field label="Период данных: с" htmlFor="period_from">
              <input id="period_from" type="date" value={draft.period_from} onChange={(e) => set("period_from", e.target.value)} />
            </Field>
            <Field label="по" htmlFor="period_to" error={errors.period_to}>
              <input id="period_to" type="date" value={draft.period_to} onChange={(e) => set("period_to", e.target.value)} aria-invalid={!!errors.period_to} />
            </Field>
            <Field label="Откуда цифры" htmlFor="basis">
              <select id="basis" value={draft.financial_basis} onChange={(e) => set("financial_basis", e.target.value as Basis)}>
                {Object.entries(financialBasisLabel).map(([k, v]) => (
                  <option key={k} value={k}>
                    {v}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="Выручка за период" htmlFor="revenue" error={errors.revenue}>
              <MoneyInput id="revenue" value={draft.revenue ?? undefined} onChange={(v) => set("revenue", v)} invalid={!!errors.revenue} />
            </Field>
            <Field label="Чистая прибыль" htmlFor="profit" error={errors.profit} hint="Убыток укажите со знаком минус">
              <MoneyInput id="profit" allowNegative value={draft.profit ?? undefined} onChange={(v) => set("profit", v)} invalid={!!errors.profit} />
            </Field>
            <Field label="Кредиты и займы сейчас" htmlFor="obligations" error={errors.obligations} hint="Остаток долга по всем банкам">
              <MoneyInput id="obligations" value={draft.obligations ?? undefined} onChange={(v) => set("obligations", v)} invalid={!!errors.obligations} />
            </Field>
            <Field label="Долг указан на дату" htmlFor="obligations_as_of" error={errors.obligations_as_of}>
              <input id="obligations_as_of" type="date" max={today()} value={draft.obligations_as_of} onChange={(e) => set("obligations_as_of", e.target.value)} aria-invalid={!!errors.obligations_as_of} />
            </Field>
          </fieldset>
        </>
      )}

      <ErrorNote error={save.error} />
      <div className="form-actions">
        <button className="btn btn-primary" disabled={save.busy || !form}>
          {save.busy ? "Сохраняем…" : productId ? "Сохранить и перейти к заявке" : "Сохранить"}
        </button>
        {saved && <span className="saved">Сохранено</span>}
      </div>
    </form>
  );
}
