import { readFileSync } from "node:fs";
import { join } from "node:path";

export const SEED_STATE_VERSION = "3";

export interface SeedAccess {
  slug: string;
  email: string;
  password: string;
}

interface SeedProfile {
  school?: { tenant_slug?: string };
  credentials?: {
    accounts?: { admin?: Array<{ email?: string; password?: string }> };
  };
}

interface SeedState {
  version?: string;
  default_profile?: string;
  profiles?: Record<string, SeedProfile>;
}

interface LoadSeedAccessOptions {
  profile?: string;
  statePath?: string;
}

function defaultSeedStatePath(): string {
  return join(process.cwd(), "..", "backend", ".seed-state.json");
}

function isMissingFile(error: unknown): boolean {
  return (
    error instanceof Error &&
    "code" in error &&
    (error as NodeJS.ErrnoException).code === "ENOENT"
  );
}

export function readSeedAccess(
  options: LoadSeedAccessOptions = {},
): SeedAccess | null {
  const statePath = options.statePath ?? defaultSeedStatePath();
  let raw: string;
  try {
    raw = readFileSync(statePath, "utf8");
  } catch (error) {
    if (isMissingFile(error)) return null;
    throw error;
  }

  const state = JSON.parse(raw) as SeedState;
  if (state.version !== SEED_STATE_VERSION) {
    throw new Error(
      `Unsupported seed state version ${JSON.stringify(state.version)}; supported version is ${SEED_STATE_VERSION}`,
    );
  }
  const profileKey = options.profile ?? state.default_profile;
  if (!profileKey) {
    throw new Error("Seed state has no default_profile");
  }
  const profile = state.profiles?.[profileKey];
  if (!profile) {
    const available = Object.keys(state.profiles ?? {})
      .sort()
      .join(", ");
    throw new Error(
      `Unknown demo school profile ${JSON.stringify(profileKey)}; available profiles: ${available}`,
    );
  }
  const admin = profile.credentials?.accounts?.admin?.[0];
  const slug = profile.school?.tenant_slug;
  if (!slug || !admin?.email || !admin.password) {
    throw new Error(
      `Demo school profile ${JSON.stringify(profileKey)} has no complete school-admin access`,
    );
  }
  return { slug, email: admin.email, password: admin.password };
}

export function loadSeedAccess(
  options: LoadSeedAccessOptions = {},
): SeedAccess | null {
  const fileAccess = readSeedAccess(options);
  let slug = fileAccess?.slug;
  let email = fileAccess?.email;
  let password = fileAccess?.password;
  if (process.env.E2E_TENANT_SLUG) slug = process.env.E2E_TENANT_SLUG;
  if (process.env.E2E_TEST_EMAIL) email = process.env.E2E_TEST_EMAIL;
  if (process.env.E2E_TEST_PASSWORD) password = process.env.E2E_TEST_PASSWORD;
  return slug && email && password ? { slug, email, password } : null;
}
