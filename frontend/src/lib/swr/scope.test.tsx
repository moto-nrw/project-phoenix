import { act, render, renderHook, screen } from "@testing-library/react";
import type { Session } from "next-auth";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// The cache contract needs the real SWR, not the global test mock.
vi.unmock("swr");

const { mockUseSession, mockAnnounce } = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockAnnounce: vi.fn(),
}));

vi.mock("next-auth/react", () => ({ useSession: mockUseSession }));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "school-a",
}));

vi.mock("~/lib/session-cache", () => ({
  announceSessionChange: mockAnnounce,
}));

// Boundary-level test: the shared wrapper may import the package directly.
import { mutate, preload, useSWRConfig } from "swr";
import { ApiError } from "~/lib/api-error";
import {
  addListOverlay,
  startListFetch,
  useListOverlays,
} from "~/lib/list-overlays";
import { _resetPersonalStorageLeave } from "~/lib/personal-storage";
import {
  useDiscardRefusedEntity,
  useImmutableSWR,
  useSWRAuth,
  useTenantMutate,
  useTenantMutateMatching,
} from "./hooks";
import { resetSWRScope, resumeSWRSession } from "./scope";
import { SWRScopeBoundary } from "./scope-boundary";
import { expectIdleRenderBudget } from "~/test/render-budget";
import { releaseFakeTimers } from "~/test/clock";

const OWNER_KEY = "moto:personal-storage-owner";

function signedIn(
  id: string,
  user: Partial<Session["user"]> = {},
): { data: Session; status: "authenticated" } {
  return {
    data: { user: { id, tenantId: 1, ...user }, expires: "2099-01-01" },
    status: "authenticated",
  };
}

const signedOut = { data: null, status: "unauthenticated" as const };

function setSession(session: unknown) {
  mockUseSession.mockReturnValue(session);
}

/** Everything a reader rendered, in order. */
let rendered: unknown[] = [];

function Reader({
  name,
  fetcher,
}: Readonly<{ name: string; fetcher: () => Promise<string> }>) {
  const { data, error } = useSWRAuth<string>(name, fetcher);
  rendered.push(data);
  return (
    <p data-testid={`reader-${name}`}>{data ?? (error ? "Fehler" : "leer")}</p>
  );
}

function Page({
  fetcher,
  name = "roster",
}: Readonly<{ fetcher: () => Promise<string>; name?: string }>) {
  return (
    <>
      <SWRScopeBoundary expectedTenantId={1} />
      <Reader name={name} fetcher={fetcher} />
    </>
  );
}

/** Data of whichever account the "cookie" currently belongs to. */
function accountFetcher() {
  return vi.fn(() => {
    const session = mockUseSession() as { data: Session | null };
    return Promise.resolve(`Daten von ${session.data?.user.id ?? "-"}`);
  });
}

function otherTabBinds(owner: string | null) {
  globalThis.dispatchEvent(
    new StorageEvent("storage", { key: OWNER_KEY, newValue: owner }),
  );
}

function cacheKeys(): string[] {
  const { result } = renderHook(() => useSWRConfig());
  return [...result.current.cache.keys()];
}

beforeEach(() => {
  resetSWRScope({ revalidate: false });
  mockAnnounce.mockClear();
  rendered = [];
  setSession(signedIn("A"));
});

afterEach(() => {
  resumeSWRSession();
  _resetPersonalStorageLeave();
});

describe("SWR cache scope (#5006)", () => {
  it("scopes every wrapper key to school and account", async () => {
    render(<Page fetcher={accountFetcher()} />);

    expect(await screen.findByText("Daten von A")).toBeInTheDocument();
    expect(cacheKeys()).toContain("school-a:A:roster");
  });

  it("shows no previous data under the key of the next account", async () => {
    const fetcher = accountFetcher();
    const view = render(<Page fetcher={fetcher} />);
    expect(await screen.findByText("Daten von A")).toBeInTheDocument();

    rendered = [];
    setSession(signedIn("B"));
    view.rerender(<Page fetcher={fetcher} />);

    // Anker: die neue Generation lädt ihre eigenen Daten.
    expect(await screen.findByText("Daten von B")).toBeInTheDocument();
    expect(rendered).not.toContain("Daten von A");
    expect(rendered[0]).toBeUndefined();
  });

  it("drops the first tab's data when a second tab signs in another account of the same school", async () => {
    const fetcher = accountFetcher();
    const view = render(<Page fetcher={fetcher} />);
    expect(await screen.findByText("Daten von A")).toBeInTheDocument();
    const fetchesBefore = fetcher.mock.calls.length;

    rendered = [];
    // The other tab already holds account B's cookie; this tab's provider
    // still reports A until it reads the cookie again.
    act(() => otherTabBinds("1:B"));

    // Anker: der Leser steht leer da, statt A zu zeigen.
    expect(screen.getByTestId("reader-roster")).toHaveTextContent("leer");
    expect(rendered).not.toContain("Daten von A");
    // Nothing loads with B's cookie under A's keys; the provider is woken.
    expect(fetcher.mock.calls.length).toBe(fetchesBefore);
    expect(mockAnnounce).toHaveBeenCalledTimes(1);

    setSession(signedIn("B"));
    view.rerender(<Page fetcher={fetcher} />);
    expect(await screen.findByText("Daten von B")).toBeInTheDocument();
    expect(rendered).not.toContain("Daten von A");
  });

  it("suspends polling and stale mutation closures until the changed cookie is confirmed", async () => {
    vi.useFakeTimers();
    let cookie = "A";
    const fetcher = vi.fn(() => Promise.resolve(`Daten von ${cookie}`));
    function PollingReader() {
      const { data } = useSWRAuth("poll", fetcher, {
        refreshInterval: 100,
        dedupingInterval: 0,
        // A caller must not override the session-boundary pause.
        isPaused: () => false,
      });
      const immutable = useImmutableSWR("immutable", fetcher);
      return (
        <>
          <p>{data ?? "leer"}</p>
          <p data-testid="immutable">{immutable.data ?? "leer"}</p>
        </>
      );
    }
    const page = (
      <>
        <SWRScopeBoundary />
        <PollingReader />
      </>
    );
    const view = render(page);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(200);
    });
    // Anker: beide Leser haben die Daten des ersten Kontos geladen.
    expect(screen.getAllByText("Daten von A")).toHaveLength(2);
    const single = renderHook(() => useTenantMutate());
    const matching = renderHook(() => useTenantMutateMatching(["poll"]));
    const staleSingle = single.result.current;
    const staleMatching = matching.result.current;
    const calls = fetcher.mock.calls.length;
    cookie = "B";
    await act(async () => {
      otherTabBinds("1:B");
      // These callbacks run before React has committed new hook options.
      await staleSingle("poll");
      await staleMatching();
      await mutate("school-a:A:poll");
      await vi.advanceTimersByTimeAsync(500);
    });
    expect(screen.getAllByText("leer")).toHaveLength(2);
    expect(fetcher).toHaveBeenCalledTimes(calls);

    // An unrelated render and even a stale lookup must not reopen readers.
    view.rerender(
      <>
        <SWRScopeBoundary />
        <PollingReader />
      </>,
    );
    setSession(signedIn("A"));
    view.rerender(
      <>
        <SWRScopeBoundary />
        <PollingReader />
      </>,
    );
    await act(async () => {
      await vi.advanceTimersByTimeAsync(200);
    });
    expect(fetcher).toHaveBeenCalledTimes(calls);
    expect(screen.getByTestId("immutable")).toHaveTextContent("leer");

    setSession({ data: signedIn("B").data, status: "loading" });
    view.rerender(
      <>
        <SWRScopeBoundary />
        <PollingReader />
      </>,
    );
    await act(async () => {
      await vi.advanceTimersByTimeAsync(200);
    });
    expect(fetcher).toHaveBeenCalledTimes(calls);

    setSession(signedIn("B"));
    view.rerender(
      <>
        <SWRScopeBoundary />
        <PollingReader />
      </>,
    );
    await act(async () => {
      await vi.advanceTimersByTimeAsync(200);
    });
    expect(screen.getAllByText("Daten von B")).toHaveLength(2);
    expect(screen.getByTestId("immutable")).toHaveTextContent("Daten von B");
    const cache = renderHook(() => useSWRConfig());
    expect(
      cache.result.current.cache.get("school-a:A:poll")?.data,
    ).toBeUndefined();
    releaseFakeTimers();
  });

  it("keeps the cache when another tab only binds the same account to another school", async () => {
    render(<Page fetcher={accountFetcher()} />);
    expect(await screen.findByText("Daten von A")).toBeInTheDocument();

    act(() => otherTabBinds("2:A"));

    // The mismatch path of #5130 handles a school change of the cookie.
    expect(screen.getByText("Daten von A")).toBeInTheDocument();
    expect(mockAnnounce).not.toHaveBeenCalled();
  });

  it("drops the cache on a logout in another tab", async () => {
    render(<Page fetcher={accountFetcher()} />);
    expect(await screen.findByText("Daten von A")).toBeInTheDocument();

    act(() => otherTabBinds(null));

    expect(screen.getByTestId("reader-roster")).toHaveTextContent("leer");
    expect(mockAnnounce).toHaveBeenCalledTimes(1);
  });

  it("loads again after another tab switched this account to another school", async () => {
    // performTenantSwitch clears the owner; the new owner binds on the other
    // school's host, whose storage this tab never sees (#5007).
    const fetcher = accountFetcher();
    const view = render(<Page fetcher={fetcher} />);
    expect(await screen.findByText("Daten von A")).toBeInTheDocument();

    act(() => otherTabBinds(null));
    expect(screen.getByTestId("reader-roster")).toHaveTextContent("leer");
    // A render of the session this tab already had confirms nothing.
    view.rerender(<Page fetcher={fetcher} />);
    expect(screen.getByTestId("reader-roster")).toHaveTextContent("leer");

    // The provider's fresh lookup: the same account, now at another school.
    setSession(signedIn("A", { tenantId: 2 }));
    view.rerender(<Page fetcher={fetcher} />);

    expect(await screen.findByText("Daten von A")).toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("keeps waiting while another tab's logout still leaves this school's cookie", async () => {
    // A relogin in another tab revokes the session family and removes the
    // owner before its signout deletes the cookie. A reader loading on that
    // cookie meets the revoked family and starts a competing logout.
    const fetcher = accountFetcher();
    const view = render(<Page fetcher={fetcher} />);
    expect(await screen.findByText("Daten von A")).toBeInTheDocument();

    act(() => otherTabBinds(null));
    setSession(signedIn("A"));
    view.rerender(<Page fetcher={fetcher} />);

    expect(screen.getByTestId("reader-roster")).toHaveTextContent("leer");
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("keeps waiting when the lookup after a removed owner shows another account", async () => {
    const fetcher = accountFetcher();
    const view = render(<Page fetcher={fetcher} />);
    expect(await screen.findByText("Daten von A")).toBeInTheDocument();

    act(() => otherTabBinds(null));
    setSession(signedIn("B"));
    view.rerender(<Page fetcher={fetcher} />);

    expect(screen.getByTestId("reader-roster")).toHaveTextContent("leer");
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("drops the cache on logout and fetches nothing without a session", async () => {
    const fetcher = accountFetcher();
    const view = render(<Page fetcher={fetcher} />);
    expect(await screen.findByText("Daten von A")).toBeInTheDocument();

    rendered = [];
    setSession(signedOut);
    view.rerender(<Page fetcher={fetcher} />);

    expect(screen.getByTestId("reader-roster")).toHaveTextContent("leer");
    expect(rendered).not.toContain("Daten von A");
    expect(cacheKeys()).not.toContain("school-a:A:roster");
    expect(fetcher).toHaveBeenCalledTimes(1);
  });

  it("empties preloads at the boundary", async () => {
    const view = render(<Page fetcher={() => Promise.resolve("Stand")} />);
    expect(await screen.findByText("Stand")).toBeInTheDocument();
    // Preloaded for a page this session has not opened yet.
    void preload("school-a:A:later", () => Promise.resolve("vorgeladen"));

    // A rights change keeps the keys but ends the generation.
    setSession(signedIn("A", { permissions: ["groups:read"] }));
    view.rerender(<Page fetcher={() => Promise.resolve("Stand")} />);
    rendered = [];
    render(<Reader name="later" fetcher={() => Promise.resolve("frisch")} />);

    expect(await screen.findByText("frisch")).toBeInTheDocument();
    expect(rendered).not.toContain("vorgeladen");
  });

  it("reloads mounted readers of an unchanged key after a rights change", async () => {
    const fetcher = vi
      .fn<() => Promise<string>>()
      .mockResolvedValueOnce("alte Rechte")
      .mockResolvedValueOnce("neue Rechte");
    const view = render(<Page fetcher={fetcher} />);
    expect(await screen.findByText("alte Rechte")).toBeInTheDocument();

    setSession(signedIn("A", { permissions: ["groups:read"] }));
    view.rerender(<Page fetcher={fetcher} />);

    expect(await screen.findByText("neue Rechte")).toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("drops the cache on a school change and loads again only back at this school", async () => {
    const fetcher = vi
      .fn<() => Promise<string>>()
      .mockResolvedValueOnce("Schule 1")
      .mockResolvedValueOnce("wieder Schule 1");
    const view = render(<Page fetcher={fetcher} />);
    expect(await screen.findByText("Schule 1")).toBeInTheDocument();

    // The cookie holds school 2: the BFF would refuse every request here.
    setSession(signedIn("A", { tenantId: 2 }));
    view.rerender(<Page fetcher={fetcher} />);
    expect(screen.getByTestId("reader-roster")).toHaveTextContent("leer");
    expect(fetcher).toHaveBeenCalledTimes(1);

    // TenantGuard switched back.
    setSession(signedIn("A"));
    view.rerender(<Page fetcher={fetcher} />);
    expect(await screen.findByText("wieder Schule 1")).toBeInTheDocument();
  });

  it("never writes a late answer of the previous account", async () => {
    let answerA: (value: string) => void = () => {};
    const fetcher = vi.fn(() => {
      const session = mockUseSession() as { data: Session };
      if (session.data.user.id === "A") {
        return new Promise<string>((resolve) => {
          answerA = resolve;
        });
      }
      return Promise.resolve("Daten von B");
    });
    const view = render(<Page fetcher={fetcher} />);
    // Anker: der Abruf von A läuft.
    expect(screen.getByTestId("reader-roster")).toHaveTextContent("leer");
    expect(fetcher).toHaveBeenCalledTimes(1);

    setSession(signedIn("B"));
    view.rerender(<Page fetcher={fetcher} />);
    expect(await screen.findByText("Daten von B")).toBeInTheDocument();

    rendered = [];
    await act(async () => {
      answerA("Daten von A");
      await Promise.resolve();
    });

    expect(screen.getByText("Daten von B")).toBeInTheDocument();
    expect(rendered).not.toContain("Daten von A");
    const { result } = renderHook(() => useSWRConfig());
    expect(result.current.cache.get("school-a:A:roster")?.data).toBeUndefined();
  });

  it("drops the rollback of a mutation that fails after the switch", async () => {
    const view = render(<Page fetcher={() => Promise.resolve("Stand A")} />);
    expect(await screen.findByText("Stand A")).toBeInTheDocument();

    let fail: (error: Error) => void = () => {};
    const saving = mutate(
      "school-a:A:roster",
      new Promise<string>((_, reject) => {
        fail = reject;
      }),
      { optimisticData: "optimistisch", rollbackOnError: true },
    ).catch(() => undefined);
    expect(await screen.findByText("optimistisch")).toBeInTheDocument();

    setSession(signedIn("B"));
    view.rerender(<Page fetcher={() => Promise.resolve("Stand B")} />);
    expect(await screen.findByText("Stand B")).toBeInTheDocument();

    await act(async () => {
      fail(new Error("Speichern fehlgeschlagen"));
      await saving;
    });

    expect(screen.getByText("Stand B")).toBeInTheDocument();
    const { result } = renderHook(() => useSWRConfig());
    expect(result.current.cache.get("school-a:A:roster")).toBeUndefined();
  });

  it("drops useSWRInfinite and subscription entries, which key filters skip", async () => {
    const view = render(<Page fetcher={() => Promise.resolve("Stand A")} />);
    expect(await screen.findByText("Stand A")).toBeInTheDocument();
    const { result } = renderHook(() => useSWRConfig());
    result.current.cache.set("$inf$school-a:A:feed", { data: [["Seite"]] });
    result.current.cache.set("$sub$school-a:A:live", { data: "live" });

    setSession(signedIn("B"));
    act(() => view.rerender(<Page fetcher={() => Promise.resolve("B")} />));

    expect(cacheKeys()).not.toContain("$inf$school-a:A:feed");
    expect(cacheKeys()).not.toContain("$sub$school-a:A:live");
  });

  it("drops the list overlays at the boundary", async () => {
    const list = "school-a|requests:A:";
    startListFetch();
    addListOverlay<string>(list, (items) => items.slice(1));
    const overlays = renderHook(() => useListOverlays<string>(list));
    expect(overlays.result.current).toHaveLength(1);

    const view = render(<Page fetcher={accountFetcher()} />);
    expect(await screen.findByText("Daten von A")).toBeInTheDocument();
    setSession(signedIn("B"));
    act(() => view.rerender(<Page fetcher={accountFetcher()} />));

    expect(overlays.result.current).toHaveLength(0);
  });

  it("stays idle within the render budget", async () => {
    vi.useFakeTimers();
    await expectIdleRenderBudget(<SWRScopeBoundary expectedTenantId={1} />);
    releaseFakeTimers();
  });
});

describe("useDiscardRefusedEntity (#5006)", () => {
  function ChildPage({ refuse }: Readonly<{ refuse: () => Promise<string> }>) {
    return (
      <>
        <Reader name="student-7-detail" fetcher={refuse} />
        <Reader
          name="student-7-notes"
          fetcher={() => Promise.resolve("Notiz")}
        />
        <Reader
          name="student-8-notes"
          fetcher={() => Promise.resolve("Andere")}
        />
      </>
    );
  }

  it("drops every entry of a refused child and keeps the refusal visible", async () => {
    const refusal = new ApiError("Forbidden", 403, {
      code: "care.guardian_permission_denied",
    });
    let refuse = false;
    const detail = vi.fn(() =>
      refuse ? Promise.reject(refusal) : Promise.resolve("Kind 7"),
    );
    const view = render(<ChildPage refuse={detail} />);
    expect(await screen.findByText("Kind 7")).toBeInTheDocument();
    expect(await screen.findByText("Notiz")).toBeInTheDocument();

    refuse = true;
    await act(async () => {
      await mutate("school-a:A:student-7-detail");
    });
    const { result } = renderHook(() => useDiscardRefusedEntity());
    rendered = [];
    let discarded = false;
    act(() => {
      discarded = result.current(refusal, {
        codes: ["care.guardian_permission_denied"],
        matches: (key) => key.startsWith("student-7-"),
      });
    });
    view.rerender(<ChildPage refuse={detail} />);

    expect(discarded).toBe(true);
    // Anker: das andere Kind bleibt.
    expect(screen.getByTestId("reader-student-8-notes")).toHaveTextContent(
      "Andere",
    );
    expect(screen.getByTestId("reader-student-7-detail")).toHaveTextContent(
      "Fehler",
    );
    expect(screen.getByTestId("reader-student-7-notes")).toHaveTextContent(
      "leer",
    );
    expect(rendered).not.toContain("Kind 7");
    expect(rendered).not.toContain("Notiz");
  });

  it("leaves the cache alone for another code or status", () => {
    const { result } = renderHook(() => useDiscardRefusedEntity());
    const matches = vi.fn(() => true);

    expect(
      result.current(
        new ApiError("Conflict", 409, { code: "general.permission" }),
        {
          codes: ["general.permission"],
          matches,
        },
      ),
    ).toBe(false);
    expect(
      result.current(
        new ApiError("Not found", 404, { code: "rooms.not_found" }),
        {
          codes: ["care.request_not_found"],
          matches,
        },
      ),
    ).toBe(false);
    expect(result.current(new Error("network"), { codes: [], matches })).toBe(
      false,
    );
    expect(matches).not.toHaveBeenCalled();
  });
});
