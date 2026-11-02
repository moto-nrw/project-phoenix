import { act, render, renderHook, screen } from "@testing-library/react";
import type { Session } from "next-auth";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// Every place SWR 2.5.1 keeps state for a key, on the real SWR.
vi.unmock("swr");

const { mockUseSession } = vi.hoisted(() => ({ mockUseSession: vi.fn() }));

vi.mock("next-auth/react", () => ({ useSession: mockUseSession }));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "school-a",
}));
vi.mock("~/lib/session-cache", () => ({ announceSessionChange: vi.fn() }));

// Boundary-level test: the shared wrapper may import the package directly.
import { mutate, preload, useSWRConfig } from "swr";
import { SWRGlobalState, getTimestamp } from "swr/_internal";
import { ApiError } from "~/lib/api-error";
import { _resetPersonalStorageLeave } from "~/lib/personal-storage";
import {
  useDiscardRefusedEntity,
  useSWRAuth,
  useTenantMutateMatching,
} from "./hooks";
import { resetSWRScope, resumeSWRSession } from "./scope";
import { SWRScopeBoundary } from "./scope-boundary";

/**
 * Invariant (#5006): after a drop no reader sees data or a request of the
 * dropped keys, whichever place SWR keeps them in. Each row of LOCATIONS is
 * one such place; each operation below runs against all of them.
 */
const LOCATIONS = [
  "entry", // cache entry data
  "infinite", // `$inf$` entry, skipped by SWR's key filter
  "subscription", // `$sub$` entry, skipped by SWR's key filter
  "preload", // PRELOAD without a cache entry
  "request", // FETCH marker without a reader's `_k` (dedupe window)
  "mutation", // mutation in flight without a cache entry
  "rollback", // optimistic mutation that fails afterwards (`_c`)
  "loading", // entry left validating by a discarded request
] as const;
type Location = (typeof LOCATIONS)[number];

function swrCache() {
  return renderHook(() => useSWRConfig()).result.current.cache;
}

function swrState() {
  const state = SWRGlobalState.get(swrCache());
  if (!state) throw new Error("SWR state missing");
  return state;
}

function key(name: string, location: Location): string {
  if (location === "infinite") return `$inf$${name}-${location}`;
  if (location === "subscription") return `$sub$${name}-${location}`;
  return `${name}-${location}`;
}

/** Puts state of every location under `name`; settle() ends what is in flight. */
function seed(name: string): { settle: () => Promise<void> } {
  const cache = swrCache();
  const [, , FETCH] = swrState();
  cache.set(key(name, "entry"), { data: "alt" });
  cache.set(key(name, "infinite"), { data: [["alt"]] });
  cache.set(key(name, "subscription"), { data: "alt" });
  void preload(key(name, "preload"), () => Promise.resolve("alt"));
  FETCH[key(name, "request")] = [Promise.resolve("alt"), getTimestamp()];
  cache.set(key(name, "loading"), {
    data: "alt",
    isValidating: true,
    isLoading: true,
  });
  let finish: (value: string) => void = () => {};
  const saving = mutate(
    key(name, "mutation"),
    new Promise<string>((resolve) => {
      finish = resolve;
    }),
    { revalidate: false },
  );
  cache.set(key(name, "rollback"), { data: "alt" });
  let fail: (error: Error) => void = () => {};
  const optimistic = mutate(
    key(name, "rollback"),
    new Promise<string>((_, reject) => {
      fail = reject;
    }),
    {
      optimisticData: "optimistisch",
      rollbackOnError: true,
      revalidate: false,
    },
  ).catch(() => undefined);
  return {
    settle: async () => {
      finish("alt");
      fail(new Error("Speichern fehlgeschlagen"));
      await saving;
      await optimistic;
    },
  };
}

/** The locations of `name` that still hold old data or an old request. */
function holding(name: string): Location[] {
  const cache = swrCache();
  const [, , FETCH, PRELOAD] = swrState();
  return LOCATIONS.filter((location) => {
    const k = key(name, location);
    if (location === "preload") return k in PRELOAD;
    if (location === "request") return k in FETCH;
    if (location === "loading") {
      const entry = cache.get(k);
      return entry?.data !== undefined || entry?.isValidating === true;
    }
    return cache.get(k)?.data !== undefined;
  });
}

const OWN = "school-a:A:student-7";
const SIBLING = "school-a:A:student-8";
const OTHER_ACCOUNT = "school-a:B:student-7";

function signedIn(user: Partial<Session["user"]> = {}) {
  return {
    data: { user: { id: "A", tenantId: 1, ...user }, expires: "2099-01-01" },
    status: "authenticated" as const,
  };
}

const refusal = new ApiError("Forbidden", 403, {
  code: "care.guardian_permission_denied",
});

interface Tools {
  view: ReturnType<typeof render>;
  discard: ReturnType<typeof useDiscardRefusedEntity>;
  matching: ReturnType<typeof useTenantMutateMatching>;
}
type Operation = (tools: Tools) => Promise<void> | void;

/** Every operation that drops state, with the names it must leave alone. */
const DROPS: [string, Operation, string[]][] = [
  [
    "rights change in this tab (scope reset)",
    ({ view }) => {
      mockUseSession.mockReturnValue(signedIn({ permissions: ["x"] }));
      view.rerender(<SWRScopeBoundary expectedTenantId={1} />);
    },
    [],
  ],
  [
    "logout",
    ({ view }) => {
      mockUseSession.mockReturnValue({ data: null, status: "unauthenticated" });
      view.rerender(<SWRScopeBoundary expectedTenantId={1} />);
    },
    [],
  ],
  [
    "another account in another tab",
    () => {
      globalThis.dispatchEvent(
        new StorageEvent("storage", {
          key: "moto:personal-storage-owner",
          newValue: "1:B",
        }),
      );
    },
    [],
  ],
  [
    "refused entity",
    ({ discard }) => {
      discard(refusal, {
        codes: ["care.guardian_permission_denied"],
        matches: (k) => k.startsWith("student-7-"),
      });
    },
    [SIBLING, OTHER_ACCOUNT],
  ],
  [
    "clearing matching keys",
    async ({ matching }) => {
      await matching({ clear: true });
    },
    [SIBLING, OTHER_ACCOUNT],
  ],
];

beforeEach(() => {
  resetSWRScope({ revalidate: false });
  mockUseSession.mockReturnValue(signedIn());
});

afterEach(() => {
  resumeSWRSession();
  _resetPersonalStorageLeave();
});

describe("no state survives a drop, in any location (#5006)", () => {
  it.each(DROPS)("%s", async (_name, operation, kept) => {
    const view = render(<SWRScopeBoundary expectedTenantId={1} />);
    const discard = renderHook(() => useDiscardRefusedEntity()).result.current;
    const matching = renderHook(() => useTenantMutateMatching(["student-7-"]))
      .result.current;
    const seeded = [OWN, SIBLING, OTHER_ACCOUNT].map(seed);

    await act(async () => {
      await operation({ view, discard, matching });
    });
    await act(async () => {
      for (const { settle } of seeded) await settle();
    });

    // Anker: was der Vorgang nicht trifft, behält jeden Zustandsort; der
    // Fehlschlag einer Rücknahme setzt dort den Stand vor der Mutation.
    for (const name of kept) expect(holding(name)).toEqual(LOCATIONS);
    expect(holding(OWN)).toEqual([]);
  });

  it("leaves a key named in `except` untouched in every location", async () => {
    seed(OWN);
    const { result } = renderHook(() =>
      useTenantMutateMatching(["student-7-"]),
    );

    await act(async () => {
      await result.current({
        clear: true,
        except: LOCATIONS.map((location) =>
          key("student-7", location).replace(/^\$(inf|sub)\$/, ""),
        ),
      });
    });

    // Anker: nichts ist verworfen; in-flight Mutationen laufen noch.
    expect(holding(OWN)).toEqual(
      LOCATIONS.filter((location) => location !== "mutation"),
    );
  });

  it("lets no later reader reuse a request or preload after revalidating", async () => {
    // The finding of #5182: a FETCH marker without a reader's `_k` stayed,
    // and the next mount inside the dedupe window took the old answer.
    seed(OWN);
    const { result } = renderHook(() =>
      useTenantMutateMatching(["student-7-"]),
    );

    await act(async () => {
      await result.current();
    });

    const [, , FETCH, PRELOAD] = swrState();
    expect(key(OWN, "request") in FETCH).toBe(false);
    expect(key(OWN, "preload") in PRELOAD).toBe(false);

    const fresh = vi.fn(() => Promise.resolve("frisch"));
    function Reader({ name }: Readonly<{ name: string }>) {
      const { data } = useSWRAuth(name, fresh);
      return <p>{data ?? "leer"}</p>;
    }
    render(
      <>
        <Reader name="student-7-request" />
        <Reader name="student-7-preload" />
      </>,
    );
    expect(await screen.findAllByText("frisch")).toHaveLength(2);
    expect(fresh).toHaveBeenCalledTimes(2);
  });
});
