#!/usr/bin/env bash
set -euo pipefail

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
  -v auth_password="$AUTH_DB_PASSWORD" \
  -v client_password="$CLIENT_DB_PASSWORD" \
  -v bank_password="$BANK_DB_PASSWORD" <<'SQL'
CREATE ROLE auth_user LOGIN PASSWORD :'auth_password';
CREATE ROLE client_user LOGIN PASSWORD :'client_password';
CREATE ROLE bank_user LOGIN PASSWORD :'bank_password';

CREATE DATABASE auth_db OWNER auth_user;
CREATE DATABASE client_db OWNER client_user;
CREATE DATABASE bank_db OWNER bank_user;
SQL
