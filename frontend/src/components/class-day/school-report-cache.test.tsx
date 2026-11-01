import { act, render, screen, waitFor } from "@testing-library/react";
import type { Session } from "next-auth";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useInvalidateMatching } from "~/lib/swr";
import { SWRConfig } from "~/lib/swr/testing";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { ToastProvider } from "~/contexts/ToastContext";
import type { ClassDayReport } from "~/lib/class-day-api";
import { ClassDayOverview } from "./class-day-overview";
import { ClassDayClass } from "./class-day-class";

vi.unmock("swr");
const identity = vi.hoisted(() => ({ session: null as Session | null }));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: identity.session,
    status: identity.session ? "authenticated" : "unauthenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({ useTenantSlugSafe: () => null }));
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams("tag=2026-09-09"),
  useRouter: () => ({ replace: vi.fn() }),
}));
vi.mock("~/lib/school-url", () => ({ schoolPath: (path: string) => path }));
vi.mock("~/lib/session-cache", () => ({ announceSessionChange: vi.fn() }));

const report: ClassDayReport = {
  school_class: "4a",
  date: "2026-09-09",
  weekday: "wed",
  school_day: true,
  enrollment_known: true,
  totals: { students: 1, staying: 1, leaving: 0, absent: 0, list_entries: 0 },
  rows: [
    {
      student_id: 1,
      first_name: "Klara",
      last_name: "Klassentag",
      registered: true,
      stays_today: true,
      offerings: ["Ganztag"],
    },
  ],
};
function signIn(id = "A", tenantId = 1) {
  identity.session = {
    user: { id, tenantId, scope: "school" },
    expires: "2099-01-01",
  } as Session;
}
function setup() {
  const cache = new Map();
  let invalidate: ReturnType<typeof useInvalidateMatching>;
  function Revalidation() {
    invalidate = useInvalidateMatching();
    return null;
  }
  const fetchClasses = vi.fn().mockResolvedValue({
    classes: ["4a", "4b"],
    weekend_follows_friday: false,
  });
  const fetchClassDay = vi.fn().mockResolvedValue(report);
  function Wrapper({ children }: Readonly<{ children: React.ReactNode }>) {
    return (
      <SWRConfig value={{ provider: () => cache, shouldRetryOnError: false }}>
        <ToastProvider>
          <SWRScopeBoundary />
          <Revalidation />
          {children}
        </ToastProvider>
      </SWRConfig>
    );
  }
  return {
    wrapper: Wrapper,
    fetchClasses,
    fetchClassDay,
    refresh: () =>
      invalidate((key) => key.startsWith("school-class-day-report-")),
  };
}
beforeEach(() => signIn());

describe("school report cache", () => {
  it("opens the report loaded by the overview immediately, with the same load key", async () => {
    const options = setup();
    const view = render(<ClassDayOverview {...options} />, options);
    await screen.findAllByText(/1 Kinder · 1 bleiben/);
    expect(options.fetchClassDay).toHaveBeenCalledTimes(2);
    options.fetchClasses.mockImplementation(() => new Promise(() => undefined));
    options.fetchClassDay.mockImplementation(
      () => new Promise(() => undefined),
    );
    view.rerender(<ClassDayClass schoolClass="4a" {...options} />);
    expect(screen.getByText("Klassentag, Klara")).toBeVisible();
    expect(screen.queryByText("Wird geladen…")).not.toBeInTheDocument();
    expect(options.fetchClasses).toHaveBeenCalledTimes(1);
    expect(options.fetchClassDay).toHaveBeenCalledTimes(2);
  });

  it("starts all class reports in parallel without waiting for another report", async () => {
    const options = setup();
    options.fetchClassDay.mockImplementation(
      () => new Promise(() => undefined),
    );
    render(<ClassDayOverview {...options} />, options);
    await waitFor(() => expect(options.fetchClassDay).toHaveBeenCalledTimes(2));
    expect(options.fetchClassDay).toHaveBeenCalledWith("4a", "2026-09-09");
    expect(options.fetchClassDay).toHaveBeenCalledWith("4b", "2026-09-09");
  });

  it.each([
    "account",
    "school",
    "logout",
    "other-tab-account",
    "other-tab-logout",
  ])(
    "withdraws the loaded report after %s, even when an old response arrives",
    async (change) => {
      const options = setup();
      const view = render(
        <ClassDayClass schoolClass="4a" {...options} />,
        options,
      );
      expect(await screen.findByText("Klassentag, Klara")).toBeVisible();
      let finish!: (value: ClassDayReport) => void;
      options.fetchClassDay.mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            finish = resolve;
          }),
      );
      let pending!: Promise<unknown>;
      act(() => {
        pending = options.refresh();
      });
      await waitFor(() =>
        expect(options.fetchClassDay).toHaveBeenCalledTimes(2),
      );

      options.fetchClassDay.mockImplementation(
        () => new Promise(() => undefined),
      );
      if (change.startsWith("other-tab")) {
        act(() =>
          globalThis.dispatchEvent(
            new StorageEvent("storage", {
              key: "moto:personal-storage-owner",
              newValue: change.endsWith("logout") ? null : "1:B",
            }),
          ),
        );
      } else {
        if (change === "logout") identity.session = null;
        else
          signIn(change === "account" ? "B" : "A", change === "school" ? 2 : 1);
        view.rerender(<ClassDayClass schoolClass="4a" {...options} />);
      }
      expect(screen.queryByText("Klassentag, Klara")).not.toBeInTheDocument();
      await act(async () => {
        finish(report);
        await pending;
      });
      expect(screen.queryByText("Klassentag, Klara")).not.toBeInTheDocument();
      // An authenticated next account can load its own report, never the old one.
      signIn("B", change === "school" ? 2 : 1);
      view.rerender(<ClassDayClass schoolClass="4a" {...options} />);
      expect(screen.queryByText("Klassentag, Klara")).not.toBeInTheDocument();
    },
  );
});

import { resumeSWRSession } from "~/lib/swr/scope";
afterEach(() => resumeSWRSession());
