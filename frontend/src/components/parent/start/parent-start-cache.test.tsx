import { act, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Session } from "next-auth";
import { SWRConfig } from "~/lib/swr/testing";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { ParentStartPage } from "./parent-start-page";

vi.unmock("swr");
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
  state.account = "A";
});
function Fixture() {
  return (
    <>
      <SWRScopeBoundary />
      <ParentStartPage />
    </>
  );
}
function mount() {
  const cache = new Map();
  return render(<Fixture />, {
    wrapper: ({ children }) => (
      <SWRConfig value={{ provider: () => cache, dedupingInterval: 60_000 }}>
        {children}
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
});
