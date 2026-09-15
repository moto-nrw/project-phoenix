import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

// Env-Quellen für die Perf-Messung (#2938), in Gewinner-Reihenfolge: die erste
// Datei, die einen Schlüssel setzt, gewinnt. `../.env` (Compose-Root) füllt
// Lücken, weil frontend/.env.local lokal nicht alle Hostnames trägt, die
// proxy.ts beim Modul-Load verlangt. process.env wird NICHT befüllt: die
// Compose-Datei trägt z. B. PORT=8080, das darf `next dev`/`next start` nie sehen.
const ENV_FILES = [".env.local", ".env", "../.env"];

/** @returns {Map<string, string>} */
function readEnvFiles() {
  const values = new Map();
  for (const file of ENV_FILES) {
    const path = join(process.cwd(), file);
    if (!existsSync(path)) continue;
    for (const line of readFileSync(path, "utf8").split(/\r?\n/)) {
      const trimmed = line.trim();
      if (!trimmed || trimmed.startsWith("#")) continue;
      const separator = trimmed.indexOf("=");
      if (separator <= 0) continue;
      const key = trimmed.slice(0, separator).trim();
      let value = trimmed.slice(separator + 1).trim();
      if (
        (value.startsWith('"') && value.endsWith('"')) ||
        (value.startsWith("'") && value.endsWith("'"))
      ) {
        value = value.slice(1, -1);
      }
      if (!values.has(key)) values.set(key, value);
    }
  }
  return values;
}

/** @param {Map<string, string>} values @param {string} name */
function requiredEnv(values, name) {
  const value = process.env[name] ?? values.get(name);
  if (!value) throw new Error(`${name} is required for the perf harness.`);
  return value;
}

/** Alles, was `next build`/`next start`/`next dev` und proxy.ts brauchen. */
const SERVER_ENV_KEYS = [
  "API_URL",
  "NEXT_PUBLIC_API_URL",
  "NEXT_PUBLIC_OPERATOR_HOSTNAME",
  "NEXT_PUBLIC_PARENTS_HOSTNAME",
  "NEXT_PUBLIC_SCHOOL_HOSTNAME",
  "TENANT_DOMAIN",
  "NEXT_PUBLIC_TENANT_DOMAIN",
  "NEXTAUTH_URL",
  "NEXTAUTH_SECRET",
  "AUTH_JWT_EXPIRY",
  "AUTH_JWT_REFRESH_EXPIRY",
  "METRICS_BEARER_TOKEN",
];

/** @returns {Record<string, string>} */
export function perfServerEnv() {
  const values = readEnvFiles();
  /** @type {Record<string, string>} */
  const env = {};
  for (const key of SERVER_ENV_KEYS) env[key] = requiredEnv(values, key);
  return env;
}

/** @returns {string} */
export function metricsBearerToken() {
  return requiredEnv(readEnvFiles(), "METRICS_BEARER_TOKEN");
}
