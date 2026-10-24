import { useEffect, useState, type FormEvent } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { clientApi } from "../../api/http";
import { ApiError, call } from "../../api/problem";
import type { CompanyProfile } from "../../api/types";
import { useSession } from "../../auth/session";
import { MoneyInput } from "../../components/inputs";
import { ErrorNote, Field, Loading, useAction, useTitle } from "../../components/ui";
import { rub } from "../../lib/format";
import { etag, isValidInn, useIntent } from "../../lib/util";
import { createDraft, loadProducts, prefillFromParams } from "./products";

type Draft = {
  inn: string;
  name: string;
  okved: string;
  registered_at: string;
  revenue: number | null | undefined;
  profit: number | null | undefined;
  obligations: number | null | undefined;
};

type Errors = Partial<Record<keyof Draft, string>>;

function fromProfile(p: CompanyProfile): Draft {
  return {
    inn: p.inn,
    name: p.name,
    okved: p.okved,
    registered_at: p.registered_at,
    revenue: p.revenue_12m.amount_kopecks,
    profit: p.profit_12m.amount_kopecks,
    obligations: p.obligations.amount_kopecks,
  };
}

function validate(d: Draft): Errors {
  const e: Errors = {};
  if (!isValidInn(d.inn)) e.inn = "ИНН состоит из 10 цифр для компании или 12 для ИП и проходит проверку контрольной суммы.";
  if (!d.name.trim()) e.name = "Укажите название, как в выписке ЕГРЮЛ.";
  if (!/^\d{2}(\.\d{1,2}){0,2}$/.test(d.okved.trim())) e.okved = "Код вида деятельности в формате 10.71 или 47.";
  if (!d.registered_at) e.registered_at = "Укажите дату регистрации.";
  else if (new Date(d.registered_at) > new Date()) e.registered_at = "Дата регистрации не может быть в будущем.";
  if (d.revenue == null) e.revenue = "Укажите выручку в рублях, например 8 400 000.";
  if (d.profit == null) e.profit = "Укажите прибыль; убыток — со знаком минус.";
  if (d.obligations == null) e.obligations = "Укажите сумму текущих кредитов и займов, если их нет — 0.";
  return e;
}

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
  useTitle("Сведения о компании");

  useEffect(() => {
    call(clientApi.GET("/client/company"))
      .then((c) => {
        setDraft(fromProfile(c.profile));
        setVersion(c.version);
      })
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) {
          const org = user?.organization;
          setDraft({ inn: org?.inn ?? "", name: org?.name ?? "", okved: "", registered_at: "", revenue: undefined, profit: undefined, obligations: undefined });
        } else setLoadError(e);
      });
  }, [user]);

  if (loadError) return <ErrorNote error={loadError} />;
  if (!draft) return <Loading />;

  const set = <K extends keyof Draft>(key: K, value: Draft[K]) => {
    setDraft({ ...draft, [key]: value });
    setSaved(false);
    if (errors[key]) setErrors({ ...errors, [key]: undefined });
  };

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!draft) return;
    const found = validate(draft);
    setErrors(found);
    if (Object.keys(found).length) return;
    const profile: CompanyProfile = {
      inn: draft.inn,
      name: draft.name.trim(),
      okved: draft.okved.trim(),
      registered_at: draft.registered_at,
      revenue_12m: rub(draft.revenue!),
      profit_12m: rub(draft.profit!),
      obligations: rub(draft.obligations!),
    };
    const result = await save.run(async () => {
      const company = await call(
        clientApi.PUT("/client/company", { params: { header: version ? { "If-Match": etag(version) } : {} }, body: profile }),
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

  const inn = draft.inn;
  return (
    <form className="narrow" onSubmit={submit} noValidate>
      <div className="page-head">
        <h1>Сведения о компании</h1>
      </div>
      <p className="lead">
        Заполните один раз — данные подставятся в каждую новую заявку. Уже отправленные заявки сохраняют сведения на момент
        отправки.
      </p>

      <fieldset className="panel form-grid">
        <legend>Реквизиты</legend>
        <Field label="ИНН" htmlFor="inn" error={errors.inn} hint="10 цифр для ООО, 12 — для ИП">
          <input id="inn" inputMode="numeric" maxLength={12} value={inn} onChange={(e) => set("inn", e.target.value.replace(/\D/g, ""))} aria-invalid={!!errors.inn} />
        </Field>
        <Field label="Название" htmlFor="name" error={errors.name}>
          <input id="name" value={draft.name} onChange={(e) => set("name", e.target.value)} aria-invalid={!!errors.name} />
        </Field>
        <Field label="Основной ОКВЭД" htmlFor="okved" error={errors.okved} hint="Например, 10.71 — производство хлеба">
          <input id="okved" value={draft.okved} onChange={(e) => set("okved", e.target.value)} aria-invalid={!!errors.okved} />
        </Field>
        <Field label="Дата регистрации" htmlFor="registered_at" error={errors.registered_at}>
          <input id="registered_at" type="date" value={draft.registered_at} onChange={(e) => set("registered_at", e.target.value)} aria-invalid={!!errors.registered_at} />
        </Field>
      </fieldset>

      <fieldset className="panel form-grid">
        <legend>Финансы за последние 12 месяцев</legend>
        <Field label="Выручка" htmlFor="revenue" error={errors.revenue}>
          <MoneyInput id="revenue" value={draft.revenue ?? undefined} onChange={(v) => set("revenue", v)} invalid={!!errors.revenue} />
        </Field>
        <Field label="Чистая прибыль" htmlFor="profit" error={errors.profit} hint="Убыток укажите со знаком минус">
          <MoneyInput id="profit" allowNegative value={draft.profit ?? undefined} onChange={(v) => set("profit", v)} invalid={!!errors.profit} />
        </Field>
        <Field label="Текущие кредиты и займы" htmlFor="obligations" error={errors.obligations} hint="Остаток долга по всем банкам">
          <MoneyInput id="obligations" value={draft.obligations ?? undefined} onChange={(v) => set("obligations", v)} invalid={!!errors.obligations} />
        </Field>
      </fieldset>

      <ErrorNote error={save.error} />
      <div className="form-actions">
        <button className="btn btn-primary" disabled={save.busy}>
          {save.busy ? "Сохраняем…" : productId ? "Сохранить и перейти к заявке" : "Сохранить"}
        </button>
        {saved && <span className="saved">Сохранено</span>}
      </div>
    </form>
  );
}
