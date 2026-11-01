import { act, renderHook, waitFor } from "@testing-library/react";
import type { Session } from "next-auth";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { SWRConfig } from "~/lib/swr/testing";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { resumeSWRSession } from "~/lib/swr/scope";
import {
  HOLIDAY_PERIOD,
  HOLIDAY_SCHOOLS,
} from "~/test/fixtures/holiday-registrations";
import { useTraegerSchools } from "./use-traeger-schools";
import {
  useTraegerHolidayPeriod,
  useTraegerHolidayPeriods,
  useTraegerSettingsSchema,
} from "./use-traeger-resources";

vi.unmock("swr");
const { api, identity, announce } = vi.hoisted(() => ({
  api: {
    fetchHolidayPeriods: vi.fn(),
    fetchHolidayPeriod: vi.fn(),
    fetchTraegerSchools: vi.fn(),
    fetchTraegerSettingsSchema: vi.fn(),
  },
  identity: { session: null as Session | null },
  announce: vi.fn(),
}));
vi.mock("~/lib/traeger-api", () => api);
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: identity.session,
    status: identity.session ? "authenticated" : "unauthenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({ useTenantSlugSafe: () => null }));
vi.mock("~/lib/session-cache", () => ({ announceSessionChange: announce }));
const schema = { tabs: [] };
function signIn(id = "A", orgId: number | undefined = 7, scope = "org") {
  identity.session = {
    user: { id, scope, orgId },
    expires: "2099-01-01",
  } as Session;
}
function setup() {
  const cache = new Map();
  function Wrapper({ children }: Readonly<{ children: React.ReactNode }>) {
    return (
      <SWRConfig value={{ provider: () => cache }}>
        <ToastProvider>
          <SWRScopeBoundary />
          {children}
        </ToastProvider>
      </SWRConfig>
    );
  }
  return { wrapper: Wrapper, cache };
}
const resources = [
  {
    name: "schools",
    key: "traeger-schools",
    fetcher: api.fetchTraegerSchools,
    value: HOLIDAY_SCHOOLS,
    read: () => {
      const response = useTraegerSchools();
      return { data: response.schools, mutate: response.mutate };
    },
  },
  {
    name: "period list",
    key: "traeger-holiday-period-list",
    fetcher: api.fetchHolidayPeriods,
    value: [HOLIDAY_PERIOD],
    read: useTraegerHolidayPeriods,
  },
  {
    name: "period detail",
    key: `traeger-holiday-period-detail-${HOLIDAY_PERIOD.id}`,
    fetcher: api.fetchHolidayPeriod,
    value: HOLIDAY_PERIOD,
    read: () => useTraegerHolidayPeriod(HOLIDAY_PERIOD.id),
  },
  {
    name: "settings schema",
    key: "traeger-settings-schema",
    fetcher: api.fetchTraegerSettingsSchema,
    value: schema,
    read: useTraegerSettingsSchema,
  },
];
beforeEach(() => {
  signIn();
  announce.mockClear();
  for (const resource of resources)
    resource.fetcher.mockReset().mockResolvedValue(resource.value);
});
afterEach(() => resumeSWRSession());

describe.each(resources)("Träger $name", (resource) => {
  it("shares the response and refreshes all readers through bound mutate", async () => {
    const options = setup();
    const view = renderHook(() => [resource.read(), resource.read()], options);
    await waitFor(() =>
      expect(view.result.current[0]?.data).toEqual(resource.value),
    );
    expect(resource.fetcher).toHaveBeenCalledTimes(1);
    expect([...options.cache.keys()]).toContain(`traeger:7:A:${resource.key}`);
    await act(async () => {
      await view.result.current[0]?.mutate();
    });
    expect(resource.fetcher).toHaveBeenCalledTimes(2);
    expect(view.result.current[1]?.data).toEqual(resource.value);
  });

  it.each([
    "account",
    "organization",
    "logout",
    "other-tab-account",
    "other-tab-organization",
    "other-tab-logout",
  ])(
    "shows no old response after %s, including late answers",
    async (change) => {
      const options = setup();
      const shown: unknown[] = [];
      const view = renderHook(() => {
        const response = resource.read();
        shown.push(response.data);
        return response;
      }, options);
      await waitFor(() =>
        expect(view.result.current.data).toEqual(resource.value),
      );
      let finish!: (value: unknown) => void;
      resource.fetcher.mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            finish = resolve;
          }),
      );
      let pending!: Promise<unknown>;
      act(() => {
        pending = view.result.current.mutate();
      });
      await waitFor(() => expect(resource.fetcher).toHaveBeenCalledTimes(2));
      resource.fetcher.mockImplementation(() => new Promise(() => undefined));
      shown.length = 0;
      if (change.startsWith("other-tab")) {
        act(() => {
          globalThis.dispatchEvent(
            new StorageEvent("storage", {
              key: "moto:personal-storage-owner",
              newValue: change.endsWith("logout")
                ? null
                : change === "other-tab-organization"
                  ? "8:A"
                  : "7:B",
            }),
          );
        });
        expect(announce).toHaveBeenCalledTimes(1);
        expect(resource.fetcher).toHaveBeenCalledTimes(2);
        if (change === "other-tab-organization") {
          // A fresh lookup of the old organization is not confirmation.
          signIn("A", 7);
          view.rerender();
          expect(view.result.current.data ?? null).toBeNull();
          expect(resource.fetcher).toHaveBeenCalledTimes(2);
        }
      } else {
        if (change === "logout") identity.session = null;
        else
          signIn(
            change === "account" ? "B" : "A",
            change === "organization" ? 8 : 7,
          );
        view.rerender();
      }
      await act(async () => {
        finish(resource.value);
        await pending;
      });
      expect(view.result.current.data ?? null).toBeNull();
      expect(shown.every((data) => data == null)).toBe(true);
      if (change.startsWith("other-tab")) {
        if (change.endsWith("logout")) identity.session = null;
        else if (change === "other-tab-organization") signIn("A", 8);
        else signIn("B");
        view.rerender();
        expect(view.result.current.data ?? null).toBeNull();
        expect(resource.fetcher).toHaveBeenCalledTimes(
          change.endsWith("logout") ? 2 : 3,
        );
      }
    },
  );

  it.each(["tenant", "parent", "school", "platform", "caterer"])(
    "never fetches metadata under a %s session",
    async (scope) => {
      signIn("A", 7, scope);
      const view = renderHook(() => resource.read(), setup());
      await act(async () => undefined);
      expect(view.result.current.data ?? null).toBeNull();
      expect(resource.fetcher).not.toHaveBeenCalled();
    },
  );

  it("fails closed without an organization", async () => {
    signIn();
    identity.session = {
      ...identity.session!,
      user: { ...identity.session!.user, orgId: undefined },
    };
    const view = renderHook(() => resource.read(), setup());
    await act(async () => undefined);
    expect(view.result.current.data ?? null).toBeNull();
    expect(resource.fetcher).not.toHaveBeenCalled();
  });
});
