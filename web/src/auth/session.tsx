import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { authApi, onSessionExpired, refreshAccessToken, setAccessToken } from "../api/http";
import { call } from "../api/problem";
import type { CurrentUser } from "../api/types";

type Session = {
  user: CurrentUser | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<CurrentUser>;
  logout: () => Promise<boolean>;
};

const SessionContext = createContext<Session | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<CurrentUser | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    onSessionExpired(() => setUser(null));
    // A page reload loses the in-memory access token; the refresh cookie restores it.
    refreshAccessToken()
      .then(async (ok) => (ok ? setUser(await call(authApi.GET("/auth/me"))) : setUser(null)))
      .catch(() => setUser(null))
      .finally(() => setLoading(false));
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    const token = await call(authApi.POST("/auth/login", { body: { email, password } }));
    setAccessToken(token.access_token);
    const me = await call(authApi.GET("/auth/me"));
    setUser(me);
    return me;
  }, []);

  // Only a confirmed logout clears the session: if the refresh cookie was not revoked,
  // reloading the page would silently sign the user back in.
  const logout = useCallback(async () => {
    try {
      await call(authApi.POST("/auth/logout"));
    } catch {
      return false;
    }
    setAccessToken(null);
    setUser(null);
    return true;
  }, []);

  const value = useMemo(() => ({ user, loading, login, logout }), [user, loading, login, logout]);
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession(): Session {
  const session = useContext(SessionContext);
  if (!session) throw new Error("useSession outside SessionProvider");
  return session;
}

export function homeFor(user: CurrentUser): string {
  return user.role === "CLIENT" ? "/cabinet" : "/bank";
}
