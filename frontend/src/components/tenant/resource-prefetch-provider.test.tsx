import { act, fireEvent, render, screen } from "@testing-library/react";
import { useState, type ComponentProps } from "react";
import { flushSync } from "react-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SWRConfig } from "~/lib/swr/testing";
import { useSWRAuth } from "~/lib/swr";
import { useDataPrefetchIntent } from "~/components/ui/data-prefetch-intent";
import {
  _setSWRCleanupForTests,
  resetSWRScope,
  resumeSWRSession,
  suspendSWRSession,
} from "~/lib/swr/scope";
import { roomKeys } from "~/lib/swr/rooms-keys";
import { ResourcePrefetchProvider } from "./resource-prefetch-provider";
import NavigationLink from "~/components/ui/navigation-link";
import { NavLink } from "~/components/ui/nav-link";
import { ButtonLink } from "~/components/ui/button";

vi.unmock("swr");
const { owner } = vi.hoisted(() => ({
  owner: { id: "account-1", scope: "tenant", tenantId: 1, slug: "school-a" },
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
vi.mock("next/link", () => ({
  default: ({
    children,
    prefetch: _prefetch,
    onNavigate: _onNavigate,
    ...props
  }: ComponentProps<"a"> & { prefetch?: boolean; onNavigate?: unknown }) => (
    <a {...props}>{children}</a>
  ),
}));

const key = roomKeys.key({ page: 1, pageSize: 1000 });
const fetcher = vi.fn<() => Promise<{ name: string }>>();
const resources = [{ key, fetcher }];
const resourcesFor = vi.fn(async () => resources);

function Destination() {
  const result = useSWRAuth(key, fetcher, { revalidateOnFocus: false });
  return (
    <output>
      {result.error ? "failed" : (result.data?.name ?? "loading")}
    </output>
  );
}
function Harness({
  Link = NavigationLink,
}: {
  Link?: typeof NavigationLink | typeof NavLink | typeof ButtonLink;
}) {
  const [opened, setOpened] = useState(false);
  return (
    <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 2000 }}>
      <ResourcePrefetchProvider resourcesFor={resourcesFor}>
        <Link
          href="/rooms"
          onClick={(event) => {
            event.preventDefault();
            setOpened(true);
          }}
        >
          Open rooms
        </Link>
        {opened && <Destination />}
      </ResourcePrefetchProvider>
    </SWRConfig>
  );
}
function CardIntent() {
  const [opened, setOpened] = useState(false);
  const intent = useDataPrefetchIntent("/rooms");
  return (
    <>
      <button {...intent} onClick={() => setOpened(true)}>
        Open card
      </button>
      {opened && <Destination />}
    </>
  );
}
function CardHarness() {
  return (
    <SWRConfig value={{ provider: () => new Map() }}>
      <ResourcePrefetchProvider resourcesFor={resourcesFor}>
        <CardIntent />
      </ResourcePrefetchProvider>
    </SWRConfig>
  );
}
const touch = (x: number, y: number) => ({
  identifier: 1,
  clientX: x,
  clientY: y,
});
async function advance(ms: number) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms);
  });
}
beforeEach(() => {
  vi.useFakeTimers();
  resetSWRScope({ revalidate: false });
  resumeSWRSession();
  owner.id = "account-1";
  owner.slug = "school-a";
  fetcher.mockReset().mockResolvedValue({ name: "confirmed rooms" });
  resourcesFor.mockClear();
  Object.defineProperty(navigator, "connection", {
    configurable: true,
    value: { saveData: false, effectiveType: "4g" },
  });
});

afterEach(() => {
  _setSWRCleanupForTests(true);
  vi.restoreAllMocks();
});

describe("review intent boundaries", () => {
  it("does not preload a card while swiping or on touch-generated focus", async () => {
    render(<CardHarness />);
    const card = screen.getByRole("button", { name: "Open card" });
    expect(card).toBeInTheDocument();
    fireEvent.touchStart(card, { touches: [touch(10, 10)] });
    fireEvent.focus(card);
    fireEvent.touchMove(card, { touches: [touch(10, 80)] });
    fireEvent.touchEnd(card, { touches: [], changedTouches: [touch(10, 100)] });
    await advance(400);
    expect(fetcher).not.toHaveBeenCalled();
  });
  it("preloads a card only at the end of a stationary tap", async () => {
    render(<CardHarness />);
    const card = screen.getByRole("button", { name: "Open card" });
    expect(card).toBeInTheDocument();
    fireEvent.touchStart(card, { touches: [touch(10, 10)] });
    await advance(400);
    expect(fetcher).not.toHaveBeenCalled();
    fireEvent.touchEnd(card, { touches: [], changedTouches: [touch(10, 10)] });
    await advance(0);
    expect(fetcher).toHaveBeenCalledTimes(1);
  });
  it("allows sustained keyboard focus after a completed swipe that never focused the card", async () => {
    render(<CardHarness />);
    const card = screen.getByRole("button", { name: "Open card" });
    expect(card).toBeInTheDocument();
    fireEvent.touchStart(card, { touches: [touch(10, 10)] });
    fireEvent.touchMove(card, { touches: [touch(10, 90)] });
    fireEvent.touchEnd(card, { touches: [], changedTouches: [touch(10, 90)] });
    await advance(300);
    expect(fetcher).not.toHaveBeenCalled();
    fireEvent.focus(card);
    await advance(300);
    expect(fetcher).toHaveBeenCalledTimes(1);
  });
  it("waits for sustained focus and cancels on blur", async () => {
    render(<Harness />);
    const link = screen.getByRole("link", { name: "Open rooms" });
    expect(link).toBeInTheDocument();
    fireEvent.focus(link);
    await advance(299);
    expect(fetcher).not.toHaveBeenCalled();
    fireEvent.blur(link);
    await advance(10);
    expect(fetcher).not.toHaveBeenCalled();
    fireEvent.focus(link);
    await advance(300);
    expect(fetcher).toHaveBeenCalledTimes(1);
  });
  it("suppresses data intent for prefers-reduced-data", async () => {
    vi.spyOn(window, "matchMedia").mockImplementation((query) => ({
      matches: query === "(prefers-reduced-data: reduce)",
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: () => true,
    }));
    render(<Harness />);
    const link = screen.getByRole("link", { name: "Open rooms" });
    expect(link).toBeInTheDocument();
    fireEvent.pointerEnter(link, { pointerType: "mouse" });
    await advance(300);
    expect(fetcher).not.toHaveBeenCalled();
  });
  it("does not hand a minute-old promise to a newly opened destination even if timers were suspended", async () => {
    fetcher.mockResolvedValueOnce({ name: "stale rooms" });
    render(<Harness />);
    const link = screen.getByRole("link", { name: "Open rooms" });
    fireEvent.pointerEnter(link, { pointerType: "mouse" });
    await advance(300);
    expect(fetcher).toHaveBeenCalledTimes(1);
    vi.setSystemTime(Date.now() + 60_000);
    fireEvent.click(link);
    await advance(10);
    expect(screen.getByText("confirmed rooms")).toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(2);
  });
  it("replaces older queued intent rather than draining obsolete destinations", async () => {
    const finish: (() => void)[] = [];
    const old = Array.from({ length: 5 }, (_, page) => ({
      key: roomKeys.key({ page: page + 1 }),
      fetcher: vi.fn(
        () =>
          new Promise<string>((resolve) => finish.push(() => resolve("old"))),
      ),
    }));
    const latest = {
      key: roomKeys.key({ page: 6 }),
      fetcher: vi.fn(async () => "latest"),
    };
    render(
      <SWRConfig value={{ provider: () => new Map() }}>
        <ResourcePrefetchProvider
          resourcesFor={async (href) => (href === "/old" ? old : [latest])}
        >
          <NavigationLink href="/old">Old intent</NavigationLink>
          <NavigationLink href="/latest">Latest intent</NavigationLink>
        </ResourcePrefetchProvider>
      </SWRConfig>,
    );
    const source = screen.getByRole("link", { name: "Old intent" });
    fireEvent.pointerEnter(source, { pointerType: "mouse" });
    await advance(300);
    expect(old.map((item) => item.fetcher.mock.calls.length)).toEqual([
      1, 1, 1, 0, 0,
    ]);
    fireEvent.pointerEnter(
      screen.getByRole("link", { name: "Latest intent" }),
      { pointerType: "mouse" },
    );
    await advance(300);
    await act(async () => finish.forEach((done) => done()));
    await advance(10);
    expect(latest.fetcher).toHaveBeenCalledTimes(1);
    expect(old.map((item) => item.fetcher.mock.calls.length)).toEqual([
      1, 1, 1, 0, 0,
    ]);
  });
  it("does not start old queued resources after the bounded wait expires", async () => {
    const finish: (() => void)[] = [];
    const loads = Array.from({ length: 5 }, (_, page) => ({
      key: roomKeys.key({ page: page + 1 }),
      fetcher: vi.fn(
        () =>
          new Promise<string>((resolve) => finish.push(() => resolve("old"))),
      ),
    }));
    render(
      <SWRConfig value={{ provider: () => new Map() }}>
        <ResourcePrefetchProvider resourcesFor={async () => loads}>
          <NavigationLink href="/rooms">Queue expiry</NavigationLink>
        </ResourcePrefetchProvider>
      </SWRConfig>,
    );
    const link = screen.getByRole("link", { name: "Queue expiry" });
    expect(link).toBeInTheDocument();
    fireEvent.pointerEnter(link, { pointerType: "mouse" });
    await advance(300);
    expect(loads.map((item) => item.fetcher.mock.calls.length)).toEqual([
      1, 1, 1, 0, 0,
    ]);
    await advance(6_000);
    await act(async () => finish.forEach((done) => done()));
    await advance(10);
    expect(loads.map((item) => item.fetcher.mock.calls.length)).toEqual([
      1, 1, 1, 0, 0,
    ]);
  });
  it("does not turn expiry of unconsumed pending speculation into a background retry", async () => {
    let reject!: (error: Error) => void;
    fetcher.mockImplementationOnce(
      () =>
        new Promise((_resolve, fail) => {
          reject = fail;
        }),
    );
    render(<Harness />);
    const link = screen.getByRole("link", { name: "Open rooms" });
    expect(link).toBeInTheDocument();
    fireEvent.pointerEnter(link, { pointerType: "mouse" });
    await advance(300);
    await advance(46_000);
    await act(async () => reject(new Error("expired speculation")));
    await advance(0);
    expect(fetcher).toHaveBeenCalledTimes(1);
    fireEvent.click(link);
    await advance(10);
    expect(screen.getByText("confirmed rooms")).toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(2);
  });
  it("does not let an old expiry timer remove a newer generation's promise", async () => {
    fetcher.mockResolvedValueOnce({ name: "old rooms" });
    const view = render(<Harness />);
    const link = screen.getByRole("link", { name: "Open rooms" });
    fireEvent.pointerEnter(link, { pointerType: "mouse" });
    await advance(300);
    expect(fetcher).toHaveBeenCalledTimes(1);
    await advance(20_000);
    await act(async () => resetSWRScope({ revalidate: false }));
    view.rerender(<Harness />);
    fireEvent.pointerEnter(link, { pointerType: "mouse" });
    await advance(300);
    expect(fetcher).toHaveBeenCalledTimes(2);
    await advance(26_000);
    fireEvent.click(link);
    await advance(10);
    expect(screen.getByText("confirmed rooms")).toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(2);
  });
});
describe("locked navigation data intent", () => {
  it.each([NavigationLink, NavLink, ButtonLink])(
    "preloads exactly the destination resource once after hover (%s)",
    async (Link) => {
      render(<Harness Link={Link} />);
      const link = screen.getByRole("link", { name: "Open rooms" });
      expect(link).toHaveAttribute("href", "/rooms");
      fireEvent.pointerEnter(link, { pointerType: "mouse" });
      await advance(299);
      expect(fetcher).not.toHaveBeenCalled();
      await advance(1);
      expect(fetcher).toHaveBeenCalledTimes(1);
      fireEvent.focus(link);
      fireEvent.touchStart(link);
      await advance(0);
      fireEvent.click(link);
      await advance(10);
      expect(screen.getByText("confirmed rooms")).toBeInTheDocument();
      expect(fetcher).toHaveBeenCalledTimes(1);
    },
  );
  it("does not preload on a passing pointer", async () => {
    render(<Harness />);
    const link = screen.getByRole("link", { name: "Open rooms" });
    expect(link).toHaveAttribute("href", "/rooms");
    fireEvent.pointerEnter(link, { pointerType: "mouse" });
    await advance(200);
    fireEvent.pointerLeave(link);
    await advance(500);
    expect(fetcher).not.toHaveBeenCalled();
    fireEvent.focus(link);
    await advance(300);
    expect(fetcher).toHaveBeenCalledTimes(1);
  });
  it.each([NavigationLink, NavLink, ButtonLink])(
    "preloads link data only after a tap ends (%s)",
    async (Link) => {
      render(<Harness Link={Link} />);
      const link = screen.getByRole("link", { name: "Open rooms" });
      expect(link).toBeInTheDocument();
      fireEvent.touchStart(link, { touches: [touch(10, 10)] });
      await advance(300);
      expect(fetcher).not.toHaveBeenCalled();
      fireEvent.touchEnd(link, {
        touches: [],
        changedTouches: [touch(10, 10)],
      });
      await advance(0);
      expect(fetcher).toHaveBeenCalledTimes(1);
    },
  );
  it.each([
    { saveData: true, effectiveType: "4g" },
    { saveData: false, effectiveType: "3g" },
    { saveData: false, effectiveType: "2g" },
  ])("suppresses preloading for %j", async (connection) => {
    Object.defineProperty(navigator, "connection", {
      configurable: true,
      value: connection,
    });
    render(<Harness />);
    const link = screen.getByRole("link", { name: "Open rooms" });
    expect(link).toBeInTheDocument();
    fireEvent.pointerEnter(link, { pointerType: "mouse" });
    await advance(300);
    fireEvent.touchStart(link, { touches: [touch(10, 10)] });
    fireEvent.touchEnd(link, { touches: [], changedTouches: [touch(10, 10)] });
    await advance(0);
    expect(fetcher).not.toHaveBeenCalled();
    fireEvent.click(link);
    await advance(10);
    expect(screen.getByText("confirmed rooms")).toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(1);
  });
  it.each([true, false])(
    "discards an answer arriving after a scope change (cleanup %s)",
    async (cleanup) => {
      _setSWRCleanupForTests(cleanup);
      let resolve!: (value: { name: string }) => void;
      fetcher.mockImplementationOnce(
        () =>
          new Promise((done) => {
            resolve = done;
          }),
      );
      const view = render(<Harness />);
      fireEvent.focus(screen.getByRole("link", { name: "Open rooms" }));
      await advance(300);
      expect(fetcher).toHaveBeenCalledTimes(1);
      await act(async () => {
        owner.id = "account-2";
        owner.slug = "school-b";
        resetSWRScope({ revalidate: false });
      });
      view.rerender(<Harness />);
      await act(async () => resolve({ name: "old secret" }));
      fireEvent.click(screen.getByRole("link", { name: "Open rooms" }));
      await advance(10);
      // Anker: die neue Schule hat ihre bestätigte Antwort geladen.
      expect(screen.getByText("confirmed rooms")).toBeInTheDocument();
      expect(screen.queryByText("old secret")).not.toBeInTheDocument();
      expect(fetcher).toHaveBeenCalledTimes(2);
    },
  );
  it("does not hand a speculative failure to the page; normal load succeeds", async () => {
    fetcher.mockRejectedValueOnce(new Error("offline during intent"));
    render(<Harness />);
    fireEvent.focus(screen.getByRole("link", { name: "Open rooms" }));
    await advance(300);
    expect(fetcher).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("link", { name: "Open rooms" }));
    await advance(10);
    // Anker: die Zielseite hat ihre eigene erfolgreiche Antwort erhalten.
    expect(screen.getByText("confirmed rooms")).toBeInTheDocument();
    expect(screen.queryByText("failed")).not.toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(2);
  });
  it("keeps a consumed speculative failure out of the destination while its own load runs", async () => {
    let rejectIntent!: (error: Error) => void;
    let finishPage!: (value: { name: string }) => void;
    fetcher
      .mockImplementationOnce(
        () =>
          new Promise((_resolve, reject) => {
            rejectIntent = reject;
          }),
      )
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            finishPage = resolve;
          }),
      );
    render(<Harness />);
    const link = screen.getByRole("link", { name: "Open rooms" });
    fireEvent.focus(link);
    await advance(300);
    expect(fetcher).toHaveBeenCalledTimes(1);
    fireEvent.click(link);
    await advance(0);
    expect(screen.getByText("loading")).toBeInTheDocument();
    await act(async () =>
      rejectIntent(new Error("intent failed after navigation")),
    );
    await advance(10);
    expect(fetcher).toHaveBeenCalledTimes(2);
    // Anker: die geöffnete Zielseite wartet auf ihren eigenen Abruf.
    expect(screen.getByText("loading")).toBeInTheDocument();
    expect(screen.queryByText("failed")).not.toBeInTheDocument();
    await act(async () => finishPage({ name: "page rooms" }));
    await advance(10);
    expect(screen.getByText("page rooms")).toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(2);
  });
  it.each([0, 1, 2, 3, 4, 5, 6])(
    "loads normally when navigation races rejection at microtask %s",
    async (turns) => {
      let rejectIntent!: (error: Error) => void;
      fetcher.mockImplementationOnce(
        () =>
          new Promise((_resolve, reject) => {
            rejectIntent = reject;
          }),
      );
      render(<Harness />);
      const link = screen.getByRole("link", { name: "Open rooms" });
      fireEvent.focus(link);
      await advance(300);
      expect(fetcher).toHaveBeenCalledTimes(1);
      await act(async () => {
        rejectIntent(new Error("intent failed alongside navigation"));
        for (let turn = 0; turn < turns; turn++) await Promise.resolve();
        flushSync(() => fireEvent.click(link));
      });
      await advance(10);
      expect(screen.getByText("confirmed rooms")).toBeInTheDocument();
      expect(fetcher).toHaveBeenCalledTimes(2);
    },
  );
  it("shows the destination failure if its handoff fallback fails too", async () => {
    let rejectIntent!: (error: Error) => void;
    fetcher
      .mockImplementationOnce(
        () =>
          new Promise((_resolve, reject) => {
            rejectIntent = reject;
          }),
      )
      .mockRejectedValueOnce(new Error("page is offline too"));
    render(<Harness />);
    const link = screen.getByRole("link", { name: "Open rooms" });
    fireEvent.focus(link);
    await advance(300);
    fireEvent.click(link);
    await advance(0);
    expect(screen.getByText("loading")).toBeInTheDocument();
    await act(async () =>
      rejectIntent(new Error("intent failed after navigation")),
    );
    await advance(10);
    expect(screen.getByText("failed")).toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(2);
  });
  it("does not retry consumed speculation after an account and school change", async () => {
    let rejectIntent!: (error: Error) => void;
    fetcher.mockImplementationOnce(
      () =>
        new Promise((_resolve, reject) => {
          rejectIntent = reject;
        }),
    );
    const view = render(<Harness />);
    const link = screen.getByRole("link", { name: "Open rooms" });
    fireEvent.focus(link);
    await advance(300);
    fireEvent.click(link);
    await advance(0);
    expect(screen.getByText("loading")).toBeInTheDocument();
    await act(async () => {
      owner.id = "account-2";
      owner.slug = "school-b";
      resetSWRScope({ revalidate: false });
    });
    view.rerender(<Harness />);
    await advance(10);
    expect(screen.getByText("confirmed rooms")).toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(2);
    await act(async () => rejectIntent(new Error("old intent failed")));
    await advance(10);
    // Anker: die neue Schule zeigt weiterhin ihre bestätigten Daten.
    expect(screen.getByText("confirmed rooms")).toBeInTheDocument();
    expect(screen.queryByText("failed")).not.toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(2);
  });

  it("discards a handoff fallback that finishes after the scope changes", async () => {
    let rejectIntent!: (error: Error) => void;
    let finishOldPage!: (value: { name: string }) => void;
    fetcher
      .mockImplementationOnce(
        () =>
          new Promise((_resolve, reject) => {
            rejectIntent = reject;
          }),
      )
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            finishOldPage = resolve;
          }),
      );
    const view = render(<Harness />);
    const link = screen.getByRole("link", { name: "Open rooms" });
    fireEvent.focus(link);
    await advance(300);
    fireEvent.click(link);
    await advance(0);
    expect(screen.getByText("loading")).toBeInTheDocument();
    await act(async () =>
      rejectIntent(new Error("intent failed after navigation")),
    );
    await advance(10);
    expect(fetcher).toHaveBeenCalledTimes(2);
    await act(async () => {
      owner.id = "account-2";
      owner.slug = "school-b";
      resetSWRScope({ revalidate: false });
    });
    view.rerender(<Harness />);
    await advance(10);
    expect(screen.getByText("confirmed rooms")).toBeInTheDocument();
    await act(async () => finishOldPage({ name: "old fallback secret" }));
    await advance(10);
    // Anker: auch nach dem späten Fallback steht die Antwort der neuen Schule da.
    expect(screen.getByText("confirmed rooms")).toBeInTheDocument();
    expect(screen.queryByText("old fallback secret")).not.toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(3);
  });
  it("does not retry consumed speculation while the session is being confirmed", async () => {
    let rejectIntent!: (error: Error) => void;
    fetcher.mockImplementationOnce(
      () =>
        new Promise((_resolve, reject) => {
          rejectIntent = reject;
        }),
    );
    const view = render(<Harness />);
    const link = screen.getByRole("link", { name: "Open rooms" });
    fireEvent.focus(link);
    await advance(300);
    fireEvent.click(link);
    await advance(0);
    expect(screen.getByText("loading")).toBeInTheDocument();
    await act(async () => suspendSWRSession());
    view.rerender(<Harness />);
    await act(async () =>
      rejectIntent(new Error("intent failed while confirming")),
    );
    await advance(10);
    // Anker: die Zielseite wartet auf die bestätigte Session.
    expect(screen.getByText("loading")).toBeInTheDocument();
    expect(screen.queryByText("failed")).not.toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(1);
  });
});

it("bounds concurrent resources and discards queued work after a scope change", async () => {
  const resolve: (() => void)[] = [];
  const loads = Array.from({ length: 5 }, (_, index) => ({
    key: roomKeys.key({ page: index + 1 }),
    fetcher: vi.fn(
      () =>
        new Promise<string>((done) => {
          resolve.push(() => done("room"));
        }),
    ),
  }));
  render(
    <SWRConfig value={{ provider: () => new Map() }}>
      <ResourcePrefetchProvider resourcesFor={async () => loads}>
        <NavigationLink href="/rooms">Queue rooms</NavigationLink>
      </ResourcePrefetchProvider>
    </SWRConfig>,
  );
  const link = screen.getByRole("link", { name: "Queue rooms" });
  expect(link).toBeInTheDocument();
  fireEvent.focus(link);
  await advance(300);
  expect(loads.map((load) => load.fetcher.mock.calls.length)).toEqual([
    1, 1, 1, 0, 0,
  ]);
  await act(async () => {
    resetSWRScope({ revalidate: false });
    resolve.forEach((done) => done());
  });
  await advance(0);
  expect(loads.map((load) => load.fetcher.mock.calls.length)).toEqual([
    1, 1, 1, 0, 0,
  ]);
});

it("keeps the normal destination error path when its own load fails too", async () => {
  fetcher.mockRejectedValue(new Error("still offline"));
  render(<Harness />);
  fireEvent.focus(screen.getByRole("link", { name: "Open rooms" }));
  await advance(300);
  expect(fetcher).toHaveBeenCalledTimes(1);
  // Anker: der Quelllink bleibt nach dem Hintergrundfehler bedienbar.
  expect(screen.getByRole("link", { name: "Open rooms" })).toBeInTheDocument();
  expect(screen.queryByText("failed")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("link", { name: "Open rooms" }));
  await advance(10);
  expect(screen.getByText("failed")).toBeInTheDocument();
  expect(fetcher).toHaveBeenCalledTimes(2);
});

it("does not duplicate a destination request that is already in flight", async () => {
  let finish!: (value: { name: string }) => void;
  fetcher.mockImplementation(
    () =>
      new Promise((done) => {
        finish = done;
      }),
  );
  render(
    <SWRConfig value={{ provider: () => new Map() }}>
      <ResourcePrefetchProvider resourcesFor={resourcesFor}>
        <NavigationLink href="/rooms">Existing rooms</NavigationLink>
        <Destination />
      </ResourcePrefetchProvider>
    </SWRConfig>,
  );
  await advance(0);
  expect(fetcher).toHaveBeenCalledTimes(1);
  fireEvent.focus(screen.getByRole("link", { name: "Existing rooms" }));
  await advance(300);
  expect(fetcher).toHaveBeenCalledTimes(1);
  await act(async () => finish({ name: "existing answer" }));
  await advance(10);
  expect(screen.getByText("existing answer")).toBeInTheDocument();
});

it("never lets an explicit link list bypass the authorized destination resources", async () => {
  const unknownKey = roomKeys.key({ page: 99 });
  const unsafeFetcher = vi.fn(async () => ({ name: "wrong projection" }));
  const supplied = [
    { key, fetcher: unsafeFetcher },
    { key: unknownKey, fetcher: unsafeFetcher },
  ];
  const view = render(
    <SWRConfig value={{ provider: () => new Map() }}>
      <ResourcePrefetchProvider resourcesFor={async () => []}>
        <NavigationLink href="/rooms" resources={supplied}>
          Selected rooms
        </NavigationLink>
      </ResourcePrefetchProvider>
    </SWRConfig>,
  );
  const link = screen.getByRole("link", { name: "Selected rooms" });
  expect(link).toBeInTheDocument();
  fireEvent.focus(link);
  await advance(300);
  expect(unsafeFetcher).not.toHaveBeenCalled();
  view.unmount();
  render(
    <SWRConfig value={{ provider: () => new Map() }}>
      <ResourcePrefetchProvider resourcesFor={resourcesFor}>
        <NavigationLink href="/rooms" resources={supplied}>
          Selected rooms
        </NavigationLink>
      </ResourcePrefetchProvider>
    </SWRConfig>,
  );
  fireEvent.focus(screen.getByRole("link", { name: "Selected rooms" }));
  await advance(300);
  expect(fetcher).toHaveBeenCalledTimes(1);
  expect(unsafeFetcher).not.toHaveBeenCalled();
});
