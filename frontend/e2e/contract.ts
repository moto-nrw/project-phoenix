import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

export const E2E_SCENARIO_MODE_MULTI_TENANT = "multi-tenant" as const;
export const E2E_SETUP_ROLE_ADMIN = "admin" as const;
export const E2E_SETUP_ROLE_STAFF = "staff" as const;

const HERE = dirname(fileURLToPath(import.meta.url));
const FRONTEND_DIR = resolve(HERE, "..");
const REPO_ROOT = resolve(FRONTEND_DIR, "..");

export const E2E_MANIFEST_PATH = resolve(
  REPO_ROOT,
  "backend",
  ".e2e-manifest.json",
);

type JsonRecord = Record<string, unknown>;

export interface E2ERuntimeConfig {
  backend_url: string;
  tenant_domain: string;
  frontend_port: number;
  operator_hostname: string;
  nextauth_secret: string;
  auth_trust_host: boolean;
}

export interface E2ETwitterTenant {
  organization_id: number;
  school_id: number;
  slug: string;
  name: string;
}

interface ActorBase {
  email: string;
  password: string;
  display_name: string;
  staff_id: number;
}

export interface SeedAdminActor extends ActorBase {
  role: "admin";
}

export interface SeedStaffActor extends ActorBase {
  role: "user";
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

export interface OperatorCredentials {
  email: string;
  password: string;
}

export interface Switching {
  link_email: string;
  verified: boolean;
  actor: SeedAdminActor;
}

export interface Device {
  key: string;
  api_key: string;
  pin: string;
}

export interface E2EAuthSetup {
  roles: string[];
  requires_secondary_tenant: boolean;
  requires_verified_switching: boolean;
}

export interface E2ESetup {
  auth: E2EAuthSetup;
}

export interface E2EManifest {
  version: string;
  runtime: E2ERuntimeConfig;
  scenario: {
    name: string;
    mode: string;
  };
  setup: E2ESetup;
  tenants: {
    primary: E2ETwitterTenant;
    secondary?: E2ETwitterTenant;
  };
  actors: {
    admin: SeedAdminActor;
    staff: SeedStaffActor;
    operator?: OperatorCredentials;
  };
  switching?: Switching;
  devices: {
    default_checkin: Device;
  };
  fixtures: {
    students: {
      search_pair: E2EStudentPair;
      present_ready: E2EStudentRef;
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

function expectRecord(value: unknown, path: string): JsonRecord {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return value as JsonRecord;
  }
  throw new Error(`${path} must be an object`);
}

function expectString(value: unknown, path: string): string {
  if (typeof value === "string" && value.length > 0) {
    return value;
  }
  throw new Error(`${path} must be a non-empty string`);
}

function expectNumber(value: unknown, path: string): number {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  throw new Error(`${path} must be a finite number`);
}

function expectBoolean(value: unknown, path: string): boolean {
  if (typeof value === "boolean") {
    return value;
  }
  throw new Error(`${path} must be a boolean`);
}

function expectStringArray(value: unknown, path: string): string[] {
  if (!Array.isArray(value) || value.length === 0) {
    throw new Error(`${path} must be a non-empty array`);
  }

  return value.map((entry, index) => expectString(entry, `${path}[${index}]`));
}

function parseTenant(value: unknown, path: string): E2ETwitterTenant {
  const tenant = expectRecord(value, path);
  return {
    organization_id: expectNumber(
      tenant.organization_id,
      `${path}.organization_id`,
    ),
    school_id: expectNumber(tenant.school_id, `${path}.school_id`),
    slug: expectString(tenant.slug, `${path}.slug`),
    name: expectString(tenant.name, `${path}.name`),
  };
}

function parseActorBase(value: unknown, path: string): ActorBase {
  const actor = expectRecord(value, path);
  return {
    email: expectString(actor.email, `${path}.email`),
    password: expectString(actor.password, `${path}.password`),
    display_name: expectString(actor.display_name, `${path}.display_name`),
    staff_id: expectNumber(actor.staff_id, `${path}.staff_id`),
  };
}

function parseAdminActor(value: unknown, path: string): SeedAdminActor {
  const role = expectString(expectRecord(value, path).role, `${path}.role`);
  if (role !== "admin") {
    throw new Error(`${path}.role must be "admin"`);
  }
  return { ...parseActorBase(value, path), role };
}

function parseStaffActor(value: unknown, path: string): SeedStaffActor {
  const role = expectString(expectRecord(value, path).role, `${path}.role`);
  if (role !== "user") {
    throw new Error(`${path}.role must be "user"`);
  }
  return { ...parseActorBase(value, path), role };
}

function parseStudentRef(value: unknown, path: string): E2EStudentRef {
  const ref = expectRecord(value, path);
  return {
    id: expectNumber(ref.id, `${path}.id`),
    first_name: expectString(ref.first_name, `${path}.first_name`),
    last_name: expectString(ref.last_name, `${path}.last_name`),
    group_key: expectString(ref.group_key, `${path}.group_key`),
    class: expectString(ref.class, `${path}.class`),
  };
}

function parseGroupRef(value: unknown, path: string): E2EGroupRef {
  const ref = expectRecord(value, path);
  return {
    id: expectNumber(ref.id, `${path}.id`),
    key: expectString(ref.key, `${path}.key`),
    display_name: expectString(ref.display_name, `${path}.display_name`),
  };
}

function parseRoomRef(value: unknown, path: string): E2ERoomRef {
  const ref = expectRecord(value, path);
  return {
    id: expectNumber(ref.id, `${path}.id`),
    name: expectString(ref.name, `${path}.name`),
  };
}

function parseActivityRef(value: unknown, path: string): E2EActivityRef {
  const ref = expectRecord(value, path);
  return {
    id: expectNumber(ref.id, `${path}.id`),
    name: expectString(ref.name, `${path}.name`),
  };
}

function parseOperator(value: unknown, path: string): OperatorCredentials {
  const operator = expectRecord(value, path);
  return {
    email: expectString(operator.email, `${path}.email`),
    password: expectString(operator.password, `${path}.password`),
  };
}

function parseDevice(value: unknown, path: string): Device {
  const device = expectRecord(value, path);
  return {
    key: expectString(device.key, `${path}.key`),
    api_key: expectString(device.api_key, `${path}.api_key`),
    pin: expectString(device.pin, `${path}.pin`),
  };
}

function parseManifest(value: unknown): E2EManifest {
  const root = expectRecord(value, "e2e manifest");
  const runtime = expectRecord(root.runtime, "e2e manifest.runtime");
  const scenario = expectRecord(root.scenario, "e2e manifest.scenario");
  const setup = expectRecord(root.setup, "e2e manifest.setup");
  const auth = expectRecord(setup.auth, "e2e manifest.setup.auth");
  const tenants = expectRecord(root.tenants, "e2e manifest.tenants");
  const actors = expectRecord(root.actors, "e2e manifest.actors");
  const devices = expectRecord(root.devices, "e2e manifest.devices");
  const fixtures = expectRecord(root.fixtures, "e2e manifest.fixtures");
  const students = expectRecord(
    fixtures.students,
    "e2e manifest.fixtures.students",
  );
  const groups = expectRecord(fixtures.groups, "e2e manifest.fixtures.groups");

  const secondaryTenant =
    tenants.secondary === undefined
      ? undefined
      : parseTenant(tenants.secondary, "e2e manifest.tenants.secondary");

  const operator =
    actors.operator === undefined
      ? undefined
      : parseOperator(actors.operator, "e2e manifest.actors.operator");

  const switching =
    root.switching === undefined
      ? undefined
      : (() => {
          const item = expectRecord(root.switching, "e2e manifest.switching");
          return {
            link_email: expectString(
              item.link_email,
              "e2e manifest.switching.link_email",
            ),
            verified: expectBoolean(
              item.verified,
              "e2e manifest.switching.verified",
            ),
            actor: parseAdminActor(item.actor, "e2e manifest.switching.actor"),
          };
        })();

  return {
    version: expectString(root.version, "e2e manifest.version"),
    runtime: {
      backend_url: expectString(
        runtime.backend_url,
        "e2e manifest.runtime.backend_url",
      ),
      tenant_domain: expectString(
        runtime.tenant_domain,
        "e2e manifest.runtime.tenant_domain",
      ),
      frontend_port: expectNumber(
        runtime.frontend_port,
        "e2e manifest.runtime.frontend_port",
      ),
      operator_hostname: expectString(
        runtime.operator_hostname,
        "e2e manifest.runtime.operator_hostname",
      ),
      nextauth_secret: expectString(
        runtime.nextauth_secret,
        "e2e manifest.runtime.nextauth_secret",
      ),
      auth_trust_host: expectBoolean(
        runtime.auth_trust_host,
        "e2e manifest.runtime.auth_trust_host",
      ),
    },
    scenario: {
      name: expectString(scenario.name, "e2e manifest.scenario.name"),
      mode: expectString(scenario.mode, "e2e manifest.scenario.mode"),
    },
    setup: {
      auth: {
        roles: expectStringArray(auth.roles, "e2e manifest.setup.auth.roles"),
        requires_secondary_tenant: expectBoolean(
          auth.requires_secondary_tenant,
          "e2e manifest.setup.auth.requires_secondary_tenant",
        ),
        requires_verified_switching: expectBoolean(
          auth.requires_verified_switching,
          "e2e manifest.setup.auth.requires_verified_switching",
        ),
      },
    },
    tenants: {
      primary: parseTenant(tenants.primary, "e2e manifest.tenants.primary"),
      secondary: secondaryTenant,
    },
    actors: {
      admin: parseAdminActor(actors.admin, "e2e manifest.actors.admin"),
      staff: parseStaffActor(actors.staff, "e2e manifest.actors.staff"),
      operator,
    },
    switching,
    devices: {
      default_checkin: parseDevice(
        devices.default_checkin,
        "e2e manifest.devices.default_checkin",
      ),
    },
    fixtures: {
      students: {
        search_pair: {
          primary: parseStudentRef(
            expectRecord(
              students.search_pair,
              "e2e manifest.fixtures.students.search_pair",
            ).primary,
            "e2e manifest.fixtures.students.search_pair.primary",
          ),
          secondary: parseStudentRef(
            expectRecord(
              students.search_pair,
              "e2e manifest.fixtures.students.search_pair",
            ).secondary,
            "e2e manifest.fixtures.students.search_pair.secondary",
          ),
        },
        present_ready: parseStudentRef(
          students.present_ready,
          "e2e manifest.fixtures.students.present_ready",
        ),
      },
      groups: {
        visible_pair: {
          primary: parseGroupRef(
            expectRecord(
              groups.visible_pair,
              "e2e manifest.fixtures.groups.visible_pair",
            ).primary,
            "e2e manifest.fixtures.groups.visible_pair.primary",
          ),
          secondary: parseGroupRef(
            expectRecord(
              groups.visible_pair,
              "e2e manifest.fixtures.groups.visible_pair",
            ).secondary,
            "e2e manifest.fixtures.groups.visible_pair.secondary",
          ),
        },
      },
      checkin: (() => {
        const checkin = expectRecord(
          fixtures.checkin,
          "e2e manifest.fixtures.checkin",
        );
        return {
          student: parseStudentRef(
            checkin.student,
            "e2e manifest.fixtures.checkin.student",
          ),
          room: parseRoomRef(
            checkin.room,
            "e2e manifest.fixtures.checkin.room",
          ),
          activity: parseActivityRef(
            checkin.activity,
            "e2e manifest.fixtures.checkin.activity",
          ),
          device_key: expectString(
            checkin.device_key,
            "e2e manifest.fixtures.checkin.device_key",
          ),
          rfid_tag: expectString(
            checkin.rfid_tag,
            "e2e manifest.fixtures.checkin.rfid_tag",
          ),
          supervisor: parseAdminActor(
            checkin.supervisor,
            "e2e manifest.fixtures.checkin.supervisor",
          ),
        };
      })(),
    },
  };
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
    return parseManifest(JSON.parse(raw) as unknown);
  } catch (err) {
    throw new Error(
      `Could not read ${E2E_MANIFEST_PATH}. Run \`./scripts/e2e.sh\` first.\n` +
        `Underlying error: ${err instanceof Error ? err.message : String(err)}`,
      { cause: err },
    );
  }
}

function shellQuote(value: string): string {
  return `'${value.replaceAll("'", `'"'"'`)}'`;
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

export function getFrontendWebServerCommand(
  manifest: E2EManifest = getE2EManifest(),
): string {
  const runtime = manifest.runtime;
  const primaryOrigin = tenantOrigin(runtime, manifest.tenants.primary);
  const env = {
    SKIP_ENV_VALIDATION: "true",
    PORT: String(runtime.frontend_port),
    API_URL: runtime.backend_url,
    NEXT_PUBLIC_API_URL: runtime.backend_url,
    TENANT_DOMAIN: runtime.tenant_domain,
    NEXT_PUBLIC_TENANT_DOMAIN: runtime.tenant_domain,
    NEXT_PUBLIC_OPERATOR_HOSTNAME: runtime.operator_hostname,
    NEXTAUTH_URL: primaryOrigin,
    NEXTAUTH_SECRET: runtime.nextauth_secret,
    AUTH_TRUST_HOST: String(runtime.auth_trust_host),
  };

  const assignments = Object.entries(env)
    .map(([key, value]) => `${key}=${shellQuote(value)}`)
    .join(" ");

  return `${assignments} pnpm run dev --port ${runtime.frontend_port}`;
}

export function requireSecondaryTenant(
  manifest: E2EManifest = getE2EManifest(),
): E2ETwitterTenant {
  const tenant = manifest.tenants.secondary;
  if (!tenant) {
    throw new Error(
      "e2e manifest secondary tenant is missing. Re-run `./scripts/e2e.sh`.",
    );
  }
  return tenant;
}

export function _resetContractCacheForTesting(): void {
  cachedManifest = undefined;
  cachedAppUrls = undefined;
}
