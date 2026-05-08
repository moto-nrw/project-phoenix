import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const HERE = dirname(fileURLToPath(import.meta.url));
const FRONTEND_DIR = resolve(HERE, "..");
const REPO_ROOT = resolve(FRONTEND_DIR, "..");

export const E2E_MANIFEST_PATH = resolve(
  REPO_ROOT,
  "backend",
  ".e2e-manifest.json",
);

export interface E2ERuntimeConfig {
  backend_url: string;
  tenant_domain: string;
  frontend_port: number;
  operator_hostname: string;
}

export interface E2ETwitterTenant {
  organization_id: number;
  school_id: number;
  slug: string;
  name: string;
}

export interface SeedAdminActor {
  email: string;
  password: string;
  display_name: string;
  role: "admin";
  staff_id: number;
}

export interface SeedStaffActor {
  email: string;
  password: string;
  display_name: string;
  role: "user";
  staff_id: number;
}

export interface E2EStudentRef {
  id: number;
  first_name: string;
  last_name: string;
  group_key: string;
  class: string;
}

export interface E2EStudentPair {
  primary: E2EStudentRef;
  secondary: E2EStudentRef;
}

export interface E2EGroupRef {
  id: number;
  key: string;
  display_name: string;
}

export interface E2EGroupPair {
  primary: E2EGroupRef;
  secondary: E2EGroupRef;
}

export interface E2ERoomRef {
  id: number;
  name: string;
}

export interface E2EActivityRef {
  id: number;
  name: string;
}

export interface E2ECheckinFixture {
  student: E2EStudentRef;
  room: E2ERoomRef;
  activity: E2EActivityRef;
  device_key: string;
  rfid_tag: string;
  supervisor: SeedAdminActor;
}

export interface E2EManifest {
  version: string;
  runtime: E2ERuntimeConfig;
  scenario: {
    name: string;
    mode: string;
  };
  tenants: {
    primary: E2ETwitterTenant;
    secondary?: E2ETwitterTenant;
  };
  actors: {
    admin: SeedAdminActor;
    staff: SeedStaffActor;
    operator?: {
      email: string;
      password: string;
    };
  };
  switching?: {
    link_email: string;
    verified: boolean;
    actor: SeedAdminActor;
  };
  devices: {
    default_checkin: {
      key: string;
      api_key: string;
      pin: string;
    };
  };
  fixtures: {
    students: {
      search_pair: E2EStudentPair;
    };
    groups: {
      visible_pair: E2EGroupPair;
    };
    checkin: E2ECheckinFixture;
  };
}

export interface AppUrls {
  primary(path?: string): string;
  secondary(path?: string): string;
  tenant(tenant: E2ETwitterTenant, path?: string): string;
  origin(tenant: E2ETwitterTenant): string;
}

let cachedManifest: E2EManifest | undefined;
let cachedAppUrls: AppUrls | undefined;

function requireRecord(value: unknown, path: string): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error(`${path} must be an object`);
  }
  return value as Record<string, unknown>;
}

function requireString(value: unknown, path: string): string {
  if (typeof value !== "string" || value.length === 0) {
    throw new Error(`${path} must be a non-empty string`);
  }
  return value;
}

function requireNumber(value: unknown, path: string): number {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    throw new Error(`${path} must be a finite number`);
  }
  return value;
}

function validateTenant(value: unknown, path: string): void {
  const tenant = requireRecord(value, path);
  requireNumber(tenant.organization_id, `${path}.organization_id`);
  requireNumber(tenant.school_id, `${path}.school_id`);
  requireString(tenant.slug, `${path}.slug`);
  requireString(tenant.name, `${path}.name`);
}

function validateActor(value: unknown, path: string): void {
  const actor = requireRecord(value, path);
  requireString(actor.email, `${path}.email`);
  requireString(actor.password, `${path}.password`);
  requireString(actor.display_name, `${path}.display_name`);
  requireString(actor.role, `${path}.role`);
  requireNumber(actor.staff_id, `${path}.staff_id`);
}

function validateStudentRef(value: unknown, path: string): void {
  const student = requireRecord(value, path);
  requireNumber(student.id, `${path}.id`);
  requireString(student.first_name, `${path}.first_name`);
  requireString(student.last_name, `${path}.last_name`);
  requireString(student.group_key, `${path}.group_key`);
  requireString(student.class, `${path}.class`);
}

function validateGroupRef(value: unknown, path: string): void {
  const group = requireRecord(value, path);
  requireNumber(group.id, `${path}.id`);
  requireString(group.key, `${path}.key`);
  requireString(group.display_name, `${path}.display_name`);
}

function validateManifest(value: unknown): E2EManifest {
  const root = requireRecord(value, "manifest");
  requireString(root.version, "manifest.version");

  const runtime = requireRecord(root.runtime, "manifest.runtime");
  requireString(runtime.backend_url, "manifest.runtime.backend_url");
  requireString(runtime.tenant_domain, "manifest.runtime.tenant_domain");
  requireNumber(runtime.frontend_port, "manifest.runtime.frontend_port");
  requireString(
    runtime.operator_hostname,
    "manifest.runtime.operator_hostname",
  );

  const scenario = requireRecord(root.scenario, "manifest.scenario");
  requireString(scenario.name, "manifest.scenario.name");
  requireString(scenario.mode, "manifest.scenario.mode");

  const tenants = requireRecord(root.tenants, "manifest.tenants");
  validateTenant(tenants.primary, "manifest.tenants.primary");
  if (tenants.secondary !== undefined) {
    validateTenant(tenants.secondary, "manifest.tenants.secondary");
  }

  const actors = requireRecord(root.actors, "manifest.actors");
  validateActor(actors.admin, "manifest.actors.admin");
  validateActor(actors.staff, "manifest.actors.staff");

  const devices = requireRecord(root.devices, "manifest.devices");
  const defaultCheckin = requireRecord(
    devices.default_checkin,
    "manifest.devices.default_checkin",
  );
  requireString(defaultCheckin.key, "manifest.devices.default_checkin.key");
  requireString(
    defaultCheckin.api_key,
    "manifest.devices.default_checkin.api_key",
  );
  requireString(defaultCheckin.pin, "manifest.devices.default_checkin.pin");

  const fixtures = requireRecord(root.fixtures, "manifest.fixtures");
  const students = requireRecord(
    fixtures.students,
    "manifest.fixtures.students",
  );
  const searchPair = requireRecord(
    students.search_pair,
    "manifest.fixtures.students.search_pair",
  );
  validateStudentRef(
    searchPair.primary,
    "manifest.fixtures.students.search_pair.primary",
  );
  validateStudentRef(
    searchPair.secondary,
    "manifest.fixtures.students.search_pair.secondary",
  );

  const groups = requireRecord(fixtures.groups, "manifest.fixtures.groups");
  const visiblePair = requireRecord(
    groups.visible_pair,
    "manifest.fixtures.groups.visible_pair",
  );
  validateGroupRef(
    visiblePair.primary,
    "manifest.fixtures.groups.visible_pair.primary",
  );
  validateGroupRef(
    visiblePair.secondary,
    "manifest.fixtures.groups.visible_pair.secondary",
  );

  const checkin = requireRecord(
    checkinFromFixtures(fixtures),
    "manifest.fixtures.checkin",
  );
  validateStudentRef(checkin.student, "manifest.fixtures.checkin.student");

  const room = requireRecord(checkin.room, "manifest.fixtures.checkin.room");
  requireNumber(room.id, "manifest.fixtures.checkin.room.id");
  requireString(room.name, "manifest.fixtures.checkin.room.name");

  const activity = requireRecord(
    checkin.activity,
    "manifest.fixtures.checkin.activity",
  );
  requireNumber(activity.id, "manifest.fixtures.checkin.activity.id");
  requireString(activity.name, "manifest.fixtures.checkin.activity.name");

  requireString(checkin.device_key, "manifest.fixtures.checkin.device_key");
  requireString(checkin.rfid_tag, "manifest.fixtures.checkin.rfid_tag");
  validateActor(checkin.supervisor, "manifest.fixtures.checkin.supervisor");

  if (root.switching !== undefined) {
    const switching = requireRecord(root.switching, "manifest.switching");
    requireString(switching.link_email, "manifest.switching.link_email");
    validateActor(switching.actor, "manifest.switching.actor");
    if (typeof switching.verified !== "boolean") {
      throw new Error("manifest.switching.verified must be a boolean");
    }
  }

  return root as unknown as E2EManifest;
}

function checkinFromFixtures(
  fixtures: Record<string, unknown>,
): Record<string, unknown> {
  return requireRecord(fixtures.checkin, "manifest.fixtures.checkin");
}

function normalizePath(path = "/"): string {
  return path.startsWith("/") ? path : `/${path}`;
}

function tenantOrigin(
  runtime: E2ERuntimeConfig,
  tenant: E2ETwitterTenant,
): string {
  return `http://${tenant.slug}.${runtime.tenant_domain}:${runtime.frontend_port}`;
}

function tenantAppURL(
  runtime: E2ERuntimeConfig,
  tenant: E2ETwitterTenant,
  path = "/",
): string {
  return `${tenantOrigin(runtime, tenant)}${normalizePath(path)}`;
}

function loadManifest(): E2EManifest {
  try {
    const raw = readFileSync(E2E_MANIFEST_PATH, "utf-8");
    return validateManifest(JSON.parse(raw) as unknown);
  } catch (err) {
    throw new Error(
      `Could not read ${E2E_MANIFEST_PATH}. Run \`pnpm --dir frontend run e2e\` first.\n` +
        `Underlying error: ${err instanceof Error ? err.message : String(err)}`,
      { cause: err },
    );
  }
}

function createAppUrls(manifest: E2EManifest): AppUrls {
  const runtime = manifest.runtime;
  return {
    primary: (path = "/") =>
      tenantAppURL(runtime, manifest.tenants.primary, path),
    secondary: (path = "/") =>
      tenantAppURL(runtime, requireSecondaryTenant(manifest), path),
    tenant: (tenant, path = "/") => tenantAppURL(runtime, tenant, path),
    origin: (tenant) => tenantOrigin(runtime, tenant),
  };
}

export function getE2EManifest(): E2EManifest {
  if (!cachedManifest) {
    cachedManifest = loadManifest();
  }
  return cachedManifest;
}

export function getE2ERuntime(): E2ERuntimeConfig {
  return getE2EManifest().runtime;
}

export function getAppUrls(): AppUrls {
  if (!cachedAppUrls) {
    cachedAppUrls = createAppUrls(getE2EManifest());
  }
  return cachedAppUrls;
}

export function requireSecondaryTenant(
  manifest: E2EManifest = getE2EManifest(),
): E2ETwitterTenant {
  const tenant = manifest.tenants.secondary;
  if (!tenant) {
    throw new Error(
      "e2e manifest secondary tenant is missing. Re-run `pnpm --dir frontend run e2e`.",
    );
  }
  return tenant;
}

export function _resetContractCacheForTesting(): void {
  cachedManifest = undefined;
  cachedAppUrls = undefined;
}
