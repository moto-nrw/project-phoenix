import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useSWRConfig } from "swr";
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
  "portal-keys": () => import("./portal-keys"),
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
    "useDiscardRefusedEntity",
    "useImmutableSWR",
    "useInvalidateFamily",
    "useInvalidateMatching",
    "useSWRAuth",
    "useSWRCacheKey",
    "useSWRPreload",
    "useSWRWithId",
    "useScopedSWRKey",
    "useTenantMutate",
    "useTenantMutateMatching",
  ],
  mutate: ["mutate"],
  "config-boundaries": ["IsolatedSWRCache", "SWRSeedBoundary"],
  "scope-boundary": ["SWRScopeBoundary"],
  "portal-keys": [
    "announcementKeys",
    "dashboardKeys",
    "databaseKeys",
    "homeLayoutKeys",
    "messageKeys",
    "operatorAnnouncementKeys",
    "operatorBillingKeys",
    "operatorInvitationKeys",
    "operatorProvisioningKeys",
    "parentKeys",
    "payrollKeys",
    "schoolSetupKeys",
    "staffMergeKeys",
    "staffOnboardingKeys",
    "studentKeys",
    "timetableKeys",
  ],
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
