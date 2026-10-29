import { act, render, renderHook, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// The matching contract on the real SWR 2.5.1: the hook acts on the cache.
vi.unmock("swr");

const { sessionState, tenantState } = vi.hoisted(() => ({
  sessionState: { signedIn: true },
  tenantState: { slug: "school-a" as string | null },
}));

vi.mock("next-auth/react", () => ({
  useSession: () =>
    sessionState.signedIn
      ? {
          data: { user: { id: "user-1" }, expires: "2099-01-01" },
          status: "authenticated",
        }
      : { data: null, status: "unauthenticated" },
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => tenantState.slug,
}));

// Boundary-level test: the shared wrapper may import the package directly.
import { unstable_serialize, useSWRConfig } from "swr";
import { SWRGlobalState } from "swr/_internal";
import { useSWRAuth, useTenantMutateMatching } from "./hooks";
import { resetSWRScope } from "./scope";

function swrCache() {
  return renderHook(() => useSWRConfig()).result.current.cache;
}

/** Puts data under every key; returns the keys whose data a call removed. */
async function clearedBy(
  substrings: string[],
  keys: string[],
  options: { except?: string[] } = {},
): Promise<string[]> {
  for (const key of keys) swrCache().set(key, { data: `alt ${key}` });
  const { result } = renderHook(() => useTenantMutateMatching(substrings));
  await act(async () => {
    await result.current({ clear: true, ...options });
  });
  return keys.filter((key) => swrCache().get(key)?.data === undefined);
}

beforeEach(() => {
  resetSWRScope({ revalidate: false });
  sessionState.signedIn = true;
  tenantState.slug = "school-a";
});

// Cross-tenant isolation is the bug magnet here. Matching by substring alone
// would invalidate every school and account a device has open in other tabs.
describe("useTenantMutateMatching (#5006)", () => {
  it("matches keys of the current tenant and account whose name contains a substring", async () => {
    expect(
      await clearedBy(
        ["active-supervision", "ogs-students"],
        [
          "school-a:user-1:active-supervision-dashboard-0",
          "school-a:user-1:ogs-students-42",
        ],
      ),
    ).toEqual([
      "school-a:user-1:active-supervision-dashboard-0",
      "school-a:user-1:ogs-students-42",
    ]);
  });

  it("rejects keys of other tenants even when the substring matches", async () => {
    const own = "school-a:user-1:active-supervision-dashboard-0";
    expect(
      await clearedBy(
        ["active-supervision"],
        [
          own,
          "school-b:user-1:active-supervision-dashboard-0",
          "school-c:user-1:active-supervision-visits-1",
        ],
      ),
    ).toEqual([own]);
  });

  it("rejects keys of another account of the same school", async () => {
    const own = "school-a:user-1:active-supervision-dashboard-0";
    expect(
      await clearedBy(
        ["active-supervision"],
        [own, "school-a:user-2:active-supervision-dashboard-0"],
      ),
    ).toEqual([own]);
  });

  it("rejects keys that match the tenant but no substring", async () => {
    const own = "school-a:user-1:ogs-students-1";
    expect(
      await clearedBy(
        ["ogs-students"],
        [
          own,
          "school-a:user-1:database-teachers-list",
          "school-a:user-1:students-search-foo",
        ],
      ),
    ).toEqual([own]);
  });

  it("rejects array keys (SWR serializes them without the scope prefix)", async () => {
    const own = "school-a:user-1:foo-1";
    const arrayKey = unstable_serialize(["school-a:user-1", "foo"]);
    expect(await clearedBy(["foo"], [own, arrayKey])).toEqual([own]);
  });

  it("matches the portal and account scope when no tenant context", async () => {
    tenantState.slug = null;
    const own = "tenant:-:user-1:operator-foo-1";
    expect(
      await clearedBy(
        ["operator-foo"],
        [own, "operator-foo-1", "tenant:-:user-1:unrelated-bar"],
      ),
    ).toEqual([own]);
  });

  it("matches nothing without a signed-in session", async () => {
    const key = "school-a:user-1:foo-1";
    swrCache().set(key, { data: "alt" });
    sessionState.signedIn = false;
    const { result } = renderHook(() => useTenantMutateMatching(["foo"]));

    let answer: unknown;
    await act(async () => {
      answer = await result.current({ clear: true });
    });

    expect(answer).toEqual([]);
    expect(swrCache().get(key)?.data).toBe("alt");
  });

  it("leaves the keys named in `except` out entirely", async () => {
    // A decision clears every other filter and view variant of its list;
    // the variant it was made in already shows its result (#4988).
    const decided = 'school-a:user-1:request-feed:7:open:[""]';
    const [, , FETCH, PRELOAD] = SWRGlobalState.get(swrCache())!;
    FETCH[decided] = [Promise.resolve("läuft"), 1];
    PRELOAD[decided] = Promise.resolve("vorgeladen");

    expect(
      await clearedBy(
        ["request-feed:7:"],
        [
          'school-a:user-1:request-feed:7:open:["Mia"]',
          'school-a:user-1:request-feed:7:history:[""]',
          decided,
          'school-b:user-1:request-feed:7:open:["Mia"]',
        ],
        { except: ['request-feed:7:open:[""]'] },
      ),
    ).toEqual([
      'school-a:user-1:request-feed:7:open:["Mia"]',
      'school-a:user-1:request-feed:7:history:[""]',
    ]);
    expect(decided in FETCH).toBe(true);
    expect(decided in PRELOAD).toBe(true);
  });

  it("revalidates mounted matching readers and keeps their data until the answer", async () => {
    let answer = "alt";
    const fetcher = vi.fn(() => Promise.resolve(answer));
    function Reader() {
      const { data } = useSWRAuth("ogs-students-1", fetcher);
      return <p data-testid="reader">{data ?? "leer"}</p>;
    }
    render(<Reader />);
    expect(await screen.findByText("alt")).toBeInTheDocument();
    const { result } = renderHook(() =>
      useTenantMutateMatching(["ogs-students-"]),
    );

    answer = "neu";
    await act(async () => {
      await result.current();
    });

    expect(await screen.findByText("neu")).toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("clears without fetching when called with { clear: true }", async () => {
    const fetcher = vi.fn(() => Promise.resolve("alt"));
    function Reader() {
      const { data } = useSWRAuth("timetable-sammel-vertretung-x", fetcher, {
        keepPreviousData: false,
      });
      return <p data-testid="reader">{data ?? "leer"}</p>;
    }
    render(<Reader />);
    expect(await screen.findByText("alt")).toBeInTheDocument();
    const { result } = renderHook(() =>
      useTenantMutateMatching(["timetable-sammel-vertretung-"]),
    );

    await act(async () => {
      await result.current({ clear: true });
    });

    expect(screen.getByTestId("reader")).toHaveTextContent("leer");
    expect(fetcher).toHaveBeenCalledTimes(1);
  });
});
