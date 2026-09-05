import { fireEvent, render, screen } from "@testing-library/react";
import {
  AppRouterContext,
  type AppRouterInstance,
} from "next/dist/shared/lib/app-router-context.shared-runtime";
import Link from "next/link";
import { useContext, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

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

describe("NavigationProgress", () => {
  beforeEach(() => {
    linkStatus.pending = false;
    route.pathname = "/dienstplan";
    route.search = "";
    router.push.mockClear();
    router.replace.mockClear();
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

  it.each(["push", "replace"] as const)(
    "shows progress for router.%s until the target URL is active",
    (method) => {
      function ProgrammaticNavigation() {
        const progressRouter = useContext(AppRouterContext);
        return (
          <button
            type="button"
            onClick={() => progressRouter?.[method]("/calendar-periods")}
          >
            Zu den Planungszeiträumen
          </button>
        );
      }

      const rendered = renderShell(<ProgrammaticNavigation />);

      fireEvent.click(
        screen.getByRole("button", { name: "Zu den Planungszeiträumen" }),
      );

      expect(router[method]).toHaveBeenCalledWith("/calendar-periods");
      expect(screen.getByTestId("navigation-progress")).toBeInTheDocument();

      route.pathname = "/calendar-periods";
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
