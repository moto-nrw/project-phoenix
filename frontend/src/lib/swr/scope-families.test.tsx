import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// Every family of SWR state, on the real SWR 2.5.1.
vi.unmock("swr");

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "A", tenantId: 1 }, expires: "2099-01-01" },
    status: "authenticated",
  }),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "school-a",
}));

// Boundary-level test: the shared wrapper may import the package directly.
import { mutate, preload, useSWRConfig } from "swr";
import { SWRGlobalState } from "swr/_internal";
import { ApiError } from "~/lib/api-error";
import { useDiscardRefusedEntity, useTenantMutateMatching } from "./hooks";
import { resetSWRScope } from "./scope";

/**
 * Invariant (#5006): every cache entry of a scope or an entity goes,
 * whatever family holds it. SWR keeps state for one key in five places: the
 * cache entry, the `$inf$` and `$sub$` entries the key filter of `mutate`
 * skips, a preload that has no cache entry yet, and a mutation in flight.
 */
const FAMILIES = ["plain", "infinite", "subscription", "preload", "mutation"];

function cache() {
  return renderHook(() => useSWRConfig()).result.current.cache;
}

function state() {
  const current = SWRGlobalState.get(cache());
  if (!current) throw new Error("SWR state missing");
  return current;
}

function keysOf(name: string) {
  return {
    plain: `${name}-detail`,
    infinite: `$inf$${name}-feed`,
    subscription: `$sub$${name}-live`,
    preload: `${name}-preloaded`,
    mutation: `${name}-saving`,
  };
}

/** Puts state of every family under `name`; returns how to finish the mutation. */
function seed(name: string): { finishMutation: () => Promise<void> } {
  const keys = keysOf(name);
  cache().set(keys.plain, { data: `${name} Daten` });
  cache().set(keys.infinite, { data: [[`${name} Seite`]] });
  cache().set(keys.subscription, { data: `${name} live` });
  void preload(keys.preload, () => Promise.resolve(`${name} vorgeladen`));
  let finish: (value: string) => void = () => {};
  const saving = mutate(
    keys.mutation,
    new Promise<string>((resolve) => {
      finish = resolve;
    }),
    { revalidate: false },
  );
  return {
    finishMutation: async () => {
      finish(`${name} gespeichert`);
      await saving;
    },
  };
}

/** Which families still hold state of `name`. */
function present(name: string): string[] {
  const keys = keysOf(name);
  const [, , , PRELOAD] = state();
  const found: string[] = [];
  if (cache().get(keys.plain)?.data !== undefined) found.push("plain");
  if (cache().get(keys.infinite)?.data !== undefined) found.push("infinite");
  if (cache().get(keys.subscription)?.data !== undefined) {
    found.push("subscription");
  }
  if (keys.preload in PRELOAD) found.push("preload");
  if (cache().get(keys.mutation)?.data !== undefined) found.push("mutation");
  return found;
}

const refusal = new ApiError("Forbidden", 403, {
  code: "care.guardian_permission_denied",
});

beforeEach(() => {
  resetSWRScope({ revalidate: false });
});

describe("every family of a scope or entity is dropped (#5006)", () => {
  it("drops a refused child in every family and leaves the others", async () => {
    const child = seed("school-a:A:student-7");
    const sibling = seed("school-a:A:student-8");
    const otherAccount = seed("school-a:B:student-7");
    const { result } = renderHook(() => useDiscardRefusedEntity());

    act(() => {
      result.current(refusal, {
        codes: ["care.guardian_permission_denied"],
        matches: (key) => key.startsWith("student-7-"),
      });
    });
    await act(async () => {
      await child.finishMutation();
      await sibling.finishMutation();
      await otherAccount.finishMutation();
    });

    // Anker: das andere Kind und das andere Konto behalten jede Familie.
    expect(present("school-a:A:student-8")).toEqual(FAMILIES);
    expect(present("school-a:B:student-7")).toEqual(FAMILIES);
    expect(present("school-a:A:student-7")).toEqual([]);
  });

  it("serves no withdrawn preload to a later reader of the refused child", async () => {
    void preload("school-a:A:student-7-preloaded", () =>
      Promise.resolve("entzogen"),
    );
    const { result } = renderHook(() => useDiscardRefusedEntity());

    let dropped = false;
    act(() => {
      dropped = result.current(refusal, {
        codes: ["care.guardian_permission_denied"],
        matches: (key) => key.startsWith("student-7-"),
      });
    });

    expect(dropped).toBe(true);
    const [, , , PRELOAD] = state();
    expect(Object.keys(PRELOAD)).not.toContain(
      "school-a:A:student-7-preloaded",
    );
  });

  it("drops every family of every scope at a session boundary", async () => {
    const own = seed("school-a:A:student-7");
    const other = seed("school-a:B:student-7");
    // Anker: vor dem Wechsel steht jede Familie.
    expect(present("school-a:A:student-7")).toEqual(
      FAMILIES.filter((family) => family !== "mutation"),
    );

    resetSWRScope({ revalidate: false });
    await act(async () => {
      await own.finishMutation();
      await other.finishMutation();
    });

    expect(present("school-a:A:student-7")).toEqual([]);
    expect(present("school-a:B:student-7")).toEqual([]);
  });

  it("drops the families the key filter skips when clearing matching keys", async () => {
    const matched = seed("school-a:A:request-feed:7");
    const kept = seed("school-a:A:request-feed:8");
    const { result } = renderHook(() =>
      useTenantMutateMatching(["request-feed:7"]),
    );

    await act(async () => {
      await result.current({ clear: true });
      await matched.finishMutation();
      await kept.finishMutation();
    });

    // Anker: der nicht getroffene Feed behält jede Familie.
    expect(present("school-a:A:request-feed:8")).toEqual(FAMILIES);
    expect(present("school-a:A:request-feed:7")).toEqual([]);
  });

  it("drops a stale preload when revalidating matching keys", async () => {
    void preload("school-a:A:request-feed:7-preloaded", () =>
      Promise.resolve("vor der Entscheidung"),
    );
    void preload("school-a:A:request-feed:8-preloaded", () =>
      Promise.resolve("anderer Feed"),
    );
    const { result } = renderHook(() =>
      useTenantMutateMatching(["request-feed:7"]),
    );

    await act(async () => {
      await result.current();
    });

    const [, , , PRELOAD] = state();
    // Anker: der nicht getroffene Preload bleibt.
    expect(Object.keys(PRELOAD)).toContain(
      "school-a:A:request-feed:8-preloaded",
    );
    expect(Object.keys(PRELOAD)).not.toContain(
      "school-a:A:request-feed:7-preloaded",
    );
  });
});
