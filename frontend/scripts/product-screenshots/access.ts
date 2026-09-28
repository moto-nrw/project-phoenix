import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

import { SEED_STATE_VERSION } from "../seed-state";
import type { Shot } from "./shot-list";

// Wohin die Pipeline fotografiert und womit sie sich anmeldet. Beides kommt aus
// dem lokalen Stack: die Hostnamen aus der Frontend-Env, die Zugänge aus
// backend/.seed-state.json (Profil `marketing`).
//
// Die Pipeline fotografiert nur den eigenen Stack. Ein Host außerhalb von
// *.localhost bricht ab (.claude/rules/no-production-requests.md).

export const PROFILE = "marketing";

export interface Credentials {
  readonly email: string;
  readonly password: string;
}

export interface StackAccess {
  /** `http://<slug>.localhost:<port>` */
  readonly tenantOrigin: string;
  /** `http://eltern.localhost:<port>` */
  readonly parentsOrigin: string;
  readonly admin: Credentials;
  readonly betreuer: Credentials | null;
  readonly eltern: Credentials | null;
}

interface SeedProfileJson {
  school?: { tenant_slug?: string };
  credentials?: {
    school_admin?: Partial<Credentials>;
    accounts?: { betreuer?: Array<Partial<Credentials>> };
    parents?: Array<Partial<Credentials>>;
  };
}

interface SeedStateJson {
  version?: string;
  profiles?: Record<string, SeedProfileJson>;
}

function readEnvFile(path: string): Record<string, string> {
  const values: Record<string, string> = {};
  if (!existsSync(path)) return values;
  for (const line of readFileSync(path, "utf8").split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;
    const separator = trimmed.indexOf("=");
    if (separator <= 0) continue;
    const key = trimmed.slice(0, separator).trim();
    const value = trimmed.slice(separator + 1).trim();
    values[key] = value.replace(/^(["'])(.*)\1$/, "$2");
  }
  return values;
}

/** Prozess-Env vor `.env.local`, wie beim Dev-Server. */
function frontendEnv(
  env: NodeJS.ProcessEnv,
  frontendDir: string,
): (name: string) => string | undefined {
  const file = readEnvFile(join(frontendDir, ".env.local"));
  return (name) => env[name] ?? file[name];
}

function assertLocalHost(host: string, name: string): void {
  const hostname = host.split(":")[0] ?? "";
  if (hostname !== "localhost" && !hostname.endsWith(".localhost")) {
    throw new Error(
      `${name}=${host} zeigt nicht auf localhost. Die Produkt-Screenshots fotografieren nur den lokalen Stack.`,
    );
  }
}

function credentialsOf(
  value: Partial<Credentials> | undefined,
): Credentials | null {
  return value?.email && value.password
    ? { email: value.email, password: value.password }
    : null;
}

export interface LoadAccessOptions {
  readonly statePath?: string;
  readonly frontendDir?: string;
  readonly env?: NodeJS.ProcessEnv;
}

export function loadStackAccess(options: LoadAccessOptions = {}): StackAccess {
  const frontendDir = options.frontendDir ?? process.cwd();
  const statePath =
    options.statePath ?? join(frontendDir, "..", "backend", ".seed-state.json");
  const env = frontendEnv(options.env ?? process.env, frontendDir);

  const parentsHost = env("NEXT_PUBLIC_PARENTS_HOSTNAME");
  const tenantDomain = env("TENANT_DOMAIN");
  if (!parentsHost || !tenantDomain) {
    throw new Error(
      "NEXT_PUBLIC_PARENTS_HOSTNAME und TENANT_DOMAIN fehlen (frontend/.env.local).",
    );
  }
  assertLocalHost(parentsHost, "NEXT_PUBLIC_PARENTS_HOSTNAME");
  assertLocalHost(tenantDomain, "TENANT_DOMAIN");
  const port = parentsHost.split(":")[1];
  const portSuffix = port ? `:${port}` : "";

  if (!existsSync(statePath)) {
    throw new Error(
      `${statePath} fehlt. Zuerst den Stack seeden: scripts/dev-native.sh backend go run . seed ...`,
    );
  }
  const state = JSON.parse(readFileSync(statePath, "utf8")) as SeedStateJson;
  if (state.version !== SEED_STATE_VERSION) {
    throw new Error(
      `Seed-State Version ${JSON.stringify(state.version)} wird nicht unterstützt, erwartet ${SEED_STATE_VERSION}.`,
    );
  }
  const profile = state.profiles?.[PROFILE];
  if (!profile) {
    throw new Error(
      `Das Seed-Profil ${PROFILE} fehlt in ${statePath}. Die Datenbank mit der aktuellen Version neu seeden.`,
    );
  }
  const slug = profile.school?.tenant_slug;
  const admin = credentialsOf(profile.credentials?.school_admin);
  if (!slug || !admin) {
    throw new Error(
      `Das Seed-Profil ${PROFILE} hat keinen vollständigen Schul-Admin-Zugang.`,
    );
  }
  return {
    tenantOrigin: `http://${slug}.${tenantDomain}${portSuffix}`,
    parentsOrigin: `http://${parentsHost}`,
    admin,
    betreuer: credentialsOf(profile.credentials?.accounts?.betreuer?.[0]),
    eltern: credentialsOf(profile.credentials?.parents?.[0]),
  };
}

/** Zugang und Origin für die Rolle eines Shots. */
export function accessForShot(
  access: StackAccess,
  shot: Pick<Shot, "portal" | "rolle" | "id">,
): { origin: string; credentials: Credentials } {
  const credentials = access[shot.rolle];
  if (!credentials) {
    throw new Error(
      `Shot ${shot.id}: Das Seed-Profil ${PROFILE} hat keinen Zugang für die Rolle ${shot.rolle}.`,
    );
  }
  return {
    origin:
      shot.portal === "eltern" ? access.parentsOrigin : access.tenantOrigin,
    credentials,
  };
}
