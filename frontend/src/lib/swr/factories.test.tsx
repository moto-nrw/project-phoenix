import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useSWRConfig } from "swr";
import { SWRGlobalState } from "swr/_internal";
import { roomKeys } from "./rooms-keys";
import { groupKeys } from "./groups-keys";
import { staffKeys } from "./staff-keys";
import { meKeys } from "./me-keys";
import { messageKeys, operatorProvisioningKeys } from "./portal-keys";
import {
  _setSWRCleanupForTests,
  resetSWRScope,
  suspendSWRSession,
  resumeSWRSession,
} from "./scope";
import {
  useSWRAuth,
  useSWRCacheKey,
  useSWRPreload,
  useInvalidateFamily,
  useTenantMutate,
  useTenantMutateMatching,
} from "./index";

vi.unmock("swr");
const { owner } = vi.hoisted(() => ({
  owner: {
    id: "account-7",
    scope: "tenant",
    tenantId: 42,
    orgId: 84,
    slug: "school-a" as string | null,
  },
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: owner, expires: "2099-01-01" },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => owner.slug,
}));

beforeEach(() => {
  resetSWRScope({ revalidate: false });
  resumeSWRSession();
  Object.assign(owner, {
    id: "account-7",
    scope: "tenant",
    tenantId: 42,
    orgId: 84,
    slug: "school-a",
  });
});

afterEach(() => _setSWRCleanupForTests(true));

const factories = [roomKeys, groupKeys, staffKeys, meKeys];

describe("shared resource factories", () => {
  it("has one canonical key per resource and parameter set", () => {
    expect(roomKeys.key({ page: 1, pageSize: 500 })).toBe(
      roomKeys.key({ pageSize: 500, page: 1 }),
    );
    expect(roomKeys.key({ search: undefined })).toBe(roomKeys.key());
    expect(groupKeys.key({ pageSize: 500, page: 1 })).toBe(
      groupKeys.key({ page: 1, pageSize: 500 }),
    );
    expect(staffKeys.key()).toBe(staffKeys.key({}, { strict: false }));
    expect(new Set(factories.map((f) => f.key())).size).toBe(4);
    expect(roomKeys.key({ includeSystem: true })).not.toBe(roomKeys.key());
    expect(roomKeys.key({ pageSize: 500 })).not.toBe(
      roomKeys.key({ pageSize: 1000 }),
    );
    expect(staffKeys.teachersKey()).not.toBe(staffKeys.key());
  });

  it.each(["parent", "org", "platform", "school", "caterer", ""])(
    "puts the portal boundary and account in every %s portal key",
    async (scope) => {
      Object.assign(owner, { scope, slug: null });
      for (const factory of factories) {
        const fetcher = vi.fn(async () => ({ name: scope }));
        const { result, unmount } = renderHook(() => ({
          reader: useSWRAuth(factory.key(), fetcher),
          key: useSWRCacheKey()(factory.key()),
        }));
        await waitFor(() =>
          expect(result.current.reader.data).toEqual({ name: scope }),
        );
        expect(result.current.key).toBe(
          scope === "platform"
            ? `operator:account-7:${factory.key()}`
            : `${scope === "org" ? "traeger:84" : `${scope || "tenant"}:42`}:account-7:${factory.key()}`,
        );
        unmount();
      }
    },
  );

  it("uses the same scoped identity for preload, read and optimistic mutation", async () => {
    const fetcher = vi.fn(async () => ["server"]);
    const key = roomKeys.key();
    const tools = renderHook(() => ({
      preload: useSWRPreload(),
      mutate: useTenantMutate(),
    }));
    await act(async () => {
      await tools.result.current.preload(key, fetcher);
    });
    const reader = renderHook(() =>
      useSWRAuth(key, fetcher, { revalidateIfStale: false }),
    );
    await waitFor(() => expect(reader.result.current.data).toEqual(["server"]));
    expect(fetcher).toHaveBeenCalledTimes(1);
    let resolve!: (value: string[]) => void;
    const request = new Promise<string[]>((r) => {
      resolve = r;
    });
    let update!: Promise<unknown>;
    await act(async () => {
      update = tools.result.current.mutate(key, request, {
        optimisticData: ["optimistic"],
        revalidate: false,
      });
    });
    expect(reader.result.current.data).toEqual(["optimistic"]);
    await act(async () => {
      resolve(["saved"]);
      await update;
    });
    expect(reader.result.current.data).toEqual(["saved"]);
  });

  it("refreshes all parameter sets after a family write, not unrelated resources", async () => {
    const first = vi.fn(async () => ["first"]);
    const second = vi.fn(async () => ["second"]);
    const unrelated = vi.fn(async () => ["group"]);
    const readers = renderHook(() => ({
      first: useSWRAuth(roomKeys.key({ page: 1 }), first),
      second: useSWRAuth(roomKeys.key({ page: 2, pageSize: 17 }), second),
      unrelated: useSWRAuth(groupKeys.key(), unrelated),
      refresh: useTenantMutateMatching([roomKeys.family]),
    }));
    await waitFor(() => {
      expect(readers.result.current.first.data).toEqual(["first"]);
      expect(readers.result.current.second.data).toEqual(["second"]);
      expect(readers.result.current.unrelated.data).toEqual(["group"]);
    });
    await act(async () => {
      await readers.result.current.refresh();
    });
    expect(first).toHaveBeenCalledTimes(2);
    expect(second).toHaveBeenCalledTimes(2);
    expect(unrelated).toHaveBeenCalledTimes(1);
  });

  it("invalidates only its scoped family, including infinite and preload-only keys", async () => {
    const { result } = renderHook(() => ({
      cache: useSWRConfig().cache,
      invalidate: useInvalidateFamily(),
    }));
    const own = `school-a:account-7:${roomKeys.key()}`;
    const keys = [
      own,
      `$inf$${own}`,
      `school-a:account-8:${roomKeys.key()}`,
      `school-b:account-7:${roomKeys.key()}`,
      `school-a:account-7:${groupKeys.key()}`,
    ];
    for (const key of keys) result.current.cache.set(key, { data: [key] });
    const preloads = SWRGlobalState.get(result.current.cache)![3];
    preloads[`${own}-preload-only`] = Promise.resolve(["preloaded"]);
    await act(async () => {
      await result.current.invalidate(roomKeys.family, { clear: true });
    });
    expect(result.current.cache.get(own)?.data).toBeUndefined();
    expect(result.current.cache.get(`$inf$${own}`)?.data).toBeUndefined();
    expect(preloads[`${own}-preload-only`]).toBeUndefined();
    for (const key of keys.slice(2))
      expect(result.current.cache.get(key)?.data).toEqual([key]);
  });

  it.each(["answer", "failure"] as const)(
    "resolves a late preload %s without exposing an old generation",
    async (kind) => {
      let resolve!: (value: string) => void;
      let reject!: (error: Error) => void;
      const pending = new Promise<string>((yes, no) => {
        resolve = yes;
        reject = no;
      });
      const { result } = renderHook(() => useSWRPreload());
      const observed = result
        .current(roomKeys.key(), () => pending)
        .then(
          (value) => ({ value }),
          (error) => ({ error }),
        );
      act(() => resetSWRScope({ revalidate: false }));
      await act(async () => {
        if (kind === "answer") resolve("old room");
        else reject(new Error("old failure"));
        expect(await observed).toEqual({ value: undefined });
      });
    },
  );

  it("does not relabel a completed preload after a generation change without cleanup", async () => {
    _setSWRCleanupForTests(false);
    const key = roomKeys.key();
    const tools = renderHook(() => useSWRPreload());
    await tools.result.current(key, async () => "old room");
    act(() => resetSWRScope({ revalidate: false }));
    await tools.result.current(key, async () => "unused preload");
    const seen: (string | undefined)[] = [];
    const fresh = vi.fn(async () => "fresh room");
    const reader = renderHook(() => {
      const state = useSWRAuth(key, fresh);
      seen.push(state.data);
      return state;
    });
    await waitFor(() => expect(reader.result.current.data).toBe("fresh room"));
    expect(seen).not.toContain("old room");
    expect(fresh).toHaveBeenCalledTimes(1);
  });

  it("does not expose a cached preload failure after a generation change without cleanup", async () => {
    _setSWRCleanupForTests(false);
    const key = roomKeys.key();
    const tools = renderHook(() => useSWRPreload());
    const old = new Error("cached old failure");
    await expect(
      tools.result.current(key, async () => {
        throw old;
      }),
    ).rejects.toBe(old);
    act(() => resetSWRScope({ revalidate: false }));
    await expect(
      tools.result.current(key, async () => "new room"),
    ).resolves.toBeUndefined();
  });

  it("preserves a preload failure of the current generation", async () => {
    const error = new Error("current failure");
    const { result } = renderHook(() => useSWRPreload());
    await expect(
      result.current(roomKeys.key(), async () => {
        throw error;
      }),
    ).rejects.toBe(error);
  });

  it("does not start an intent preload from a retained or suspended session", async () => {
    const fetcher = vi.fn(async () => "fresh");
    const { result } = renderHook(() => useSWRPreload());
    const retained = result.current;
    act(() => resetSWRScope({ revalidate: false }));
    await retained(roomKeys.key(), fetcher);
    expect(fetcher).toHaveBeenCalledTimes(0);
    act(() => suspendSWRSession());
    await result.current(roomKeys.key(), fetcher);
    expect(fetcher).toHaveBeenCalledTimes(0);
  });

  it("shares operator keys across list/detail consumers and separates every messaging dimension", () => {
    // The organization page and its school page share one key, not an alias.
    expect(operatorProvisioningKeys).not.toHaveProperty("organizationSchools");
    expect(operatorProvisioningKeys.organizationSchoolSummaries("7")).toBe(
      "operator-organization-school-summaries-7",
    );
    expect(messageKeys.groupMessageAudience("group", "7")).not.toBe(
      messageKeys.groupMessageAudience("group", "70"),
    );
    expect(messageKeys.messagesInbox(true)).not.toBe(
      messageKeys.messagesInbox(false),
    );
  });
});
