import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const HERE = dirname(fileURLToPath(import.meta.url));
const FRONTEND_DIR = resolve(HERE, "..");
const REPO_ROOT = resolve(FRONTEND_DIR, "..");

export const E2E_STATE_PATH = resolve(REPO_ROOT, "backend", ".e2e-state.json");

export type E2ETwitterTenant = {
  slug: string;
  name: string;
  school_id: number;
  organization_id: number;
};

export type E2EActor = {
  email: string;
  password: string;
  display_name: string;
  role: string;
  staff_id: number;
};

export type E2EDevice = {
  key: string;
  api_key: string;
  pin: string;
};

export type E2EStudentRef = {
  id: number;
  first_name: string;
  last_name: string;
  group_key: string;
  class: string;
};

export type E2EStudentPair = {
  primary: E2EStudentRef;
  secondary: E2EStudentRef;
};

export type E2EGroupPair = {
  primary: {
    id: number;
    key: string;
    display_name: string;
  };
  secondary: {
    id: number;
    key: string;
    display_name: string;
  };
};

export type E2ECheckinFixture = {
  student: E2EStudentRef;
  room: {
    id: number;
    name: string;
  };
  activity: {
    id: number;
    name: string;
  };
  device_key: string;
  rfid_tag: string;
  supervisor: E2EActor;
};

export type E2EAuthSetup = {
  roles: string[];
  requires_secondary_tenant: boolean;
  requires_verified_switching: boolean;
};

export type E2EState = {
  version: string;
  runtime: {
    backend_url: string;
    tenant_domain: string;
    frontend_port: number;
    operator_hostname: string;
    nextauth_secret: string;
    auth_trust_host: boolean;
  };
  world: {
    scenario: {
      name: string;
      mode: string;
    };
    tenants: {
      primary: E2ETwitterTenant;
      secondary?: E2ETwitterTenant;
    };
    actors: {
      admin: E2EActor;
      staff: E2EActor;
      operator?: {
        email: string;
        password: string;
      };
    };
    devices: {
      default_checkin: E2EDevice;
    };
  };
  setup: {
    auth: E2EAuthSetup;
  };
  fixtures: {
    students: {
      present_ready: E2EStudentRef;
      search_pair: E2EStudentPair;
    };
    groups: {
      visible_pair: E2EGroupPair;
    };
    checkin: E2ECheckinFixture;
  };
  assertions: {
    switching: {
      required: boolean;
      verified: boolean;
      link_email?: string;
      actor?: E2EActor;
    };
  };
};

export type AppUrls = {
  primary(path?: string): string;
  secondary(path?: string): string;
  tenant(tenant: E2ETwitterTenant, path?: string): string;
  origin(tenant: E2ETwitterTenant): string;
};

let cachedState: E2EState | undefined;

function loadState(): E2EState {
  try {
    return JSON.parse(readFileSync(E2E_STATE_PATH, "utf-8")) as E2EState;
  } catch (err) {
    throw new Error(
      `Could not read ${E2E_STATE_PATH}. Run \`go run . e2e run\` or \`go run . e2e prepare\` from backend/ first.\n` +
        `Underlying error: ${err instanceof Error ? err.message : String(err)}`,
      { cause: err },
    );
  }
}

function getE2EState(): E2EState {
  if (cachedState === undefined) {
    cachedState = loadState();
  }
  return cachedState;
}

function normalizePath(path = "/"): string {
  return path.startsWith("/") ? path : `/${path}`;
}

export function tenantOrigin(tenant: E2ETwitterTenant): string {
  const state = getE2EState();
  return `http://${tenant.slug}.${state.runtime.tenant_domain}:${state.runtime.frontend_port}`;
}

function tenantAppURL(tenant: E2ETwitterTenant, path = "/"): string {
  return `${tenantOrigin(tenant)}${normalizePath(path)}`;
}

export function getAppUrls(): AppUrls {
  return {
    primary: (path = "/") => tenantAppURL(getPrimaryTenant(), path),
    secondary: (path = "/") => tenantAppURL(requireSecondaryTenant(), path),
    tenant: (tenant, path = "/") => tenantAppURL(tenant, path),
    origin: (tenant) => tenantOrigin(tenant),
  };
}

export function getBackendBaseURL(): string {
  return getE2EState().runtime.backend_url;
}

export function getPrimaryTenant(): E2ETwitterTenant {
  return getE2EState().world.tenants.primary;
}

export function requireSecondaryTenant(): E2ETwitterTenant {
  const tenant = getE2EState().world.tenants.secondary;
  if (!tenant) {
    throw new Error(
      "e2e state secondary tenant is missing. Re-run `go run . e2e prepare` from backend/.",
    );
  }
  return tenant;
}

export function getAdminActor(): E2EActor {
  return getE2EState().world.actors.admin;
}

export function getStaffActor(): E2EActor {
  return getE2EState().world.actors.staff;
}

export function getAuthSetup(): E2EAuthSetup {
  return getE2EState().setup.auth;
}

export function getScenarioMode(): string {
  return getE2EState().world.scenario.mode;
}

export function isTenantSwitchVerified(): boolean {
  return getE2EState().assertions.switching.verified;
}

export function getCheckinDevice(): E2EDevice {
  return getE2EState().world.devices.default_checkin;
}

export function getPresentReadyStudent(): E2EStudentRef {
  return getE2EState().fixtures.students.present_ready;
}

export function getStudentSearchScenario(): E2EStudentPair {
  return getE2EState().fixtures.students.search_pair;
}

export function getGroupVisibilityScenario(): E2EGroupPair {
  return getE2EState().fixtures.groups.visible_pair;
}

export function getCheckinScenario(): E2ECheckinFixture {
  return getE2EState().fixtures.checkin;
}
