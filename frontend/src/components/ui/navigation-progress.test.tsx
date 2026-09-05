import { act, fireEvent, render, screen } from "@testing-library/react";
import {
  AppRouterContext,
  type AppRouterInstance,
} from "next/dist/shared/lib/app-router-context.shared-runtime";
import Link from "next/link";
import { useContext, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const linkStatus = vi.hoisted(() => ({ pending: false }));
const route = vi.hoisted(() => ({ pathname: "/dienstplan", search: "" }));
const router = vi.hoisted(() => ({
  back: vi.fn(),
  forward: vi.fn(),
  prefetch: vi.fn(),
  push: vi.fn(),
  refresh: vi.fn(),
  replace: vi.fn(),
}));

vi.mock("next/link", () => ({
  useLinkStatus: () => linkStatus,
  default: ({
    children,
    href,
  }: {
    children?: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));

vi.mock("next/navigation", () => ({
  usePathname: () => route.pathname,
  useSearchParams: () => new URLSearchParams(route.search),
}));

import {
  NavigationProgressBar,
  NavigationProgressProvider,
  NavigationProgressReporter,
} from "./navigation-progress";

const appRouter = router as unknown as AppRouterInstance;

function renderShell(children?: ReactNode) {
  return render(
    <AppRouterContext.Provider value={appRouter}>
      <NavigationProgressProvider>
        <NavigationProgressBar />
        {children ?? (
          <Link href="/calendar-periods">
            <NavigationProgressReporter />
          </Link>
        )}
      </NavigationProgressProvider>
    </AppRouterContext.Provider>,
  );
}

function navigateTo(url: string) {
  window.history.replaceState({}, "", url);
  const location = new URL(url, window.location.origin);
  route.pathname = location.pathname;
  route.search = location.search;
}

describe("NavigationProgress", () => {
  beforeEach(() => {
    linkStatus.pending = false;
    navigateTo("/dienstplan");
    router.push.mockClear();
    router.replace.mockClear();
    router.back.mockClear();
    router.forward.mockClear();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("shows nothing while no navigation is pending", () => {
    renderShell();

    expect(screen.queryByTestId("navigation-progress")).toBeNull();
    expect(screen.getByRole("status")).toHaveTextContent("");
  });

  it("shows the bar and announces the load when the Link reports a pending navigation", () => {
    const rendered = renderShell();
    linkStatus.pending = true;
    rendered.rerender(
      <AppRouterContext.Provider value={appRouter}>
        <NavigationProgressProvider>
          <NavigationProgressBar />
          <Link href="/calendar-periods">
            <NavigationProgressReporter />
          </Link>
        </NavigationProgressProvider>
      </AppRouterContext.Provider>,
    );

    expect(screen.getByTestId("navigation-progress")).toHaveClass(
      "moto-nav-progress",
    );
    expect(screen.getByRole("status")).toHaveTextContent("Seite wird geladen");
  });

  it.each(["push", "replace", "back", "forward"] as const)(
    "shows progress for router.%s until the target URL is active",
    (method) => {
      const isHistoryNavigation = method === "back" || method === "forward";

      function ProgrammaticNavigation() {
        const progressRouter = useContext(AppRouterContext);
        return (
          <button
            type="button"
            onClick={() => {
              if (!progressRouter) return;
              if (method === "back" || method === "forward") {
                progressRouter[method]();
                return;
              }
              progressRouter[method]("/calendar-periods");
            }}
          >
            Zu den Planungszeiträumen
          </button>
        );
      }

      const rendered = renderShell(<ProgrammaticNavigation />);

      fireEvent.click(
        screen.getByRole("button", { name: "Zu den Planungszeiträumen" }),
      );

      if (isHistoryNavigation) {
        expect(router[method]).toHaveBeenCalledWith();
      } else {
        expect(router[method]).toHaveBeenCalledWith("/calendar-periods");
      }
      expect(screen.getByTestId("navigation-progress")).toBeInTheDocument();

      navigateTo("/calendar-periods");
      rendered.rerender(
        <AppRouterContext.Provider value={appRouter}>
          <NavigationProgressProvider>
            <NavigationProgressBar />
            <ProgrammaticNavigation />
          </NavigationProgressProvider>
        </AppRouterContext.Provider>,
      );

      expect(screen.queryByTestId("navigation-progress")).toBeNull();
    },
  );

  it("keeps the latest programmatic navigation pending until its target URL is active", () => {
    function ProgrammaticNavigation() {
      const progressRouter = useContext(AppRouterContext);
      return (
        <>
          <button type="button" onClick={() => progressRouter?.push("/first")}>
            Erstes Ziel
          </button>
          <button
            type="button"
            onClick={() => progressRouter?.push("/calendar-periods")}
          >
            Zweites Ziel
          </button>
        </>
      );
    }

    const rendered = renderShell(<ProgrammaticNavigation />);
    fireEvent.click(screen.getByRole("button", { name: "Erstes Ziel" }));
    fireEvent.click(screen.getByRole("button", { name: "Zweites Ziel" }));

    navigateTo("/first");
    rendered.rerender(
      <AppRouterContext.Provider value={appRouter}>
        <NavigationProgressProvider>
          <NavigationProgressBar />
          <ProgrammaticNavigation />
        </NavigationProgressProvider>
      </AppRouterContext.Provider>,
    );
    expect(screen.getByTestId("navigation-progress")).toBeInTheDocument();

    navigateTo("/calendar-periods");
    rendered.rerender(
      <AppRouterContext.Provider value={appRouter}>
        <NavigationProgressProvider>
          <NavigationProgressBar />
          <ProgrammaticNavigation />
        </NavigationProgressProvider>
      </AppRouterContext.Provider>,
    );
    expect(screen.queryByTestId("navigation-progress")).toBeNull();
  });

  it("ends a history navigation after a popstate event without a URL change", () => {
    function ProgrammaticNavigation() {
      const progressRouter = useContext(AppRouterContext);
      return (
        <button type="button" onClick={() => progressRouter?.back()}>
          Zurück
        </button>
      );
    }

    renderShell(<ProgrammaticNavigation />);
    fireEvent.click(screen.getByRole("button", { name: "Zurück" }));
    expect(screen.getByTestId("navigation-progress")).toBeInTheDocument();

    act(() => {
      window.dispatchEvent(new PopStateEvent("popstate"));
    });

    expect(screen.queryByTestId("navigation-progress")).toBeNull();
  });

  it("ends a programmatic navigation after a redirected destination is active", () => {
    function ProgrammaticNavigation() {
      const progressRouter = useContext(AppRouterContext);
      return (
        <button
          type="button"
          onClick={() => progressRouter?.push("/staff/dienstplan")}
        >
          Zum Dienstplan
        </button>
      );
    }

    navigateTo("/rooms");
    const rendered = renderShell(<ProgrammaticNavigation />);
    fireEvent.click(screen.getByRole("button", { name: "Zum Dienstplan" }));
    expect(screen.getByTestId("navigation-progress")).toBeInTheDocument();

    navigateTo("/dienstplan");
    rendered.rerender(
      <AppRouterContext.Provider value={appRouter}>
        <NavigationProgressProvider>
          <NavigationProgressBar />
          <ProgrammaticNavigation />
        </NavigationProgressProvider>
      </AppRouterContext.Provider>,
    );

    expect(screen.queryByTestId("navigation-progress")).toBeNull();
  });

  it("ends an aborted programmatic navigation after the fallback timeout", () => {
    function ProgrammaticNavigation() {
      const progressRouter = useContext(AppRouterContext);
      return (
        <button
          type="button"
          onClick={() => progressRouter?.push("/calendar-periods")}
        >
          Zu den Planungszeiträumen
        </button>
      );
    }

    vi.useFakeTimers();
    renderShell(<ProgrammaticNavigation />);
    fireEvent.click(
      screen.getByRole("button", { name: "Zu den Planungszeiträumen" }),
    );
    expect(screen.getByTestId("navigation-progress")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(10_000);
    });

    expect(screen.queryByTestId("navigation-progress")).toBeNull();
  });

  it("keeps the announcement region mounted so it can be read out at all", () => {
    linkStatus.pending = false;
    renderShell();

    // Ein Bereich, der erst beim Wechsel eingehängt wird, wird von
    // Screenreadern nicht angesagt — er muss vorher da sein.
    expect(screen.getByRole("status")).toHaveAttribute("aria-live", "polite");
  });

  it("renders nothing outside a provider instead of throwing", () => {
    linkStatus.pending = true;
    render(<NavigationProgressBar />);

    expect(screen.queryByTestId("navigation-progress")).toBeNull();
  });
});
