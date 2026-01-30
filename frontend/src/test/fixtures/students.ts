import type { BackendStudent, Student } from "~/lib/student-helpers";

let studentIdCounter = 0;

function nextId() {
  return ++studentIdCounter;
}

/**
 * Builds a BackendStudent with sensible defaults (snake_case, numeric IDs).
 * All fields can be overridden.
 */
export function buildBackendStudent(
  overrides?: Partial<BackendStudent>,
): BackendStudent {
  const id = overrides?.id ?? nextId();
  return {
    id,
    person_id: 100 + id,
    first_name: "Max",
    last_name: "Mustermann",
    school_class: "3a",
    current_location: "Schule",
    group_id: 10,
    tag_id: `TAG${String(id).padStart(3, "0")}`,
    created_at: "2024-01-01T00:00:00Z",
    updated_at: "2024-01-15T12:00:00Z",
    ...overrides,
  };
}

/**
 * Builds a frontend Student with sensible defaults (string IDs, camelCase).
 */
export function buildStudent(overrides?: Partial<Student>): Student {
  const id = overrides?.id ?? String(nextId());
  return {
    id,
    name: "Max Mustermann",
    first_name: "Max",
    second_name: "Mustermann",
    school_class: "3a",
    current_location: "Schule",
    group_id: "10",
    ...overrides,
  };
}

/** Reset the auto-increment counter (call in beforeEach if needed). */
export function resetStudentIds() {
  studentIdCounter = 0;
}
