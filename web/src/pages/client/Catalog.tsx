import { useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { clientApi } from "../../api/http";
import { ApiError, call } from "../../api/problem";
import type { Product, PurposeTag } from "../../api/types";
import { MoneyInput } from "../../components/inputs";
import { PurposeIcon } from "../../components/icons";
import { ErrorNote, Field, Loading, useAction, useTitle } from "../../components/ui";
import { money, months, moneyShort, monthlyPayment, rate, rub } from "../../lib/format";
import { documentLabel, purposeHint, purposeLabel } from "../../lib/labels";
import { useIntents } from "../../lib/util";
import { createDraft, misfit, prefillFromParams, prefillToParams, type Prefill } from "./products";

const purposes: PurposeTag[] = ["WORKING_CAPITAL", "EQUIPMENT", "EXPANSION"];

export function CatalogPage() {
  const [params, setParams] = useSearchParams();
  const initial = prefillFromParams(params);
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

  const ranked = products
    ?.map((p) => ({ p, why: misfit(p, prefill.amount, term) }))
    .sort((a, b) => Number(!!a.why) - Number(!!b.why));
  const fitting = ranked?.filter((r) => !r.why).length ?? 0;

  return (
    <>
      <section className="purpose-pick">
        <h1>На что нужны деньги?</h1>
        <div className="purpose-options" role="group" aria-label="Цель кредита">
          {purposes.map((p) => (
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
            {ranked && (prefill.amount || term)
              ? fitting
                ? `Подходит ${fitting === 1 ? "один продукт" : fitting < 5 ? `${fitting} продукта` : `${fitting} продуктов`}`
                : "Под эти условия продуктов нет — измените сумму или срок"
              : "Укажите сумму и срок — покажем подходящие продукты и платёж"}
          </p>
        </div>
      </section>

      <ErrorNote error={error ?? create.error} />
      {!products && !error && <Loading label="Подбираем продукты…" />}
      {products && products.length === 0 && (
        <div className="empty panel">
          <p>Под эту цель сейчас нет продуктов. Выберите другую цель или посмотрите все предложения.</p>
          <button type="button" className="btn btn-secondary" onClick={() => setParams(prefillToParams({ ...prefill, purpose: undefined }))}>
            Показать все
          </button>
        </div>
      )}
      <div className="products">
        {ranked?.map(({ p, why }) => {
          const estimate = !why && prefill.amount && term && p.indicative_rate_bps !== undefined ? monthlyPayment(rub(prefill.amount), term, p.indicative_rate_bps) : null;
          return (
            <article key={p.id} className={`product panel${why ? " product-off" : ""}`}>
              <header>
                <h2>{p.name}</h2>
                <p>{p.description}</p>
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
              <p className="product-docs">
                {p.required_document_types.length
                  ? `Понадобится: ${p.required_document_types.map((d) => documentLabel[d].toLowerCase()).join(", ")}`
                  : "Документы банк подберёт по форме бизнеса и налоговому режиму"}
              </p>
              <footer>
                <button type="button" className={`btn ${why ? "btn-secondary" : "btn-primary"}`} disabled={create.busy} onClick={() => void apply(p)}>
                  Оформить заявку
                </button>
                <small>Ставка и платёж предварительные: точные условия банк предложит после проверки.</small>
              </footer>
            </article>
          );
        })}
      </div>
    </>
  );
}
