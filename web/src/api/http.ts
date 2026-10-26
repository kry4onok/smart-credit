import createClient from "openapi-fetch";
import type { paths as AuthPaths } from "./generated/auth";
import type { paths as ClientPaths } from "./generated/client";
import type { paths as BankPaths } from "./generated/bank";
import type { paths as AssessmentPaths } from "./generated/assessment";
import type { AccessToken } from "./types";

const MOCK = import.meta.env.VITE_API_MOCK === "true";

let mockFetch: Promise<(request: Request) => Promise<Response>> | null = null;

// The mock is imported lazily so a production build never contains it.
function transport(request: Request): Promise<Response> {
  if (MOCK) {
    mockFetch ??= import("../mock/server").then((m) => m.mockFetch);
    return mockFetch.then((handle) => handle(request));
  }
  return fetch(request);
}

// The access token lives only in memory; the refresh token is an HttpOnly cookie.
let accessToken: string | null = null;
let refreshing: Promise<boolean> | null = null;
let onSessionLost: (() => void) | null = null;

export function setAccessToken(token: string | null) {
  accessToken = token;
}

export function onSessionExpired(handler: () => void) {
  onSessionLost = handler;
}

export function refreshAccessToken(): Promise<boolean> {
  refreshing ??= transport(new Request("/api/auth/refresh", { method: "POST", credentials: "same-origin" }))
    .then(async (response) => {
      if (!response.ok) return false;
      const body = (await response.json()) as AccessToken;
      accessToken = body.access_token;
      return true;
    })
    .catch(() => false)
    .finally(() => {
      refreshing = null;
    });
  return refreshing;
}

function withToken(request: Request): Request {
  const next = new Request(request, { credentials: "same-origin" });
  if (accessToken) next.headers.set("Authorization", `Bearer ${accessToken}`);
  return next;
}

// One retry after refresh on 401, as required for every API client in the web app.
async function authFetch(request: Request): Promise<Response> {
  const retry = request.clone();
  const response = await transport(withToken(request));
  if (response.status !== 401 || new URL(request.url).pathname.startsWith("/api/auth/")) {
    return response;
  }
  if (!(await refreshAccessToken())) {
    accessToken = null;
    onSessionLost?.();
    return response;
  }
  return transport(withToken(retry));
}

const options = { baseUrl: "/api", fetch: authFetch };

export const authApi = createClient<AuthPaths>(options);
export const clientApi = createClient<ClientPaths>(options);
export const bankApi = createClient<BankPaths>(options);
export const assessmentApi = createClient<AssessmentPaths>(options);

export const isMock = MOCK;
