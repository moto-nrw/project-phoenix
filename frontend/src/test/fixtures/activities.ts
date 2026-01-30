import type {
  BackendActivity,
  BackendActivityCategory,
  Activity,
  ActivityCategory,
} from "~/lib/activity-helpers";

let activityIdCounter = 0;
let categoryIdCounter = 0;

function nextActivityId() {
  return ++activityIdCounter;
}

function nextCategoryId() {
  return ++categoryIdCounter;
}

/**
 * Builds a BackendActivity with sensible defaults.
 */
export function buildBackendActivity(
  overrides?: Partial<BackendActivity>,
): BackendActivity {
  const id = overrides?.id ?? nextActivityId();
  return {
    id,
    name: "Basketball AG",
    max_participants: 20,
    is_open: true,
    category_id: 1,
    supervisor_id: 10,
    enrollment_count: 15,
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
    ...overrides,
  };
}

/**
 * Builds a frontend Activity with sensible defaults.
 */
export function buildActivity(overrides?: Partial<Activity>): Activity {
  const id = overrides?.id ?? String(nextActivityId());
  return {
    id,
    name: "Basketball AG",
    max_participant: 20,
    is_open_ags: true,
    supervisor_id: "10",
    ag_category_id: "1",
    category_name: "Sport",
    participant_count: 15,
    created_at: new Date("2024-01-01T00:00:00Z"),
    updated_at: new Date("2024-01-01T00:00:00Z"),
    ...overrides,
  };
}

/**
 * Builds a BackendActivityCategory with sensible defaults.
 */
export function buildBackendCategory(
  overrides?: Partial<BackendActivityCategory>,
): BackendActivityCategory {
  const id = overrides?.id ?? nextCategoryId();
  return {
    id,
    name: "Sport",
    description: "Sports activities",
    color: "#3b82f6",
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-01T00:00:00Z",
    ...overrides,
  };
}

/**
 * Builds a frontend ActivityCategory with sensible defaults.
 */
export function buildCategory(
  overrides?: Partial<ActivityCategory>,
): ActivityCategory {
  const id = overrides?.id ?? String(nextCategoryId());
  return {
    id,
    name: "Sport",
    description: "Sports activities",
    color: "#3b82f6",
    created_at: new Date("2024-01-01T00:00:00Z"),
    updated_at: new Date("2024-01-01T00:00:00Z"),
    ...overrides,
  };
}

/** Reset auto-increment counters. */
export function resetActivityIds() {
  activityIdCounter = 0;
  categoryIdCounter = 0;
}
