import { act, render, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, onTestFinished, vi } from "vitest";
import { SWRConfig, useSWRConfig, type Middleware } from "swr";
import { SWRSeedBoundary } from "./config-boundaries";
import {
  MUTATION_OPTION_KEYS,
  READER_OPTION_KEYS,
  useTenantMutate,
} from "./hooks";
import { messageKeys } from "./portal-keys";
import { roomKeys } from "./rooms-keys";
import { resetSWRScope, resumeSWRSession, suspendSWRSession } from "./scope";
import {
  mutate,
  useInvalidateMatching,
  useSWRAuth,
  useSWRSeedFrom,
  useTenantMutateMatching,
  writeSWRSeeds,
} from "./index";

vi.unmock("swr");
const { owner } = vi.hoisted(() => ({
  owner: { id: "account-7", scope: "tenant", tenantId: 42 },
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: owner, expires: "2099-01-01" },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "school-a",
}));

beforeEach(() => {
  resetSWRScope({ revalidate: false });
  resumeSWRSession();
});

const own = (key: string) => `school-a:account-7:${key}`;

// The reviewed public surface (#5007): swr/no-cache-access lets code outside
// ~/lib/swr reach these modules only. A new export lands here in review; none
// may hand out the cache, a provider or a raw mutate.
const PUBLIC_SURFACE: Record<string, () => Promise<object>> = {
  index: () => import("./index"),
  hooks: () => import("./hooks"),
  mutate: () => import("./mutate"),
  "config-boundaries": () => import("./config-boundaries"),
  "scope-boundary": () => import("./scope-boundary"),
  "course-waitlist-keys": () => import("./course-waitlist-keys"),
  "portal-keys": () => import("./portal-keys"),
  "shell-keys": () => import("./shell-keys"),
  "parent-keys": () => import("./parent-keys"),
  "rooms-keys": () => import("./rooms-keys"),
  "groups-keys": () => import("./groups-keys"),
  "staff-keys": () => import("./staff-keys"),
  "me-keys": () => import("./me-keys"),
  rooms: () => import("./rooms"),
  groups: () => import("./groups"),
  staff: () => import("./staff"),
  me: () => import("./me"),
  "search-students-key": () => import("./search-students-key"),
  "room-derived-caches": () => import("./room-derived-caches"),
  optimistic: () => import("./optimistic"),
};
const REVIEWED_EXPORTS: Record<string, string[]> = {
  index: [
    "IsolatedSWRCache",
    "SWRScopeBoundary",
    "SWRSeedBoundary",
    "authGeneration",
    "mutate",
    "scopedSWRKey",
    "stampSWRSeeds",
    "tenantSWRScope",
    "useImmutableSWR",
    "useInvalidateFamily",
    "useInvalidateMatching",
    "useSWRAuth",
    "useSWRCacheKey",
    "useSWRPreload",
    "useSWRSeedFrom",
    "useSWRSessionRefresh",
    "useTenantMutate",
    "useTenantMutateMatching",
    "writeSWRSeeds",
  ],
  hooks: [
    "MUTATION_OPTION_KEYS",
    "READER_OPTION_KEYS",
    "useDiscardRefusedEntity",
    "useImmutableSWR",
    "useInvalidateFamily",
    "useInvalidateMatching",
    "useSWRAuth",
    "useSWRCacheKey",
    "useSWRPreload",
    "useSWRRequestGuard",
    "useSWRWithId",
    "useScopedSWRKey",
    "useTenantMutate",
    "useTenantMutateMatching",
  ],
  mutate: ["mutate"],
  "config-boundaries": ["IsolatedSWRCache", "SWRSeedBoundary"],
  "scope-boundary": ["SWRScopeBoundary"],
  "course-waitlist-keys": ["activityWaitlistKeys", "courseWaitlistKeys"],
  "portal-keys": [
    "announcementKeys",
    "catererKeys",
    "dashboardKeys",
    "databaseKeys",
    "displayKeys",
    "homeLayoutKeys",
    "messageKeys",
    "operatorAnnouncementKeys",
    "operatorBillingKeys",
    "operatorInvitationKeys",
    "operatorProvisioningKeys",
    "parentAnnouncementKeys",
    "parentKeys",
    "payrollKeys",
    "schoolPortalKeys",
    "schoolSetupKeys",
    "staffMergeKeys",
    "staffOnboardingKeys",
    "studentKeys",
    "timeTrackingKeys",
    "timetableKeys",
    "traegerKeys",
    "weekProtocolKeys",
  ],
  "shell-keys": ["SWR_FAMILIES", "announcementKeys", "dashboardKeys"],
  "parent-keys": ["parentKeys"],
  "rooms-keys": ["roomKeys"],
  "groups-keys": ["groupKeys"],
  "staff-keys": ["staffKeys"],
  "me-keys": ["meKeys"],
  rooms: ["rooms"],
  groups: ["groups"],
  staff: ["staff"],
  me: ["me"],
  "search-students-key": [
    "SEARCH_STUDENTS_ALL_GROUPS_KEY",
    "SEARCH_STUDENTS_KEY_PREFIX",
    "normalizeSearchStudentsGroupId",
    "searchStudentsGroupScope",
    "searchStudentsKeyTargetsGroup",
  ],
  "room-derived-caches": ["ROOM_DERIVED_CACHE_KEY_FRAGMENTS"],
  optimistic: ["useOptimisticAction"],
};

// #5007: outside ~/lib/swr no cache access bypasses the generation lock.
describe("cache boundary of ~/lib/swr", () => {
  it.each(Object.keys(PUBLIC_SURFACE))(
    "keeps the reviewed exports of the public module %s",
    async (name) => {
      const exports = await PUBLIC_SURFACE[name]!();
      expect(Object.keys(exports).sort()).toEqual(REVIEWED_EXPORTS[name]);
      // Neither SWR's provider nor anything carrying its cache or mutate.
      for (const value of Object.values(exports)) {
        expect(value).not.toHaveProperty("defaultValue");
        expect(value).not.toHaveProperty("cache");
      }
    },
  );

  it("passes only revalidate to a clear, never written data", async () => {
    const { result } = renderHook(() => useSWRConfig().cache);
    result.current.set(own("held"), { data: "server" });
    const options = {
      revalidate: false,
      optimisticData: "smuggled",
      populateCache: false,
    };
    await act(async () => {
      await mutate(own("held"), undefined, options);
    });
    expect(result.current.get(own("held"))?.data).toBeUndefined();
  });

  it("refuses a raw data write, also after a scope bump", async () => {
    const key = roomKeys.key();
    const reader = renderHook(() =>
      useSWRAuth(key, async () => ["server"], { revalidateIfStale: false }),
    );
    await waitFor(() => expect(reader.result.current.data).toEqual(["server"]));
    act(() => resetSWRScope({ revalidate: false }));

    await expect(
      // @ts-expect-error a data write is no longer part of the contract
      mutate(own(key), ["late-raw"]),
    ).rejects.toThrow(TypeError);
    await expect(
      // @ts-expect-error neither through a key filter
      mutate(() => true, ["late-raw"]),
    ).rejects.toThrow(TypeError);
    expect(reader.result.current.data).not.toEqual(["late-raw"]);
  });

  it("still revalidates and clears through key filters", async () => {
    const fetcher = vi.fn(async () => ["server"]);
    const reader = renderHook(() => useSWRAuth(roomKeys.key(), fetcher));
    await waitFor(() => expect(reader.result.current.data).toEqual(["server"]));
    await act(async () => {
      await mutate((key) => typeof key === "string" && key.includes("rooms-"));
    });
    expect(fetcher).toHaveBeenCalledTimes(2);
  });
});

describe("useInvalidateMatching", () => {
  it("passes the resource key and touches only the session's scope", async () => {
    const { result } = renderHook(() => ({
      cache: useSWRConfig().cache,
      invalidate: useInvalidateMatching(),
    }));
    const keys = [
      own("staff-month-summary-1"),
      "school-a:account-8:staff-month-summary-1",
      "school-b:account-7:staff-month-summary-1",
      own("students-list"),
    ];
    for (const key of keys) result.current.cache.set(key, { data: [key] });
    const seen: string[] = [];
    await act(async () => {
      await result.current.invalidate(
        (key) => {
          seen.push(key);
          return key.startsWith("staff-month-summary-");
        },
        { clear: true },
      );
    });
    expect(seen).toEqual(
      expect.arrayContaining(["staff-month-summary-1", "students-list"]),
    );
    expect(result.current.cache.get(keys[0]!)?.data).toBeUndefined();
    for (const key of keys.slice(1))
      expect(result.current.cache.get(key)?.data).toEqual([key]);
  });

  it("refreshes every reader of the session for `() => true`", async () => {
    const rooms = vi.fn(async () => ["rooms"]);
    const inbox = vi.fn(async () => ["inbox"]);
    const readers = renderHook(() => ({
      rooms: useSWRAuth(roomKeys.key(), rooms),
      inbox: useSWRAuth(messageKeys.messagesInbox(false), inbox),
      invalidate: useInvalidateMatching(),
    }));
    await waitFor(() => {
      expect(readers.result.current.rooms.data).toEqual(["rooms"]);
      expect(readers.result.current.inbox.data).toEqual(["inbox"]);
    });
    await act(async () => {
      await readers.result.current.invalidate(() => true);
    });
    expect(rooms).toHaveBeenCalledTimes(2);
    expect(inbox).toHaveBeenCalledTimes(2);
  });

  it("matches nothing while the session is being confirmed", async () => {
    const { result } = renderHook(() => ({
      cache: useSWRConfig().cache,
      invalidate: useInvalidateMatching(),
    }));
    result.current.cache.set(own("students-list"), { data: ["kept"] });
    act(() => suspendSWRSession());
    await act(async () => {
      await result.current.invalidate(() => true, { clear: true });
    });
    expect(result.current.cache.get(own("students-list"))?.data).toEqual([
      "kept",
    ]);
  });
});

describe("seed helpers", () => {
  type Row = { thread_id: string; subject: string };
  const derive = (rows: Row[]) => rows.find((row) => row.thread_id === "t1");

  it("seeds a target from current source data and drops it with the source", async () => {
    const inboxKey = messageKeys.messagesInbox(false);
    const threadKey = messageKeys.messageThread("t1");
    const tools = renderHook(() => ({
      inbox: useSWRAuth(inboxKey, async () => [
        { thread_id: "t1", subject: "Ausflug" },
      ]),
      seedFrom: useSWRSeedFrom(),
      clear: useTenantMutateMatching([inboxKey, threadKey]),
    }));
    await waitFor(() => expect(tools.result.current.inbox.data).toBeDefined());
    const seed = tools.result.current.seedFrom(inboxKey, threadKey, derive);
    expect(seed).toEqual({ thread_id: "t1", subject: "Ausflug" });

    // The seed shows under the target's reader without a request.
    const fetcher = vi.fn(async () => ({ thread_id: "t1", subject: "neu" }));
    const reader = renderHook(() =>
      useSWRAuth(threadKey, fetcher, {
        fallbackData: seed,
        revalidateOnMount: false,
      }),
    );
    expect(reader.result.current.data).toBe(seed);

    // A drop of the source ends the seed: nothing older is seeded or shown.
    await act(async () => {
      await tools.result.current.clear({ clear: true });
    });
    expect(
      tools.result.current.seedFrom(inboxKey, threadKey, derive),
    ).toBeUndefined();
    expect(reader.result.current.data).not.toBe(seed);
  });

  it("writes server seeds without replacing an entry a reader holds", async () => {
    const { result } = renderHook(() => useSWRConfig().cache);
    result.current.set(own("held"), { data: "reader" });
    act(() => {
      writeSWRSeeds({ [own("held")]: "seed", [own("fresh")]: "seed" });
    });
    await waitFor(() =>
      expect(result.current.get(own("fresh"))?.data).toBe("seed"),
    );
    expect(result.current.get(own("held"))?.data).toBe("reader");
  });
});

// #5007, third review: options and configuration objects are the fifth way
// into the cache. Each public reader, mutation and boundary copies an
// allowlist; nothing else reaches SWR, whatever the static type let through.
describe("option allowlists of ~/lib/swr", () => {
  it("keeps the reviewed reader and mutation options", () => {
    expect([...READER_OPTION_KEYS].sort()).toEqual([
      "dedupingInterval",
      "errorRetryCount",
      "fallbackData",
      "focusThrottleInterval",
      "isPaused",
      "keepPreviousData",
      "onError",
      "onSuccess",
      "refreshInterval",
      "revalidateIfStale",
      "revalidateOnFocus",
      "revalidateOnMount",
      "revalidateOnReconnect",
      "shouldRetryOnError",
    ]);
    expect([...MUTATION_OPTION_KEYS].sort()).toEqual([
      "optimisticData",
      "populateCache",
      "revalidate",
      "rollbackOnError",
      "throwOnError",
    ]);
  });

  // Every key SWR's configuration knows, plus the hook-only ones: a key
  // outside the allowlist that slipped through would fail here.
  const swrKeys = [
    ...new Set([
      ...Object.keys(SWRConfig.defaultValue),
      "use",
      "provider",
      "fallback",
      "fetcher",
      "compare",
      "onErrorRetry",
      "suspense",
      "strictServerPrefetchWarning",
    ]),
  ].filter((name) => !(READER_OPTION_KEYS as readonly string[]).includes(name));

  it.each(swrKeys)("refuses the reader option %s", (name) => {
    expect(() =>
      renderHook(() =>
        useSWRAuth(roomKeys.key(), async () => [], {
          [name]: () => undefined,
        } as never),
      ),
    ).toThrow(`takes no option '${name}'`);
  });

  it("runs no middleware from the options", () => {
    const middleware = vi.fn<Middleware>((next) => next);
    // Typed wide, as a caller could build it; no cast at the call.
    const options: { refreshInterval: number; use: Middleware[] } = {
      refreshInterval: 0,
      use: [middleware],
    };
    expect(() =>
      renderHook(() => useSWRAuth(roomKeys.key(), async () => [], options)),
    ).toThrow("takes no option 'use'");
    expect(middleware).not.toHaveBeenCalled();
  });

  it("passes only the seed to SWR, no middleware or provider", async () => {
    const middleware = vi.fn<Middleware>((next) => next);
    const provider = vi.fn(() => new Map());
    const seed = {
      fallback: {},
      cacheData: {},
      use: [middleware],
      provider,
    };
    function Reader() {
      useSWRAuth(roomKeys.key(), async () => ["server"]);
      return null;
    }
    render(
      <SWRSeedBoundary seed={seed}>
        <Reader />
      </SWRSeedBoundary>,
    );
    await waitFor(() => expect(middleware).not.toHaveBeenCalled());
    expect(provider).not.toHaveBeenCalled();
  });

  it("hands a reader's callbacks the value alone, never SWR's configuration", async () => {
    const onSuccess = vi.fn();
    renderHook(() =>
      useSWRAuth(roomKeys.key(), async () => ["server"], { onSuccess }),
    );
    await waitFor(() => expect(onSuccess).toHaveBeenCalled());
    expect(onSuccess.mock.calls[0]).toEqual([["server"]]);
  });

  // Production drops what development refuses: no option outside the
  // allowlist reaches SWR there either.
  function inProduction() {
    vi.stubEnv("NODE_ENV", "production");
    onTestFinished(() => {
      vi.unstubAllEnvs();
    });
  }

  it("drops a refused reader option in production", () => {
    inProduction();
    const middleware = vi.fn<Middleware>((next) => next);
    const options = { refreshInterval: 0, use: [middleware] };
    renderHook(() => useSWRAuth(roomKeys.key(), async () => [], options));
    expect(middleware).not.toHaveBeenCalled();
  });

  async function boundReader() {
    const reader = renderHook(() =>
      useSWRAuth(roomKeys.key(), async () => ["server"], {
        revalidateIfStale: false,
      }),
    );
    await waitFor(() => expect(reader.result.current.data).toEqual(["server"]));
    const { result } = renderHook(() => useSWRConfig().cache);
    return { reader, cache: result.current };
  }

  it("lets a bound clear take only revalidate and throwOnError", async () => {
    const { reader, cache } = await boundReader();
    await act(async () => {
      await reader.result.current.mutate(undefined, {
        revalidate: false,
        throwOnError: true,
      });
    });
    expect(cache.get(own(roomKeys.key()))?.data).toBeUndefined();
    const middleware = vi.fn<Middleware>((next) => next);
    const options = {
      revalidate: false,
      populateCache: () => ["raw-populate"],
      use: [middleware],
    };
    expect(() => reader.result.current.mutate(undefined, options)).toThrow(
      "takes no option 'populateCache'",
    );
  });

  it("clears without the refused options in production", async () => {
    const { reader, cache } = await boundReader();
    inProduction();
    const options = {
      revalidate: false,
      populateCache: () => ["raw-populate"],
      optimisticData: ["raw-optimistic"],
    };
    await act(async () => {
      await reader.result.current.mutate(undefined, options);
    });
    // Neither populateCache nor optimisticData wrote: the entry is cleared.
    expect(cache.get(own(roomKeys.key()))?.data).toBeUndefined();
  });

  it("refuses a mutation option outside the allowlist on the bound mutate", async () => {
    const { reader } = await boundReader();
    const options = { revalidate: false, extra: 1 };
    expect(() => reader.result.current.mutate(["saved"], options)).toThrow(
      "takes no option 'extra'",
    );
  });

  it("drops a refused mutation option in production", async () => {
    const { reader, cache } = await boundReader();
    inProduction();
    const options = { revalidate: false, populateCache: true, extra: 1 };
    await act(async () => {
      await reader.result.current.mutate(["saved"], options);
    });
    expect(cache.get(own(roomKeys.key()))?.data).toEqual(["saved"]);
  });

  it("refuses a mutation option outside the allowlist", () => {
    const { result } = renderHook(() => useTenantMutate());
    const options = { revalidate: false, optimisticData: ["x"], extra: 1 };
    expect(() => result.current(roomKeys.key(), ["saved"], options)).toThrow(
      "takes no option 'extra'",
    );
  });
});
