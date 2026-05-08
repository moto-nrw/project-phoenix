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
      sick_present: RawStudentRef;
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
    const state = JSON.parse(readFileSync(E2E_STATE_PATH, "utf-8")) as RawState;
    validateRawState(state);
    return state;
  } catch (err) {
    throw new Error(
      `Could not read ${E2E_STATE_PATH}. Run the canonical flow with \`cd frontend && pnpm e2e\` first.\n` +
        `Underlying error: ${err instanceof Error ? err.message : String(err)}`,
      { cause: err },
    );
  }
}

function assertNonEmpty(value: unknown, path: string): asserts value is string {
  if (typeof value !== "string" || value.trim() === "") {
    throw new Error(`${path} must be a non-empty string`);
  }
}

function assertPositiveInteger(
  value: unknown,
  path: string,
): asserts value is number {
  if (!Number.isInteger(value) || typeof value !== "number" || value <= 0) {
    throw new Error(`${path} must be a positive integer`);
  }
}

function assertBoolean(value: unknown, path: string): asserts value is boolean {
  if (typeof value !== "boolean") {
    throw new Error(`${path} must be a boolean`);
  }
}

function assertHTTPURL(value: string, path: string): void {
  const parsed = new URL(value);
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    throw new Error(`${path} must use http or https`);
  }
  if (!parsed.host) {
    throw new Error(`${path} must include a host`);
  }
}

function validateTenant(path: string, tenant: RawTenant): void {
  assertPositiveInteger(tenant.organization_id, `${path}.organization_id`);
  assertPositiveInteger(tenant.school_id, `${path}.school_id`);
  assertNonEmpty(tenant.slug, `${path}.slug`);
  assertNonEmpty(tenant.name, `${path}.name`);
}

function validateActor(path: string, actor: RawActor): void {
  assertNonEmpty(actor.email, `${path}.email`);
  assertNonEmpty(actor.password, `${path}.password`);
  assertNonEmpty(actor.display_name, `${path}.display_name`);
  assertNonEmpty(actor.role, `${path}.role`);
  assertPositiveInteger(actor.staff_id, `${path}.staff_id`);
}

function validateStudent(path: string, student: RawStudentRef): void {
  assertPositiveInteger(student.id, `${path}.id`);
  assertNonEmpty(student.first_name, `${path}.first_name`);
  assertNonEmpty(student.last_name, `${path}.last_name`);
  assertNonEmpty(student.group_key, `${path}.group_key`);
  assertNonEmpty(student.class, `${path}.class`);
}

function validateGroup(path: string, group: RawGroupRef): void {
  assertPositiveInteger(group.id, `${path}.id`);
  assertNonEmpty(group.key, `${path}.key`);
  assertNonEmpty(group.display_name, `${path}.display_name`);
}

function validateRawState(state: RawState): void {
  assertNonEmpty(state.version, "state.version");
  if (state.version !== "1") {
    throw new Error(`state.version must be 1, got ${state.version}`);
  }

  assertNonEmpty(state.runtime.backend_url, "state.runtime.backend_url");
  assertHTTPURL(state.runtime.backend_url, "state.runtime.backend_url");
  assertNonEmpty(state.runtime.tenant_domain, "state.runtime.tenant_domain");
  assertPositiveInteger(
    state.runtime.frontend_port,
    "state.runtime.frontend_port",
  );
  assertNonEmpty(
    state.runtime.operator_hostname,
    "state.runtime.operator_hostname",
  );
  assertNonEmpty(
    state.runtime.nextauth_secret,
    "state.runtime.nextauth_secret",
  );
  assertBoolean(state.runtime.auth_trust_host, "state.runtime.auth_trust_host");

  assertNonEmpty(state.world.scenario.name, "state.world.scenario.name");
  assertNonEmpty(state.world.scenario.mode, "state.world.scenario.mode");
  if (state.world.scenario.name !== EXPECTED_E2E_SCENARIO) {
    throw new Error(
      `state.world.scenario.name must be ${EXPECTED_E2E_SCENARIO}`,
    );
  }
  if (state.world.scenario.mode !== EXPECTED_E2E_SCENARIO_MODE) {
    throw new Error(
      `state.world.scenario.mode must be ${EXPECTED_E2E_SCENARIO_MODE}`,
    );
  }

  validateTenant("state.world.tenants.primary", state.world.tenants.primary);
  if (state.setup.auth.requires_secondary_tenant) {
    if (!state.world.tenants.secondary) {
      throw new Error(
        "state.setup.auth.requires_secondary_tenant requires state.world.tenants.secondary",
      );
    }
    validateTenant(
      "state.world.tenants.secondary",
      state.world.tenants.secondary,
    );
  }

  validateActor("state.world.actors.admin", state.world.actors.admin);
  validateActor("state.world.actors.staff", state.world.actors.staff);
  if (state.world.actors.operator) {
    assertNonEmpty(
      state.world.actors.operator.email,
      "state.world.actors.operator.email",
    );
    assertNonEmpty(
      state.world.actors.operator.password,
      "state.world.actors.operator.password",
    );
  }

  assertNonEmpty(
    state.world.devices.default_checkin.key,
    "state.world.devices.default_checkin.key",
  );
  assertNonEmpty(
    state.world.devices.default_checkin.api_key,
    "state.world.devices.default_checkin.api_key",
  );
  assertNonEmpty(
    state.world.devices.default_checkin.pin,
    "state.world.devices.default_checkin.pin",
  );

  if (
    !Array.isArray(state.setup.auth.roles) ||
    state.setup.auth.roles.length === 0
  ) {
    throw new Error("state.setup.auth.roles must be a non-empty array");
  }
  assertBoolean(
    state.setup.auth.requires_secondary_tenant,
    "state.setup.auth.requires_secondary_tenant",
  );
  assertBoolean(
    state.setup.auth.requires_verified_switching,
    "state.setup.auth.requires_verified_switching",
  );

  validateStudent(
    "state.fixtures.students.search_pair.primary",
    state.fixtures.students.search_pair.primary,
  );
  validateStudent(
    "state.fixtures.students.search_pair.secondary",
    state.fixtures.students.search_pair.secondary,
  );
  validateStudent(
    "state.fixtures.students.sick_present",
    state.fixtures.students.sick_present,
  );
  validateGroup(
    "state.fixtures.groups.visible_pair.primary",
    state.fixtures.groups.visible_pair.primary,
  );
  validateGroup(
    "state.fixtures.groups.visible_pair.secondary",
    state.fixtures.groups.visible_pair.secondary,
  );

  validateStudent(
    "state.fixtures.checkin.student",
    state.fixtures.checkin.student,
  );
  assertPositiveInteger(
    state.fixtures.checkin.room.id,
    "state.fixtures.checkin.room.id",
  );
  assertNonEmpty(
    state.fixtures.checkin.room.name,
    "state.fixtures.checkin.room.name",
  );
  assertPositiveInteger(
    state.fixtures.checkin.activity.id,
    "state.fixtures.checkin.activity.id",
  );
  assertNonEmpty(
    state.fixtures.checkin.activity.name,
    "state.fixtures.checkin.activity.name",
  );
  assertNonEmpty(
    state.fixtures.checkin.device_key,
    "state.fixtures.checkin.device_key",
  );
  assertNonEmpty(
    state.fixtures.checkin.rfid_tag,
    "state.fixtures.checkin.rfid_tag",
  );
  validateActor(
    "state.fixtures.checkin.supervisor",
    state.fixtures.checkin.supervisor,
  );

  assertBoolean(
    state.assertions.switching.required,
    "state.assertions.switching.required",
  );
  assertBoolean(
    state.assertions.switching.verified,
    "state.assertions.switching.verified",
  );
  if (
    state.setup.auth.requires_verified_switching &&
    !state.assertions.switching.verified
  ) {
    throw new Error(
      "state.setup.auth.requires_verified_switching requires state.assertions.switching.verified",
    );
  }
  if (state.assertions.switching.verified) {
    assertNonEmpty(
      state.assertions.switching.link_email,
      "state.assertions.switching.link_email",
    );
    if (!state.assertions.switching.actor) {
      throw new Error(
        "state.assertions.switching.actor is required when switching is verified",
      );
    }
    validateActor(
      "state.assertions.switching.actor",
      state.assertions.switching.actor,
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

export function getSickPresentStudent(): StudentRef {
  return mapStudent(getRawState().fixtures.students.sick_present);
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
