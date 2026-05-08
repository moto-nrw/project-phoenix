import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const HERE = dirname(fileURLToPath(import.meta.url));
const FRONTEND_DIR = resolve(HERE, "..");
const REPO_ROOT = resolve(FRONTEND_DIR, "..");

export const E2E_STATE_PATH = resolve(REPO_ROOT, "backend", ".e2e-state.json");
export const EXPECTED_E2E_SCENARIO = "e2e-multi-tenant";
export const EXPECTED_E2E_SCENARIO_MODE = "multi-tenant";

type Scenario = {
  name: string;
  mode: string;
};

type RawTenant = {
  slug: string;
  name: string;
  school_id: number;
  organization_id: number;
};

type Actor = {
  email: string;
  password: string;
  displayName: string;
  role: string;
  staffId: number;
};

type RawActor = {
  email: string;
  password: string;
  display_name: string;
  role: string;
  staff_id: number;
};

type Device = {
  key: string;
  apiKey: string;
  pin: string;
};

type RawDevice = {
  key: string;
  api_key: string;
  pin: string;
};

type StudentRef = {
  id: number;
  firstName: string;
  lastName: string;
  groupKey: string;
  schoolClass: string;
};

type RawStudentRef = {
  id: number;
  first_name: string;
  last_name: string;
  group_key: string;
  class: string;
};

type StudentPair = {
  primary: StudentRef;
  secondary: StudentRef;
};

type StudentSearchProbe = {
  searchTerm: string;
  expectedVisibleName: string;
  expectedFilteredOutName: string;
};

type RawStudentPair = {
  primary: RawStudentRef;
  secondary: RawStudentRef;
};

type GroupRef = {
  id: number;
  key: string;
  displayName: string;
};

type RawGroupRef = {
  id: number;
  key: string;
  display_name: string;
};

type GroupPair = {
  primary: GroupRef;
  secondary: GroupRef;
};

type GroupVisibilityProbe = {
  expectedVisibleNames: [string, string];
};

type RawGroupPair = {
  primary: RawGroupRef;
  secondary: RawGroupRef;
};

type RoomRef = {
  id: number;
  name: string;
};

type ActivityRef = {
  id: number;
  name: string;
};

type CheckinFixture = {
  student: StudentRef;
  room: RoomRef;
  activity: ActivityRef;
  deviceKey: string;
  rfidTag: string;
  supervisor: Actor;
};

type RawCheckinFixture = {
  student: RawStudentRef;
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
  supervisor: RawActor;
};

type AuthSetup = {
  roles: string[];
  requiresSecondaryTenant: boolean;
  requiresVerifiedSwitching: boolean;
};

type RawAuthSetup = {
  roles: string[];
  requires_secondary_tenant: boolean;
  requires_verified_switching: boolean;
};

type Tenant = {
  slug: string;
  name: string;
  schoolId: number;
  organizationId: number;
};

type RawState = {
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
      primary: RawTenant;
      secondary?: RawTenant;
    };
    actors: {
      admin: RawActor;
      staff: RawActor;
      operator?: {
        email: string;
        password: string;
      };
    };
    devices: {
      default_checkin: RawDevice;
    };
  };
  setup: {
    auth: RawAuthSetup;
  };
  fixtures: {
    students: {
      present_ready: RawStudentRef;
      search_pair: RawStudentPair;
    };
    groups: {
      visible_pair: RawGroupPair;
    };
    checkin: RawCheckinFixture;
  };
  assertions: {
    switching: {
      required: boolean;
      verified: boolean;
      link_email?: string;
      actor?: RawActor;
    };
  };
};

export type AppUrls = {
  primary(path?: string): string;
  secondary(path?: string): string;
  tenant(tenant: Tenant, path?: string): string;
  origin(tenant: Tenant): string;
};

let cachedState: RawState | undefined;

function loadState(): RawState {
  try {
    return JSON.parse(readFileSync(E2E_STATE_PATH, "utf-8")) as RawState;
  } catch (err) {
    throw new Error(
      `Could not read ${E2E_STATE_PATH}. Run the canonical flow with \`cd frontend && pnpm e2e\` first.\n` +
        `Underlying error: ${err instanceof Error ? err.message : String(err)}`,
      { cause: err },
    );
  }
}

function getRawState(): RawState {
  if (cachedState === undefined) {
    cachedState = loadState();
  }
  return cachedState;
}

function mapTenant(tenant: RawTenant): Tenant {
  return {
    slug: tenant.slug,
    name: tenant.name,
    schoolId: tenant.school_id,
    organizationId: tenant.organization_id,
  };
}

function mapActor(actor: RawActor): Actor {
  return {
    email: actor.email,
    password: actor.password,
    displayName: actor.display_name,
    role: actor.role,
    staffId: actor.staff_id,
  };
}

function mapDevice(device: RawDevice): Device {
  return {
    key: device.key,
    apiKey: device.api_key,
    pin: device.pin,
  };
}

function mapStudent(student: RawStudentRef): StudentRef {
  return {
    id: student.id,
    firstName: student.first_name,
    lastName: student.last_name,
    groupKey: student.group_key,
    schoolClass: student.class,
  };
}

function mapStudentPair(pair: RawStudentPair): StudentPair {
  return {
    primary: mapStudent(pair.primary),
    secondary: mapStudent(pair.secondary),
  };
}

function mapGroup(group: RawGroupRef): GroupRef {
  return {
    id: group.id,
    key: group.key,
    displayName: group.display_name,
  };
}

function mapGroupPair(pair: RawGroupPair): GroupPair {
  return {
    primary: mapGroup(pair.primary),
    secondary: mapGroup(pair.secondary),
  };
}

function mapCheckinFixture(fixture: RawCheckinFixture): CheckinFixture {
  return {
    student: mapStudent(fixture.student),
    room: fixture.room,
    activity: fixture.activity,
    deviceKey: fixture.device_key,
    rfidTag: fixture.rfid_tag,
    supervisor: mapActor(fixture.supervisor),
  };
}

function mapAuthSetup(setup: RawAuthSetup): AuthSetup {
  return {
    roles: [...setup.roles],
    requiresSecondaryTenant: setup.requires_secondary_tenant,
    requiresVerifiedSwitching: setup.requires_verified_switching,
  };
}

function normalizePath(path = "/"): string {
  return path.startsWith("/") ? path : `/${path}`;
}

function fullName(student: Pick<StudentRef, "firstName" | "lastName">): string {
  return `${student.firstName} ${student.lastName}`;
}

export function tenantOrigin(tenant: Tenant): string {
  const state = getRawState();
  return `http://${tenant.slug}.${state.runtime.tenant_domain}:${state.runtime.frontend_port}`;
}

function tenantAppURL(tenant: Tenant, path = "/"): string {
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

export function getScenario(): Scenario {
  return { ...getRawState().world.scenario };
}

export function getBackendBaseURL(): string {
  return getRawState().runtime.backend_url;
}

export function getPrimaryTenant(): Tenant {
  return mapTenant(getRawState().world.tenants.primary);
}

export function requireSecondaryTenant(): Tenant {
  const tenant = getRawState().world.tenants.secondary;
  if (!tenant) {
    throw new Error(
      "e2e state secondary tenant is missing. Re-run the canonical flow with `cd frontend && pnpm e2e`.",
    );
  }
  return mapTenant(tenant);
}

export function getAdminActor(): Actor {
  return mapActor(getRawState().world.actors.admin);
}

export function getStaffActor(): Actor {
  return mapActor(getRawState().world.actors.staff);
}

export function getAuthSetup(): AuthSetup {
  return mapAuthSetup(getRawState().setup.auth);
}

export function isTenantSwitchVerified(): boolean {
  return getRawState().assertions.switching.verified;
}

export function getCheckinDevice(): Device {
  return mapDevice(getRawState().world.devices.default_checkin);
}

export function getPresentReadyStudent(): StudentRef {
  return mapStudent(getRawState().fixtures.students.present_ready);
}

export function getStudentSearchProbe(): StudentSearchProbe {
  const scenario = mapStudentPair(getRawState().fixtures.students.search_pair);
  return {
    searchTerm: scenario.primary.firstName,
    expectedVisibleName: fullName(scenario.primary),
    expectedFilteredOutName: fullName(scenario.secondary),
  };
}

export function getGroupVisibilityProbe(): GroupVisibilityProbe {
  const scenario = mapGroupPair(getRawState().fixtures.groups.visible_pair);
  return {
    expectedVisibleNames: [
      scenario.primary.displayName,
      scenario.secondary.displayName,
    ],
  };
}

export function getCheckinScenario(): CheckinFixture {
  return mapCheckinFixture(getRawState().fixtures.checkin);
}

export type {
  ActivityRef,
  Actor,
  AuthSetup,
  CheckinFixture,
  Device,
  Scenario,
  GroupVisibilityProbe,
  GroupPair,
  RoomRef,
  StudentSearchProbe,
  StudentPair,
  StudentRef,
  Tenant,
};
