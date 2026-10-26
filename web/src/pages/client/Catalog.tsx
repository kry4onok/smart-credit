import { useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useSession } from "../../auth/session";
import { demoCompany } from "../../lib/demoCompanies";
import { productDescription, productName } from "../../lib/presentation";
import { clientApi, isMock } from "../../api/http";
import { ApiError, call } from "../../api/problem";
import type { Product } from "../../api/types";
import { MoneyInput } from "../../components/inputs";
import { PurposeIcon } from "../../components/icons";
import { ErrorNote, Field, Loading, useAction, useTitle } from "../../components/ui";
import { money, months, moneyShort, monthlyPayment, rate, rub } from "../../lib/format";
import { documentLabel, purposeHint, purposeLabel, purposeOrder } from "../../lib/labels";
import { useIntents } from "../../lib/util";
import { createDraft, misfit, prefillFromParams, prefillToParams, type Prefill } from "./products";

export function CatalogPage() {
  const [params, setParams] = useSearchParams();
  const initial = prefillFromParams(params);
  const { user } = useSession();
  const scenario = isMock ? undefined : demoCompany(user?.organization?.inn);
  const purpose = initial.purpose ?? null;
  const [amount, setAmount] = useState<number | null | undefined>(initial.amount);
  const [termText, setTermText] = useState(initial.term ? String(initial.term) : "");
  const [products, setProducts] = useState<Product[] | null>(null);
  const [error, setError] = useState<unknown>(null);
  const create = useAction();
  const intentFor = useIntents();
  const navigate = useNavigate();
  useTitle("Подбор кредита");

  const term = /^\d{1,3}$/.test(termText) && Number(termText) > 0 ? Number(termText) : undefined;
  const prefill: Prefill = { amount: amount ?? undefined, term, purpose: purpose ?? undefined };

  useEffect(() => {
    setProducts(null);
    call(clientApi.GET("/client/products", { params: { query: purpose ? { purpose } : {} } }))
      .then((r) => {
        setProducts(r.items);
        setError(null);
      })
      .catch(setError);
  }, [purpose]);

  // Keep the request in the URL so «Назад» and a reload return to the same selection.
  useEffect(() => {
    const id = window.setTimeout(() => setParams(prefillToParams(prefill), { replace: true }), 400);
    return () => window.clearTimeout(id);
  }, [amount, term, purpose]); // eslint-disable-line react-hooks/exhaustive-deps

  async function apply(product: Product) {
    const created = await create.run(async () => {
      // The draft takes a snapshot of the company profile, so the profile must exist first.
      try {
        await call(clientApi.GET("/client/company"));
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) {
          navigate(`/cabinet/company?${new URLSearchParams({ product: product.id, ...prefillToParams(prefill) })}`);
          return null;
        }
        throw e;
      }
      return intentFor(product.id).send((key) => createDraft(product, prefill, key));
    });
    if (created) navigate(`/cabinet/applications/${created.id}`);
  }

  // The bank returns the whole catalog with products for the chosen purpose first; products that
  // do not fit stay visible below with the reason.
  const ranked = products
    ?.map((p) => ({ p, why: misfit(p, prefill.amount, term, purpose ?? undefined), wrongPurpose: !!purpose && !p.purpose_tags.includes(purpose) }))
    .sort((a, b) => Number(!!a.why) - Number(!!b.why));
  const fitting = ranked?.filter((r) => !r.why).length ?? 0;

  return (
    <>
      {scenario && (
        <section className="panel">
          <h2>Параметры запроса</h2>
          <p>Начните с суммы {money(rub(scenario.suggested_amount_kopecks))} на {months(scenario.suggested_term_months)} или укажите свои условия ниже.</p>
          {scenario.email === "workshop@smart-credit.test" && <p className="field-hint">Если готовы рассмотреть меньшую сумму, укажите её минимум в черновике, в разделе «Если всю сумму дать не получится».</p>}
          <button type="button" className="btn btn-secondary" onClick={() => {
            setAmount(scenario.suggested_amount_kopecks);
            setTermText(String(scenario.suggested_term_months));
            setParams(prefillToParams({ amount: scenario.suggested_amount_kopecks, term: scenario.suggested_term_months, purpose: "WORKING_CAPITAL" }));
          }}>Подставить параметры</button>
        </section>
      )}
      <section className="purpose-pick">
        <h1>На что нужны деньги?</h1>
        <div className="purpose-options" role="group" aria-label="Цель кредита">
          {purposeOrder.map((p) => (
            <button
              key={p}
              type="button"
              aria-pressed={purpose === p}
              className={`purpose${purpose === p ? " purpose-on" : ""}`}
              onClick={() => setParams(prefillToParams({ ...prefill, purpose: purpose === p ? undefined : p }), { replace: true })}
            >
              <PurposeIcon purpose={p} />
              <strong>{purposeLabel[p]}</strong>
              <span>{purposeHint[p]}</span>
            </button>
          ))}
        </div>
      </section>

      <section className="calc panel" aria-labelledby="calc-title">
        <h2 id="calc-title">Сколько и на какой срок</h2>
        <div className="calc-fields">
          <Field label="Сумма" htmlFor="calc-amount" error={amount === null ? "Введите сумму числом, например 1 500 000." : null}>
            <MoneyInput id="calc-amount" value={amount ?? undefined} onChange={setAmount} invalid={amount === null} />
          </Field>
          <Field label="Срок, месяцев" htmlFor="calc-term">
            <input id="calc-term" inputMode="numeric" value={termText} onChange={(e) => setTermText(e.target.value.replace(/\D/g, ""))} />
          </Field>
          <p className="calc-result" aria-live="polite">
            {ranked && (prefill.amount || term || purpose)
              ? fitting
                ? `Подходит ${fitting === 1 ? "один продукт" : fitting < 5 ? `${fitting} продукта` : `${fitting} продуктов`}`
                : purpose
                  ? "Под эту цель и условия продуктов нет — измените цель, сумму или срок"
                  : "Под эти условия продуктов нет — измените сумму или срок"
              : "Укажите сумму и срок — покажем подходящие продукты и платёж"}
          </p>
        </div>
      </section>

      <ErrorNote error={error ?? create.error} />
      {!products && !error && <Loading label="Подбираем продукты…" />}
      {products && products.length === 0 && (
        <div className="empty panel">
          <p>Банк сейчас не публикует продукты. Загляните чуть позже.</p>
        </div>
      )}
      <div className="products">
        {ranked?.map(({ p, why, wrongPurpose }) => {
          const estimate = !why && prefill.amount && term && p.indicative_rate_bps !== undefined ? monthlyPayment(rub(prefill.amount), term, p.indicative_rate_bps) : null;
          return (
            <article key={p.id} className={`product panel${why ? " product-off" : ""}`}>
              <header>
                <h2>{productName(p.name)}</h2>
                <p>{productDescription(p.code, p.description)}</p>
              </header>
              {why && <p className="product-why">Не подходит: {why.toLowerCase()}</p>}
              {estimate && (
                <p className="product-estimate">
                  <span className="amount">≈ {money(estimate)}</span> в месяц
                </p>
              )}
              <dl className="product-terms">
                <div>
                  <dt>Сумма</dt>
                  <dd>
                    {moneyShort(p.min_amount)} – {moneyShort(p.max_amount)}
                  </dd>
                </div>
                <div>
                  <dt>Срок</dt>
                  <dd>
                    {p.min_term_months}–{months(p.max_term_months)}
                  </dd>
                </div>
                {p.indicative_rate_bps !== undefined && (
                  <div>
                    <dt>Ставка</dt>
                    <dd>от {rate(p.indicative_rate_bps)}</dd>
                  </div>
                )}
              </dl>
              <p className="product-purposes">Цели: {purposeOrder.filter((t) => p.purpose_tags.includes(t)).map((t) => purposeLabel[t].toLowerCase()).join(", ")}</p>
              <p className="product-docs">
                {p.required_document_types.length
                  ? `Понадобится: ${p.required_document_types.map((d) => documentLabel[d].toLowerCase()).join(", ")}`
                  : "Документы банк подберёт по форме бизнеса и налоговому режиму"}
              </p>
              <footer>
                <button type="button" className={`btn ${why ? "btn-secondary" : "btn-primary"}`} disabled={create.busy || wrongPurpose} onClick={() => void apply(p)}>
                  Оформить заявку
                </button>
                <small>
                  {wrongPurpose
                    ? "Продукт выдаётся только на перечисленные цели. Выберите другую цель или другой продукт."
                    : "Ставка и платёж предварительные: точные условия банк предложит после проверки."}
                </small>
              </footer>
            </article>
          );
        })}
      </div>
    </>
  );
}
