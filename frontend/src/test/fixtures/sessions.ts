import type {
  BackendActiveGroup,
  BackendVisit,
  BackendSupervisor,
} from "~/lib/active-helpers";

let activeGroupIdCounter = 0;
let visitIdCounter = 0;
let supervisorIdCounter = 0;

function nextActiveGroupId() {
  return ++activeGroupIdCounter;
}

function nextVisitId() {
  return ++visitIdCounter;
}

function nextSupervisorId() {
  return ++supervisorIdCounter;
}

/**
 * Builds a BackendActiveGroup with sensible defaults.
 */
export function buildBackendActiveSession(
  overrides?: Partial<BackendActiveGroup>,
): BackendActiveGroup {
  const id = overrides?.id ?? nextActiveGroupId();
  return {
    id,
    group_id: 10,
    room_id: 5,
    start_time: "2024-01-15T08:00:00Z",
    end_time: "2024-01-15T12:00:00Z",
    is_active: true,
    notes: "Morning session",
    visit_count: 25,
    supervisor_count: 2,
    room: { id: 5, name: "Room A", category: "classroom" },
    actual_group: { id: 10, name: "Class 3A" },
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-15T08:00:00Z",
    ...overrides,
  };
}

/**
 * Builds a BackendVisit with sensible defaults.
 */
export function buildBackendVisit(
  overrides?: Partial<BackendVisit>,
): BackendVisit {
  const id = overrides?.id ?? nextVisitId();
  return {
    id,
    student_id: 50,
    active_group_id: 1,
    check_in_time: "2024-01-15T08:30:00Z",
    check_out_time: "2024-01-15T11:45:00Z",
    is_active: false,
    notes: "Early checkout",
    student_name: "Max Mustermann",
    school_class: "3a",
    group_name: "OGS Group A",
    active_group_name: "Morning Session",
    created_at: "2024-01-15T08:30:00Z",
    updated_at: "2024-01-15T11:45:00Z",
    ...overrides,
  };
}

/**
 * Builds a BackendSupervisor with sensible defaults.
 */
export function buildBackendSupervisor(
  overrides?: Partial<BackendSupervisor>,
): BackendSupervisor {
  const id = overrides?.id ?? nextSupervisorId();
  return {
    id,
    staff_id: 30,
    active_group_id: 1,
    start_time: "2024-01-15T08:00:00Z",
    end_time: "2024-01-15T12:00:00Z",
    is_active: true,
    notes: "Primary supervisor",
    staff_name: "Frau Schmidt",
    active_group_name: "Morning Session",
    created_at: "2024-01-15T08:00:00Z",
    updated_at: "2024-01-15T08:00:00Z",
    ...overrides,
  };
}

/** Reset auto-increment counters. */
export function resetSessionIds() {
  activeGroupIdCounter = 0;
  visitIdCounter = 0;
  supervisorIdCounter = 0;
}
