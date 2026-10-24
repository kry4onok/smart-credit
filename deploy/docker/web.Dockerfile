# Builds the React app and serves the static files with nginx; /api is proxied to the services.
FROM node:22.12.0-alpine AS build
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM nginx:1.30.6-alpine
COPY deploy/nginx/web.conf /etc/nginx/nginx.conf
COPY --from=build /src/web/dist /usr/share/nginx/html
EXPOSE 8080
