import { fileURLToPath } from "node:url";
import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

// In dev the browser calls the same origin under /api, as in production.
// The proxy forwards to the dev gateway (deploy/nginx/nginx.conf), which strips /api;
// its port comes from GATEWAY_PORT in the repository .env, like for `make gateway-up`.
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const root = loadEnv(mode, fileURLToPath(new URL("..", import.meta.url)), "");
  const target = env.WEB_API_PROXY ?? `http://127.0.0.1:${root.GATEWAY_PORT || "8000"}`;
  return {
    plugins: [react()],
    server: {
      port: 5173,
      proxy: env.VITE_API_MOCK === "true" ? undefined : { "/api": { target, changeOrigin: false } },
    },
  };
});
