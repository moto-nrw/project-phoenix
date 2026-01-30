import type { BackendGroup, Group } from "~/lib/group-helpers";

let groupIdCounter = 0;

function nextId() {
  return ++groupIdCounter;
}

/**
 * Builds a BackendGroup with sensible defaults.
 */
export function buildBackendGroup(
  overrides?: Partial<BackendGroup>,
): BackendGroup {
  const id = overrides?.id ?? nextId();
  return {
    id,
    name: "Class 3A",
    room_id: 10,
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-15T12:00:00Z",
    ...overrides,
  };
}

/**
 * Builds a frontend Group with sensible defaults.
 */
export function buildGroup(overrides?: Partial<Group>): Group {
  const id = overrides?.id ?? String(nextId());
  return {
    id,
    name: "Class 3A",
    room_id: "10",
    ...overrides,
  };
}

/** Reset the auto-increment counter. */
export function resetGroupIds() {
  groupIdCounter = 0;
}
