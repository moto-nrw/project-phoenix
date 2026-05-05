import { randomUUID } from "node:crypto";
import { withAdminContext } from "./admin-api";
import { BACKEND_URL } from "./iot";

/**
 * Resource factories for E2E specs. Replaces the per-spec
 * `withAdminContext + try/finally + ad-hoc Date.now() suffix` pattern
 * with auto-cleanup fixtures that:
 *
 *  1. ALWAYS clean up after the test, even on assertion failure,
 *     because Playwright fixture teardown runs unconditionally.
 *  2. Generate parallel-safe unique suffixes via `randomUUID()`.
 *     `Date.now()` collides at four workers when two specs hit the
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

export function makeStudentFactory(): StudentFactory {
  const created: number[] = [];

  return {
    async create(overrides: NewStudent = {}): Promise<CreatedStudent> {
      const suffix = uniqueSuffix();
      const data = {
        first_name: overrides.first_name ?? `E2EStudent${suffix}`,
        last_name: overrides.last_name ?? "Probe",
        school_class: overrides.school_class ?? "1a",
      };

      const student = await withAdminContext(async (ctx, headers) => {
        const res = await ctx.post(`${BACKEND_URL}/api/students`, {
          headers,
          data,
        });
        if (!res.ok()) {
          throw new Error(
            `studentFactory.create failed (${res.status()}): ${await res.text()}`,
          );
        }
        const body = (await res.json()) as { data: CreatedStudent };
        return body.data;
      });

      created.push(student.id);
      return student;
    },
    track(id: number): void {
      created.push(id);
    },
    _created: () => created,
  };
}

export async function teardownStudents(ids: readonly number[]): Promise<void> {
  if (ids.length === 0) return;
  // One admin context for all deletes — saves N round-trips of token
  // acquisition. `failOnStatusCode: false` because a test may have
  // already deleted the row (e.g. the UI-delete spec's whole point).
  await withAdminContext(async (ctx, headers) => {
    for (const id of ids) {
      await ctx.delete(`${BACKEND_URL}/api/students/${id}`, {
        headers,
        failOnStatusCode: false,
      });
    }
  });
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

export function makeGroupFactory(): GroupFactory {
  const created: number[] = [];

  return {
    async create(overrides: NewGroup = {}): Promise<CreatedGroup> {
      const suffix = uniqueSuffix();
      const data = {
        name: overrides.name ?? `E2E Group ${suffix}`,
      };

      const group = await withAdminContext(async (ctx, headers) => {
        const res = await ctx.post(`${BACKEND_URL}/api/groups`, {
          headers,
          data,
        });
        if (!res.ok()) {
          throw new Error(
            `groupFactory.create failed (${res.status()}): ${await res.text()}`,
          );
        }
        const body = (await res.json()) as { data: CreatedGroup };
        return body.data;
      });

      created.push(group.id);
      return group;
    },
    track(id: number): void {
      created.push(id);
    },
    _created: () => created,
  };
}

export async function teardownGroups(ids: readonly number[]): Promise<void> {
  if (ids.length === 0) return;
  // Group DELETE returns 409 while students are still attached. Tests
  // that move students into a factory-owned group are responsible for
  // either deleting the student first (via studentFactory teardown,
  // which runs BEFORE this one if both fixtures are in scope) or
  // detaching it. The 409 is tolerated here so a single-spec failure
  // doesn't block teardown of OTHER groups in the same fixture.
  await withAdminContext(async (ctx, headers) => {
    for (const id of ids) {
      await ctx.delete(`${BACKEND_URL}/api/groups/${id}`, {
        headers,
        failOnStatusCode: false,
      });
    }
  });
}
