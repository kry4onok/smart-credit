import { useState, type CSSProperties, type FormEvent } from "react";
import { Navigate, useLocation, useNavigate } from "react-router-dom";
import { isMock } from "../api/http";
import { ApiError, errorMessage } from "../api/problem";
import { homeFor, useSession } from "../auth/session";
import { Field, useTitle } from "../components/ui";

const heroStations = ["Заявка", "Проверка", "Решение", "Деньги"];

const heroPoints = [
  "Подбор кредита под задачу с примерным платежом",
  "Данные из реестров и бюро кредитных историй банк получит сам",
  "Видно, на каком этапе заявка и что сделать дальше",
];

const demoClients = [
  { email: "bakery@demo.local", who: "Северная пекарня", outcome: "автоматическое предложение, профиль не заполнен" },
  { email: "warehouse@demo.local", who: "Балтийский склад", outcome: "заявка на ручном разборе у аналитика" },
  { email: "service@demo.local", who: "Сибирский сервис", outcome: "банк запросит данные" },
  { email: "workshop@demo.local", who: "Уральская мастерская", outcome: "черновик сверх лимита, будет контрпредложение" },
  { email: "closing@demo.local", who: "Закрытие", outcome: "отказ: компания ликвидируется" },
];

// Accounts created by `make auth-dev-seed`; their passwords live in the .env (AUTH_DEV_*).
// Each client persona has its own company because a profile INN cannot be changed.
const seedAccounts = [
  { email: "client@smart-credit.test", who: "Клиент: ООО «Северная пекарня» — варианты и выдача" },
  { email: "ip@smart-credit.test", who: "Клиент: ИП на патенте — банк запросит патент" },
  { email: "closing@smart-credit.test", who: "Клиент: ликвидируемое ООО — отказ" },
  { email: "analyst@smart-credit.test", who: "Кредитный аналитик" },
  { email: "admin@smart-credit.test", who: "Администратор" },
];

const demoStaff = [{ email: "analyst@demo.local", who: "Анна Аналитик", outcome: "очередь дел и ручные решения" }];

export function LoginPage() {
  const { user, login } = useSession();
  const navigate = useNavigate();
  const from = (useLocation().state as { from?: string } | null)?.from;
  useTitle("Вход");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  if (user) return <Navigate to={homeFor(user)} replace />;

  async function signIn(address: string, secret: string) {
    setBusy(true);
    setError(null);
    try {
      const me = await login(address.trim(), secret);
      // Return to the page that asked for a session, if it belongs to this role.
      const home = homeFor(me);
      navigate(from && from.startsWith(home) ? from : home, { replace: true });
    } catch (err) {
      // Auth answers every rejected login with the same 401, so the form names the likely cause.
      setError(err instanceof ApiError && err.status === 401 ? "Неверная почта или пароль." : errorMessage(err));
    } finally {
      setBusy(false);
    }
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    void signIn(email, password);
  }

  const demo = (list: typeof demoClients, title: string) => (
    <div className="demo-group">
      <h3>{title}</h3>
      <ul>
        {list.map((a) => (
          <li key={a.email}>
            <button
              type="button"
              disabled={busy}
              onClick={() => {
                setEmail(a.email);
                setPassword("demo");
                void signIn(a.email, "demo");
              }}
            >
              <span className="demo-avatar" aria-hidden>
                {a.who[0]}
              </span>
              <span>
                <strong>{a.who}</strong>
                <span>{a.outcome}</span>
              </span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  );

  return (
    <div className="login">
      <section className="login-hero">
        <div className="login-brand">
          <img src="/favicon.svg" alt="" width={36} height={36} />
          <span>
            Кредитный кабинет
            <small>для малого бизнеса</small>
          </span>
        </div>
        <h1>Кредит для бизнеса — от задачи до денег на счёте</h1>
        <p className="login-lead">Заполните заявку один раз и следите за ней, как за посылкой: каждый этап виден, следующий шаг подсказан.</p>

        <div className="hero-card" aria-hidden="true">
          <div className="hero-card-top">
            <span>Пример заявки</span>
            <span className="hero-pill">
              <span className="pill-before">На рассмотрении</span>
              <span className="pill-after">Деньги выданы</span>
            </span>
          </div>
          <strong className="amount">3 000 000 ₽</strong>
          <span className="hero-card-meta">Кредит на развитие, 24 месяца</span>
          <ol className="hero-track">
            {heroStations.map((label, i) => (
              <li key={label} style={{ "--i": i } as CSSProperties}>
                <span className="hero-dot" />
                {label}
              </li>
            ))}
          </ol>
        </div>

        <ul className="hero-points">
          {heroPoints.map((p) => (
            <li key={p}>{p}</li>
          ))}
        </ul>
      </section>

      <section className="login-side">
        <form className="login-form" onSubmit={submit} noValidate>
          <h2>Вход в кабинет</h2>
          <p className="muted">Для представителей компаний и сотрудников банка</p>
          <Field label="Электронная почта" htmlFor="email">
            <input id="email" type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} required />
          </Field>
          <Field label="Пароль" htmlFor="password">
            <input id="password" type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} required />
          </Field>
          {error && (
            <p className="note note-danger" role="alert">
              {error}
            </p>
          )}
          <button className="btn btn-primary btn-block" disabled={busy || !email || !password}>
            {busy ? "Входим…" : "Войти"}
          </button>
        </form>
        {!isMock && (
          <div className="demo-accounts">
            <p>
              Демо-учётки стенда. Пароли указаны в README проекта; при локальном запуске — в <code>.env</code> (<code>AUTH_DEV_*</code>).
            </p>
            <div className="demo-group">
              <ul>
                {seedAccounts.map((a) => (
                  <li key={a.email}>
                    <button
                      type="button"
                      onClick={() => {
                        setEmail(a.email);
                        document.getElementById("password")?.focus();
                      }}
                    >
                      <span className="demo-avatar" aria-hidden>
                        {a.who[0]}
                      </span>
                      <span>
                        <strong>{a.who}</strong>
                        <span>{a.email}</span>
                      </span>
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        )}
        {isMock && (
          <div className="demo-accounts">
            <p>
              Демо-режим: войдите одним нажатием. Пароль у всех учёток — <code>demo</code>.
            </p>
            {demo(demoClients, "Компании")}
            {demo(demoStaff, "Банк")}
          </div>
        )}
      </section>
    </div>
  );
}
