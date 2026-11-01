import { act, fireEvent, render, screen } from "@testing-library/react";
import { useState, type ComponentProps } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SWRConfig } from "~/lib/swr/testing";
import { useSWRAuth } from "~/lib/swr";
import {
  _setSWRCleanupForTests,
  resetSWRScope,
  resumeSWRSession,
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

afterEach(() => _setSWRCleanupForTests(true));

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
    await advance(0);
    expect(fetcher).toHaveBeenCalledTimes(1);
  });
  it.each(["focus", "touchStart"] as const)(
    "preloads immediately on %s",
    async (event) => {
      render(<Harness />);
      const link = screen.getByRole("link", { name: "Open rooms" });
      expect(link).toBeInTheDocument();
      fireEvent[event](link);
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
    fireEvent.focus(link);
    fireEvent.touchStart(link);
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
      await advance(0);
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
    await advance(0);
    expect(fetcher).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("link", { name: "Open rooms" }));
    await advance(10);
    // Anker: die Zielseite hat ihre eigene erfolgreiche Antwort erhalten.
    expect(screen.getByText("confirmed rooms")).toBeInTheDocument();
    expect(screen.queryByText("failed")).not.toBeInTheDocument();
    expect(fetcher).toHaveBeenCalledTimes(2);
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
  await advance(0);
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
  await advance(0);
  expect(fetcher).toHaveBeenCalledTimes(1);
  // Anker: der Quelllink bleibt nach dem Hintergrundfehler bedienbar.
  expect(screen.getByRole("link", { name: "Open rooms" })).toBeInTheDocument();
  expect(screen.queryByText("failed")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("link", { name: "Open rooms" }));
  await advance(10);
  expect(screen.getByText("failed")).toBeInTheDocument();
  expect(fetcher).toHaveBeenCalledTimes(2);
});
