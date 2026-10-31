import { act, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Profiler } from "react";
import type { Session } from "next-auth";
import { SWRConfig } from "~/lib/swr/testing";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { ApiError } from "~/lib/api-error";
import { resetSWRScope, resumeSWRSession } from "~/lib/swr/scope";
import { ParentRealtimeBridge } from "~/components/parent/parent-realtime-bridge";
import {
  fetchParentProfile,
  getChildFeatures,
  getChildToday,
  listMyChildren,
  listMessageThreads,
  listAnnouncements,
} from "~/lib/parent-api";
import { ParentStartPage } from "./parent-start-page";

vi.unmock("swr");
vi.mock("~/lib/hooks/use-sse", () => ({ useSSE: () => undefined }));
const state = vi.hoisted(() => ({ account: "A" as string | null }));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    status: state.account ? "authenticated" : "unauthenticated",
    data: state.account
      ? ({
          user: { id: state.account, scope: "parent" },
          expires: "2099-01-01",
        } satisfies Session)
      : null,
  }),
}));
vi.mock("~/lib/tenant-context", () => ({ useTenantSlugSafe: () => null }));
vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: () => ({ profile: { firstName: "Eltern" } }),
  useShellAuthSafe: () => undefined,
}));
vi.mock("~/lib/parent-api", async (original) => ({
  ...(await original<typeof import("~/lib/parent-api")>()),
  listMyChildren: vi.fn(async () => [
    {
      student_id: "42",
      tenant_id: "7",
      first_name: `Kind ${state.account}`,
      last_name: "Test",
      school_name: "Schule",
      school_class: "1a",
    },
  ]),
  getChildFeatures: vi.fn(async () => ({})),
  getChildToday: vi.fn(async () => ({ at_ogs: null, state: "unknown" })),
  fetchParentProfile: vi.fn(async () => ({ first_name: "Eltern" })),
  listAnnouncements: vi.fn(async () => []),
  listMessageThreads: vi.fn(async () => []),
}));
vi.mock("~/lib/personal-calendar-api", () => ({
  getParentCalendar: vi.fn(async () => ({ events: [] })),
}));

beforeEach(() => {
  resumeSWRSession();
  resetSWRScope({ revalidate: false });
  state.account = "A";
  vi.clearAllMocks();
  vi.mocked(listAnnouncements).mockResolvedValue([]);
  vi.mocked(fetchParentProfile).mockResolvedValue({
    first_name: "Eltern",
    portal_locale: null,
  });
  vi.mocked(listMyChildren).mockImplementation(async () => [
    {
      student_id: "42",
      tenant_id: "7",
      first_name: `Kind ${state.account}`,
      last_name: "Test",
      school_name: "Schule",
      school_class: "1a",
      status: "active",
      school_slug: "schule",
    },
  ]);
  vi.mocked(getChildFeatures).mockResolvedValue(
    {} as Awaited<ReturnType<typeof getChildFeatures>>,
  );
});
afterEach(() => {
  resumeSWRSession();
});

function Fixture({ visible = true }: Readonly<{ visible?: boolean }>) {
  return (
    <>
      <SWRScopeBoundary />
      <ParentRealtimeBridge />
      {visible ? <ParentStartPage /> : null}
    </>
  );
}
function mount(onCommit?: () => void) {
  const cache = new Map();
  return render(<Fixture />, {
    wrapper: ({ children }) => (
      <SWRConfig value={{ provider: () => cache, dedupingInterval: 60_000 }}>
        <Profiler id="parent-cache" onRender={() => onCommit?.()}>
          {children}
        </Profiler>
      </SWRConfig>
    ),
  });
}

describe("parent start cache isolation", () => {
  it("withdraws the old child's data on account change", async () => {
    const view = mount();
    expect(await screen.findByText("Kind A Test")).toBeInTheDocument();
    state.account = "B";
    view.rerender(<Fixture />);
    // Anker: der bestehende Ladekasten oder die neue Kindkarte ersetzt A.
    expect(
      screen.getByText(/Kinder werden geladen|Kind B Test/),
    ).toBeInTheDocument();
    expect(screen.queryByText("Kind A Test")).not.toBeInTheDocument();
    expect(await screen.findByText("Kind B Test")).toBeInTheDocument();
  });
  it("withdraws child data immediately on logout in another tab", async () => {
    mount();
    expect(await screen.findByText("Kind A Test")).toBeInTheDocument();
    act(() => {
      window.dispatchEvent(
        new StorageEvent("storage", {
          key: "moto:personal-storage-owner",
          newValue: null,
        }),
      );
    });
    // Anker: das echte Kind-Ladeskelett ersetzt die alte Karte.
    expect(
      screen.getByTestId("parent-start-child-skeleton"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Kind A Test")).not.toBeInTheDocument();
  });
  it("withdraws every child view after the server refuses the relationship", async () => {
    mount();
    expect(await screen.findByText("Kind A Test")).toBeInTheDocument();
    vi.mocked(getChildFeatures).mockImplementation(async () => {
      vi.mocked(listMyChildren).mockResolvedValue([]);
      throw new ApiError("not linked", 403, { code: "care.child_not_linked" });
    });
    act(() => {
      window.dispatchEvent(
        new CustomEvent("parent-child-status-refresh", {
          detail: { studentId: "42" },
        }),
      );
    });
    // Anker: die neu bestätigte leere Kinderliste ersetzt die entzogene Karte.
    expect(
      await screen.findByText(
        "Mit Ihrem Konto ist noch kein Kind verknüpft. Die OGS Ihrer Schule kann das einrichten.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Kind A Test")).not.toBeInTheDocument();
  });

  it("shows a returning page synchronously without repeating shared GETs", async () => {
    const view = mount();
    expect(await screen.findByText("Kind A Test")).toBeInTheDocument();
    expect(listMyChildren).toHaveBeenCalledTimes(1);
    expect(getChildFeatures).toHaveBeenCalledTimes(1);
    expect(getChildToday).toHaveBeenCalledTimes(1);
    view.rerender(<Fixture visible={false} />);
    view.rerender(<Fixture />);
    // Anker: bestätigte Kindkarte bereits beim ersten Render, kein Kaltstart.
    expect(screen.getByText("Kind A Test")).toBeInTheDocument();
    expect(
      screen.queryByTestId("parent-start-child-skeleton"),
    ).not.toBeInTheDocument();
    expect(listMyChildren).toHaveBeenCalledTimes(1);
    expect(getChildFeatures).toHaveBeenCalledTimes(1);
    expect(getChildToday).toHaveBeenCalledTimes(1);
  });
  it("withdraws child data when another tab reports a relationship refusal", async () => {
    mount();
    expect(await screen.findByText("Kind A Test")).toBeInTheDocument();
    vi.mocked(listMyChildren).mockResolvedValue([]);
    const channel = new BroadcastChannel("moto:parent-access-refused");
    channel.postMessage({
      source: "other-tab",
      account: "A",
      code: "care.child_not_linked",
      status: 403,
    });
    channel.close();
    // Anker: die neue Kinderliste bestätigt, dass diese Beziehung fehlt.
    expect(
      await screen.findByText(
        "Mit Ihrem Konto ist noch kein Kind verknüpft. Die OGS Ihrer Schule kann das einrichten.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Kind A Test")).not.toBeInTheDocument();
  });
  it("starts child features and day status without waiting for the profile", async () => {
    vi.mocked(fetchParentProfile).mockReturnValue(new Promise(() => undefined));
    mount();
    await waitFor(() => expect(getChildFeatures).toHaveBeenCalledWith("42"));
    expect(getChildToday).toHaveBeenCalledWith("42");
    expect(await screen.findByText("Kind A Test")).toBeInTheDocument();
  });
  it("never paints the no-children state while presenting a load failure", async () => {
    const commits: string[] = [];
    vi.mocked(listMyChildren).mockRejectedValue(
      new ApiError("offline", 503, { code: "general.unavailable" }),
    );
    mount(() => commits.push(document.body.textContent ?? ""));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(
      commits.some((text) =>
        text.includes("Mit Ihrem Konto ist noch kein Kind verknüpft"),
      ),
    ).toBe(false);
  });
  it("never paints all-done while presenting an incomplete todo feed", async () => {
    const commits: string[] = [];
    let rejectFeed!: (error: unknown) => void;
    vi.mocked(listAnnouncements).mockReturnValue(
      new Promise((_resolve, reject) => {
        rejectFeed = reject;
      }),
    );
    mount(() => commits.push(document.body.textContent ?? ""));
    expect(await screen.findByText("Kind A Test")).toBeInTheDocument();
    await act(async () => {
      rejectFeed(new ApiError("offline", 503, { code: "general.unavailable" }));
    });
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(commits.some((text) => text.includes("Alles erledigt"))).toBe(false);
  });
  it("revalidates messages without repeating unrelated child requests", async () => {
    mount();
    expect(await screen.findByText("Kind A Test")).toBeInTheDocument();
    expect(listMessageThreads).toHaveBeenCalledTimes(1);
    act(() => {
      window.dispatchEvent(new CustomEvent("parent-threads-refresh"));
    });
    await waitFor(() => expect(listMessageThreads).toHaveBeenCalledTimes(2));
    expect(listMyChildren).toHaveBeenCalledTimes(1);
    expect(getChildFeatures).toHaveBeenCalledTimes(1);
    expect(getChildToday).toHaveBeenCalledTimes(1);
  });
});
