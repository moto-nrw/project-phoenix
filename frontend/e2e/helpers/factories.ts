import { randomUUID } from "node:crypto";
import type { APIRequestContext } from "@playwright/test";

/**
 * Resource factories for E2E specs. Replaces the per-spec
 * ad-hoc login contexts, try/finally cleanup, and timestamp-based suffixes
 * with auto-cleanup fixtures that:
 *
 *  1. ALWAYS clean up after the test, even on assertion failure,
 *     because Playwright fixture teardown runs unconditionally.
 *  2. Generate parallel-safe unique suffixes via `randomUUID()`.
 *     Timestamp suffixes collide at four workers when two specs hit the
 *     same millisecond — uuid is collision-safe by construction.
 *  3. Keep the spec body focused on the assertion, not the bookkeeping.
 *
 * These are exported as factory helpers that the fixture wires into
 * Playwright's lifecycle. The fixtures themselves live in
 * `fixtures.ts`; spec authors interact only with the factory API.
 */

/**
 * Short, alphanumeric, parallel-safe suffix. UUID gives us 32 hex chars
 * of entropy; we use the first 8 (16 bits per char × 8 = ~32 bits) which
 * is plenty for "no two workers collide on the same suffix in a single
 * test run". Lowercase to look at home in `firstName + suffix`.
 */
export function uniqueSuffix(): string {
  return randomUUID().replaceAll("-", "").slice(0, 8);
}

// === Students ================================================================

export interface CreatedStudent {
  id: number;
  first_name: string;
  last_name: string;
  school_class: string;
}

export interface NewStudent {
  first_name?: string;
  last_name?: string;
  school_class?: string;
}

/**
 * Tracks created students so the fixture can delete them after the test.
 * Created via {@link makeStudentFactory}; consumed by the
 * `studentFactory` Playwright fixture.
 */
export interface StudentFactory {
  create(overrides?: NewStudent): Promise<CreatedStudent>;
  /**
   * Register an externally-created student for cleanup. Use this when
   * the test creates a row via a path the factory doesn't own (e.g.
   * the UI's create modal) but wants the same auto-cleanup safety net.
   */
  track(id: number): void;
  /** Internal — used by the fixture teardown to know what to clean up. */
  _created(): readonly number[];
}

export function makeStudentFactory(
  adminApi: APIRequestContext,
): StudentFactory {
  const created: number[] = [];

  return {
    async create(overrides: NewStudent = {}): Promise<CreatedStudent> {
      const suffix = uniqueSuffix();
      const data = {
        first_name: overrides.first_name ?? `E2EStudent${suffix}`,
        last_name: overrides.last_name ?? "Probe",
        school_class: overrides.school_class ?? "1a",
      };

      const res = await adminApi.post("/api/students", {
        data,
      });
      if (!res.ok()) {
        throw new Error(
          `studentFactory.create failed (${res.status()}): ${await res.text()}`,
        );
      }
      const body = (await res.json()) as { data: CreatedStudent };
      const student = body.data;

      created.push(student.id);
      return student;
    },
    track(id: number): void {
      created.push(id);
    },
    _created: () => created,
  };
}

export async function teardownStudents(
  adminApi: APIRequestContext,
  ids: readonly number[],
): Promise<void> {
  if (ids.length === 0) return;
  const failures: string[] = [];
  for (const id of ids) {
    const res = await adminApi.delete(`/api/students/${id}`, {
      failOnStatusCode: false,
    });
    if (res.status() === 200 || res.status() === 204 || res.status() === 404) {
      continue;
    }
    failures.push(
      `student ${id} cleanup failed (${res.status()}): ${await res.text()}`,
    );
  }
  if (failures.length > 0) {
    throw new Error(failures.join("\n"));
  }
}

// === Groups ==================================================================

export interface CreatedGroup {
  id: number;
  name: string;
}

export interface NewGroup {
  name?: string;
}

export interface GroupFactory {
  create(overrides?: NewGroup): Promise<CreatedGroup>;
  /**
   * Register an externally-created group for cleanup. Same purpose as
   * {@link StudentFactory.track}.
   */
  track(id: number): void;
  _created(): readonly number[];
}

export function makeGroupFactory(adminApi: APIRequestContext): GroupFactory {
  const created: number[] = [];

  return {
    async create(overrides: NewGroup = {}): Promise<CreatedGroup> {
      const suffix = uniqueSuffix();
      const data = {
        name: overrides.name ?? `E2E Group ${suffix}`,
      };

      const res = await adminApi.post("/api/groups", {
        data,
      });
      if (!res.ok()) {
        throw new Error(
          `groupFactory.create failed (${res.status()}): ${await res.text()}`,
        );
      }
      const body = (await res.json()) as { data: CreatedGroup };
      const group = body.data;

      created.push(group.id);
      return group;
    },
    track(id: number): void {
      created.push(id);
    },
    _created: () => created,
  };
}

export async function teardownGroups(
  adminApi: APIRequestContext,
  ids: readonly number[],
): Promise<void> {
  if (ids.length === 0) return;
  const failures: string[] = [];
  for (const id of ids) {
    const res = await adminApi.delete(`/api/groups/${id}`, {
      failOnStatusCode: false,
    });
    if (res.status() === 200 || res.status() === 204 || res.status() === 404) {
      continue;
    }
    // Group cleanup can legitimately race still-pending student cleanup when
    // a spec failed before it detached or deleted the student it moved into
    // the group. Keep the original product failure visible instead of turning
    // teardown into the primary red with a known "group not empty" 409.
    if (res.status() === 409) {
      continue;
    }
    failures.push(
      `group ${id} cleanup failed (${res.status()}): ${await res.text()}`,
    );
  }
  if (failures.length > 0) {
    throw new Error(failures.join("\n"));
  }
}
