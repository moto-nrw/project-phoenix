import { act, renderHook, waitFor } from "@testing-library/react";
import type { Session } from "next-auth";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SWRConfig } from "~/lib/swr/testing";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { ToastProvider } from "~/contexts/ToastContext";
import { useTraegerSchools } from "./use-traeger-schools";

vi.unmock("swr");
const { fetchSchools, sessionState } = vi.hoisted(() => ({
  fetchSchools: vi.fn(),
  sessionState: { data: null as Session | null },
}));
vi.mock("~/lib/traeger-api", () => ({ fetchTraegerSchools: fetchSchools }));
vi.mock("~/lib/tenant-context", () => ({ useTenantSlugSafe: () => null }));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: sessionState.data,
    status: sessionState.data ? "authenticated" : "unauthenticated",
  }),
}));

function signIn(id = "A", orgId = 7, scope = "org") {
  sessionState.data = {
    user: { id, orgId, scope },
    expires: "2099-01-01",
  } as Session;
}
const schools = [
  { id: "1", name: "Am Bach", slug: "bach", subdomain: "bach", city: "Bonn" },
];
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
  return { cache, wrapper: Wrapper };
}
beforeEach(() => {
  signIn();
  fetchSchools.mockReset().mockResolvedValue(schools);
});

describe("shared Träger schools", () => {
  it("shares one request across readers and remounts, and revalidates explicitly", async () => {
    const options = setup();
    const first = renderHook(
      () => [useTraegerSchools(), useTraegerSchools()],
      options,
    );
    await waitFor(() =>
      expect(first.result.current[0]?.schools).toEqual(schools),
    );
    expect(fetchSchools).toHaveBeenCalledTimes(1);
    expect([...options.cache.keys()]).toContain("traeger:7:A:traeger-schools");
    first.unmount();
    const next = renderHook(() => useTraegerSchools(), options);
    expect(next.result.current.schools).toEqual(schools);
    expect(fetchSchools).toHaveBeenCalledTimes(1);
    await act(async () => {
      await next.result.current.mutate();
    });
    expect(fetchSchools).toHaveBeenCalledTimes(2);
  });

  it.each(["account", "organization", "logout"])(
    "hides old schools on %s change",
    async (change) => {
      const options = setup();
      const view = renderHook(() => useTraegerSchools(), options);
      await waitFor(() => expect(view.result.current.schools).toEqual(schools));
      fetchSchools.mockImplementation(() => new Promise(() => undefined));
      if (change === "logout") sessionState.data = null;
      else
        signIn(
          change === "account" ? "B" : "A",
          change === "organization" ? 8 : 7,
        );
      view.rerender();
      expect(view.result.current.schools).toBeNull();
    },
  );

  it.each(["tenant", "parent", "school", "platform"])(
    "does not fetch with a %s session",
    async (scope) => {
      signIn("A", 7, scope);
      const view = renderHook(() => useTraegerSchools(), setup());
      await act(async () => undefined);
      expect(view.result.current.schools).toBeNull();
      expect(fetchSchools).not.toHaveBeenCalled();
    },
  );
});
