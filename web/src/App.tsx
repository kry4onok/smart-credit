import { lazy, type ReactNode } from "react";
import { Navigate, Route, Routes, useLocation } from "react-router-dom";
import type { CurrentUser } from "./api/types";
import { homeFor, useSession } from "./auth/session";
import { Layout } from "./components/Layout";
import { ErrorBoundary, Loading } from "./components/ui";
import { LoginPage } from "./pages/Login";

// Each role downloads only its own screens.
const QueuePage = lazy(() => import("./pages/bank/Queue").then((m) => ({ default: m.QueuePage })));
const CasePage = lazy(() => import("./pages/bank/Case").then((m) => ({ default: m.CasePage })));
const ApplicationsPage = lazy(() => import("./pages/client/Applications").then((m) => ({ default: m.ApplicationsPage })));
const ApplicationPage = lazy(() => import("./pages/client/Application").then((m) => ({ default: m.ApplicationPage })));
const CatalogPage = lazy(() => import("./pages/client/Catalog").then((m) => ({ default: m.CatalogPage })));
const CompanyPage = lazy(() => import("./pages/client/Company").then((m) => ({ default: m.CompanyPage })));

// The role only chooses the visible section; the backend enforces access on every request.
function RequireRole({ roles, children }: { roles: CurrentUser["role"][]; children: ReactNode }) {
  const { user, loading } = useSession();
  const location = useLocation();
  if (loading) return <Loading label="Проверяем сессию…" />;
  if (!user) return <Navigate to="/login" replace state={{ from: location.pathname }} />;
  if (!roles.includes(user.role)) return <Navigate to={homeFor(user)} replace />;
  return (
<ErrorBoundary>{children}</ErrorBoundary>
  );
}

function Home() {
  const { user, loading } = useSession();
  if (loading) return <Loading label="Проверяем сессию…" />;
  return <Navigate to={user ? homeFor(user) : "/login"} replace />;
}

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={<Home />} />
      <Route
        element={
          <RequireRole roles={["CLIENT"]}>
            <Layout />
          </RequireRole>
        }
      >
        <Route path="/cabinet" element={<ApplicationsPage />} />
        <Route path="/cabinet/products" element={<CatalogPage />} />
        <Route path="/cabinet/company" element={<CompanyPage />} />
        <Route path="/cabinet/applications/:id" element={<ApplicationPage />} />
      </Route>
      <Route
        element={
          <RequireRole roles={["ANALYST", "ADMIN"]}>
            <Layout />
          </RequireRole>
        }
      >
        <Route path="/bank" element={<QueuePage />} />
        <Route path="/bank/cases/:id" element={<CasePage />} />
      </Route>
      <Route path="*" element={<Home />} />
    </Routes>
  );
}
