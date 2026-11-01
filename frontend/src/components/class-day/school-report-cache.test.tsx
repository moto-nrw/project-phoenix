import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import type { Session } from "next-auth";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useInvalidateMatching } from "~/lib/swr";
import { SWRConfig } from "~/lib/swr/testing";
import { schoolPortalKeys } from "~/lib/swr/portal-keys";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
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
    cache,
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

  it.each([
    ["overview", 403],
    ["class", 403],
    ["overview", 503],
    ["class", 503],
  ] as const)(
    "handles a %s refresh returning %s without exposing refused reports",
    async (reader, status) => {
      const options = setup();
      const view = render(
        reader === "overview" ? (
          <ClassDayOverview {...options} />
        ) : (
          <ClassDayClass schoolClass="4a" {...options} />
        ),
        options,
      );
      // Anker: Der erfolgreiche Erstabruf zeigt den später entzogenen Report.
      if (reader === "overview")
        expect(await screen.findAllByText(/1 Kinder · 1 bleiben/)).toHaveLength(
          2,
        );
      else expect(await screen.findByText("Klassentag, Klara")).toBeVisible();

      const failure = new ApiError("Report refresh failed", status);
      options.fetchClassDay.mockImplementation((klass: string) =>
        klass === "4a" ? Promise.reject(failure) : Promise.resolve(report),
      );
      await act(async () => {
        await options.refresh();
      });
      if (status === 403)
        expect(await screen.findByRole("alert")).toBeVisible();
      const cachedReport = [...options.cache.entries()].find(([key]) =>
        key.endsWith(`:${schoolPortalKeys.classDayReport("4a", "2026-09-09")}`),
      )?.[1];
      expect(cachedReport).toBeDefined();
      expect(cachedReport.data).toBe(status === 403 ? undefined : report);
      expect(cachedReport.error).toBe(failure);
      if (reader === "overview")
        expect(screen.getAllByText(/1 Kinder · 1 bleiben/)).toHaveLength(
          status === 403 ? 1 : 2,
        );
      if (reader === "class" && status === 403)
        expect(screen.queryByText("Klassentag, Klara")).not.toBeInTheDocument();

      // Reopening must not reveal a refused cached report, even while the
      // next request is pending. Transient failures retain the warm report.
      options.fetchClassDay.mockImplementation(
        () => new Promise(() => undefined),
      );
      view.rerender(
        <ClassDayClass key="reopened" schoolClass="4a" {...options} />,
      );
      if (status === 403) {
        // Anker: Der neue Leser zeigt weiter den echten Zugriffsfehler.
        expect(await screen.findByRole("alert")).toBeVisible();
        expect(screen.queryByText("Klassentag, Klara")).not.toBeInTheDocument();
      } else {
        // Anker: Der neue Leser behält den geladenen Report ohne Fehlerkarte.
        expect(screen.getByText("Klassentag, Klara")).toBeVisible();
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
      }
    },
  );

  it("keeps both loaded summaries and their links after background report failures", async () => {
    const options = setup();
    render(<ClassDayOverview {...options} />, options);
    expect(await screen.findAllByText(/1 Kinder · 1 bleiben/)).toHaveLength(2);
    const links = screen
      .getAllByText(/1 Kinder · 1 bleiben/)
      .map((node) => node.closest("a"));
    await act(async () => {
      await import("~/lib/error-presentation");
    });
    options.fetchClassDay.mockImplementation(async () => {
      throw new ApiError("unavailable", 503);
    });
    await act(async () => {
      await options.refresh();
    });
    // Anker: beide vorher geladenen Zahlen bleiben an denselben Links sichtbar.
    expect(screen.getAllByText(/1 Kinder · 1 bleiben/)).toHaveLength(2);
    expect(
      screen
        .getAllByText(/1 Kinder · 1 bleiben/)
        .map((node) => node.closest("a")),
    ).toEqual(links);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("shows one shared error outside the tiles and retries every missing report", async () => {
    const options = setup();
    options.fetchClassDay.mockImplementation(async () => {
      throw new ApiError("unavailable", 503);
    });
    await act(async () => {
      await import("~/lib/error-presentation");
    });
    render(<ClassDayOverview {...options} />, options);
    const failedLinks = await waitFor(() => {
      const links = screen
        .getAllByText("Konnte nicht geladen werden")
        .map((node) => node.closest("a")!);
      expect(links).toHaveLength(2);
      return links;
    });
    const errors = await screen.findAllByRole("alert");
    expect(errors).toHaveLength(1);
    for (const link of failedLinks)
      expect(link).not.toContainElement(errors[0]!);
    options.fetchClassDay.mockResolvedValue(report);
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    // Anker: ein Retry stellt beide Klassen wieder her, ohne neue Kacheln.
    expect(await screen.findAllByText(/1 Kinder · 1 bleiben/)).toHaveLength(2);
    expect(
      screen
        .getAllByText(/1 Kinder · 1 bleiben/)
        .map((node) => node.closest("a")),
    ).toEqual(failedLinks);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(options.fetchClassDay).toHaveBeenCalledTimes(4);
  });

  it("groups denied reports without a retry and recovers on later revalidation", async () => {
    const options = setup();
    options.fetchClassDay.mockImplementation(async () => {
      throw new ApiError("denied", 403);
    });
    render(<ClassDayOverview {...options} />, options);
    expect(
      await screen.findAllByText("Konnte nicht geladen werden"),
    ).toHaveLength(2);
    expect(await screen.findAllByRole("alert")).toHaveLength(1);
    // Anker: die betroffenen Klassen stehen da, aber keine verweigerten Zahlen.
    expect(screen.getByRole("link", { name: /Klasse 4a/ })).toBeVisible();
    expect(screen.getByRole("link", { name: /Klasse 4b/ })).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Wiederholen" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/1 Kinder · 1 bleiben/)).not.toBeInTheDocument();
    options.fetchClassDay.mockResolvedValue(report);
    await act(async () => {
      await options.refresh();
    });
    // Anker: neue erlaubte Antworten heben den Entzug auf, nicht alte Daten.
    expect(screen.getAllByText(/1 Kinder · 1 bleiben/)).toHaveLength(2);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
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
    "other-tab-school",
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
              newValue: change.endsWith("logout")
                ? null
                : change === "other-tab-school"
                  ? "2:A"
                  : "1:B",
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
      // A fresh lookup of the old school is not confirmation of its replacement.
      if (change === "other-tab-school") {
        signIn();
        view.rerender(<ClassDayClass schoolClass="4a" {...options} />);
        expect(screen.queryByText("Klassentag, Klara")).not.toBeInTheDocument();
        expect(options.fetchClassDay).toHaveBeenCalledTimes(2);
      }
      signIn(
        change === "other-tab-school" ? "A" : "B",
        change.includes("school") ? 2 : 1,
      );
      view.rerender(<ClassDayClass schoolClass="4a" {...options} />);
      expect(screen.queryByText("Klassentag, Klara")).not.toBeInTheDocument();
    },
  );
});

import { resumeSWRSession } from "~/lib/swr/scope";
afterEach(() => resumeSWRSession());
