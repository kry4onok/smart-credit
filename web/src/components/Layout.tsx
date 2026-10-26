import { Suspense, useState } from "react";
import { NavLink, Outlet } from "react-router-dom";
import { useSession } from "../auth/session";
import { Loading } from "./ui";

const clientNav = [
  { to: "/cabinet", label: "Мои заявки", end: true },
  { to: "/cabinet/products", label: "Подобрать кредит" },
  { to: "/cabinet/company", label: "Компания" },
];

const bankNav = [{ to: "/bank", label: "Очередь дел", end: true }];

export function Layout() {
  const { user, logout } = useSession();
  const [logoutFailed, setLogoutFailed] = useState(false);
  const isClient = user?.role === "CLIENT";
  const nav = isClient ? clientNav : bankNav;
  return (
    <div className={isClient ? "app app-client" : "app app-bank"}>
      <a className="skip" href="#main">
        К содержимому
      </a>
      <header className="topbar">
        <div className="topbar-inner">
          <NavLink to="/" className="brand" aria-label="На главную">
            <img src="/favicon.svg" alt="" width={28} height={28} />
            <span>
              Кредитный кабинет
              <small>{isClient ? "для бизнеса" : "рабочее место аналитика"}</small>
            </span>
          </NavLink>
          <nav className="nav" aria-label="Разделы">
            {nav.map((item) => (
              <NavLink key={item.to} to={item.to} end={item.end}>
                {item.label}
              </NavLink>
            ))}
          </nav>
          <div className="whoami">
            <span>
              {user?.full_name}
              <small>{isClient ? user?.organization?.name : user?.role === "ADMIN" ? "Администратор" : "Кредитный аналитик"}</small>
            </span>
            <button type="button" className="btn btn-quiet btn-sm" onClick={() => void logout().then((ok) => setLogoutFailed(!ok))}>
              Выйти
            </button>
            {logoutFailed && (
              <span className="logout-error" role="alert">
                Не удалось выйти: нет связи с сервером
              </span>
            )}
          </div>
        </div>
      </header>
      <main className="page" id="main" tabIndex={-1}>
        <Suspense fallback={<Loading />}>
          <Outlet />
        </Suspense>
      </main>
    </div>
  );
}
