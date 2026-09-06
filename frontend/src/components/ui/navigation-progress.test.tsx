import { act, fireEvent, render, screen } from "@testing-library/react";
import {
  AppRouterContext,
  type AppRouterInstance,
} from "next/dist/shared/lib/app-router-context.shared-runtime";
import NextLink from "next/link";
import Link from "./navigation-link";
import { useContext, type MouseEvent, type ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const route = vi.hoisted(() => ({ pathname: "/dienstplan", search: "" }));
type RouterMock = Pick<
  AppRouterInstance,
  "back" | "forward" | "prefetch" | "push" | "refresh" | "replace"
>;
const router = vi.hoisted(
  () =>
    ({
      back: vi.fn(),
      forward: vi.fn(),
      prefetch: vi.fn(),
      push: vi.fn(),
      refresh: vi.fn(),
      replace: vi.fn(),
    }) satisfies RouterMock,
);

vi.mock("next/link", async () => {
  const { AppRouterContext } =
    await import("next/dist/shared/lib/app-router-context.shared-runtime");
  const { useContext } = await import("react");

  return {
    default: ({
      children,
      href,
      onClick,
      onNavigate,
      ...rest
    }: {
      children?: React.ReactNode;
      href: string;
      onClick?: React.MouseEventHandler<HTMLAnchorElement>;
      onNavigate?: (event: { preventDefault: () => void }) => void;
    }) => {
      const router = useContext(AppRouterContext) as AppRouterInstance | null;

      return (
        <a
          href={href}
          {...rest}
          onClick={(event) => {
            onClick?.(event);
            if (event.defaultPrevented) return;

            let cancelled = false;
            onNavigate?.({
              preventDefault: () => {
                cancelled = true;
              },
            });
            if (cancelled) return;

            event.preventDefault();
            router?.push(href);
          }}
        >
          {children}
        </a>
      );
    },
  };
});

vi.mock("next/navigation", () => ({
  usePathname: () => route.pathname,
  useSearchParams: () => new URLSearchParams(route.search),
}));

import {
  NavigationProgressBar,
  NavigationProgressProvider,
} from "./navigation-progress";
import ProtectedLoading from "~/app/[tenant]/(protected)/loading";

const appRouter = router as unknown as AppRouterInstance;

function useProgressRouter(): AppRouterInstance | null {
  return useContext(AppRouterContext) as AppRouterInstance | null;
}

function renderShell(children?: ReactNode) {
  return render(
    <AppRouterContext.Provider value={appRouter}>
      <NavigationProgressProvider>
        <NavigationProgressBar />
        {children ?? <Link href="/calendar-periods">Planungszeiträume</Link>}
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

  it("tracks a NavigationLink before its loading boundary renders", () => {
    const rendered = renderShell(
      <>
        <ProtectedLoading />
        <Link href="/calendar-periods">Planungszeiträume</Link>
      </>,
    );

    expect(screen.getByLabelText("Lädt...")).toBeVisible();
    fireEvent.click(screen.getByRole("link", { name: "Planungszeiträume" }));

    expect(screen.getByTestId("navigation-progress")).toHaveClass(
      "moto-nav-progress",
    );
    expect(screen.getByRole("status")).toHaveTextContent("Seite wird geladen");
    expect(screen.queryByLabelText("Lädt...")).toBeNull();

    navigateTo("/calendar-periods");
    rendered.rerender(
      <AppRouterContext.Provider value={appRouter}>
        <NavigationProgressProvider>
          <NavigationProgressBar />
          <ProtectedLoading />
          <Link href="/calendar-periods">Planungszeiträume</Link>
        </NavigationProgressProvider>
      </AppRouterContext.Provider>,
    );

    expect(screen.queryByTestId("navigation-progress")).toBeNull();
  });

  it("does not start progress when a Link click is cancelled", () => {
    renderShell(
      <Link
        href="/calendar-periods"
        onClick={(event: MouseEvent<HTMLAnchorElement>) =>
          event.preventDefault()
        }
      >
        Planungszeiträume
      </Link>,
    );
    fireEvent.click(screen.getByRole("link", { name: "Planungszeiträume" }));

    expect(screen.queryByTestId("navigation-progress")).toBeNull();
  });

  it("completes a redirected Link without a second pending entry", () => {
    navigateTo("/rooms");
    const rendered = renderShell(
      <Link href="/staff/dienstplan">Zum Dienstplan</Link>,
    );
    fireEvent.click(screen.getByRole("link", { name: "Zum Dienstplan" }));

    expect(screen.getByTestId("navigation-progress")).toBeInTheDocument();

    navigateTo("/dienstplan");
    rendered.rerender(
      <AppRouterContext.Provider value={appRouter}>
        <NavigationProgressProvider>
          <NavigationProgressBar />
          <Link href="/staff/dienstplan">Zum Dienstplan</Link>
        </NavigationProgressProvider>
      </AppRouterContext.Provider>,
    );

    expect(screen.queryByTestId("navigation-progress")).toBeNull();
  });

  it("reuses a Link's progress entry when its click also calls router.push", () => {
    function LinkedProgrammaticNavigation() {
      const progressRouter = useProgressRouter();
      return (
        <Link
          href="/staff/dienstplan"
          onClick={() => progressRouter?.push("/staff/dienstplan")}
        >
          Zum Dienstplan
        </Link>
      );
    }

    navigateTo("/rooms");
    const rendered = renderShell(<LinkedProgrammaticNavigation />);
    fireEvent.click(screen.getByRole("link", { name: "Zum Dienstplan" }));

    expect(router.push).toHaveBeenCalledWith("/staff/dienstplan");
    expect(screen.getByTestId("navigation-progress")).toBeInTheDocument();

    navigateTo("/dienstplan");
    rendered.rerender(
      <AppRouterContext.Provider value={appRouter}>
        <NavigationProgressProvider>
          <NavigationProgressBar />
          <LinkedProgrammaticNavigation />
        </NavigationProgressProvider>
      </AppRouterContext.Provider>,
    );

    expect(screen.queryByTestId("navigation-progress")).toBeNull();
  });

  it.each(["push", "replace"] as const)(
    "shows progress for router.%s until the target URL is active",
    (method) => {
      function ProgrammaticNavigation() {
        const progressRouter = useProgressRouter();
        return (
          <button
            type="button"
            onClick={() => {
              if (!progressRouter) return;
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

      expect(router[method]).toHaveBeenCalledWith("/calendar-periods");
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
      const progressRouter = useProgressRouter();
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

  it("keeps progress for repeated targets until every navigation commits", () => {
    function ProgrammaticNavigation() {
      const progressRouter = useProgressRouter();
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
    fireEvent.click(screen.getByRole("button", { name: "Erstes Ziel" }));

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

    expect(screen.getByTestId("navigation-progress")).toBeInTheDocument();

    navigateTo("/first");
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

  it("shows the protected fallback initially but not during a pending navigation", () => {
    function ProgrammaticNavigation() {
      const progressRouter = useProgressRouter();
      return (
        <button
          type="button"
          onClick={() => progressRouter?.push("/calendar-periods")}
        >
          Zu den Planungszeiträumen
        </button>
      );
    }

    renderShell(
      <>
        <ProtectedLoading />
        <ProgrammaticNavigation />
      </>,
    );

    expect(screen.getByLabelText("Lädt...")).toBeVisible();
    fireEvent.click(
      screen.getByRole("button", { name: "Zu den Planungszeiträumen" }),
    );

    expect(screen.queryByLabelText("Lädt...")).toBeNull();
  });

  it("tracks a native content link before its loading boundary renders", () => {
    renderShell(
      <>
        <ProtectedLoading />
        <NextLink href="/calendar-periods">Planungszeiträume</NextLink>
      </>,
    );

    expect(screen.getByLabelText("Lädt...")).toBeVisible();
    fireEvent.click(screen.getByRole("link", { name: "Planungszeiträume" }));

    expect(screen.getByTestId("navigation-progress")).toBeInTheDocument();
    expect(screen.queryByLabelText("Lädt...")).toBeNull();
  });

  it("tracks a native query link before its loading boundary renders", () => {
    navigateTo("/calendar-periods?view=week");
    renderShell(
      <>
        <ProtectedLoading />
        <NextLink href="/calendar-periods?view=month">Monatsansicht</NextLink>
      </>,
    );

    expect(screen.getByLabelText("Lädt...")).toBeVisible();
    fireEvent.click(screen.getByRole("link", { name: "Monatsansicht" }));

    expect(screen.getByTestId("navigation-progress")).toBeInTheDocument();
    expect(screen.queryByLabelText("Lädt...")).toBeNull();
  });

  it("restores the fallback after a cancelled native link click", async () => {
    renderShell(
      <>
        <ProtectedLoading />
        <NextLink
          href="/calendar-periods"
          onClick={(event) => event.preventDefault()}
        >
          Planungszeiträume
        </NextLink>
      </>,
    );

    fireEvent.click(screen.getByRole("link", { name: "Planungszeiträume" }));
    await act(async () => {});

    expect(screen.queryByTestId("navigation-progress")).toBeNull();
    expect(screen.getByLabelText("Lädt...")).toBeVisible();
  });

  it("does not show progress when router.back cannot traverse history", () => {
    function ProgrammaticNavigation() {
      const progressRouter = useProgressRouter();
      return (
        <button type="button" onClick={() => progressRouter?.back()}>
          Zurück
        </button>
      );
    }

    renderShell(<ProgrammaticNavigation />);
    fireEvent.click(screen.getByRole("button", { name: "Zurück" }));
    expect(screen.queryByTestId("navigation-progress")).toBeNull();
  });

  it("does not show progress when popstate keeps the current URL", () => {
    renderShell();

    act(() => {
      window.dispatchEvent(new PopStateEvent("popstate"));
    });

    expect(screen.queryByTestId("navigation-progress")).toBeNull();
  });

  it("shows progress for a browser history navigation", () => {
    navigateTo("/rooms");
    const rendered = renderShell();

    act(() => {
      window.history.replaceState({}, "", "/calendar-periods");
      window.dispatchEvent(new PopStateEvent("popstate"));
    });

    expect(screen.getByTestId("navigation-progress")).toBeInTheDocument();

    route.pathname = "/calendar-periods";
    rendered.rerender(
      <AppRouterContext.Provider value={appRouter}>
        <NavigationProgressProvider>
          <NavigationProgressBar />
        </NavigationProgressProvider>
      </AppRouterContext.Provider>,
    );

    expect(screen.queryByTestId("navigation-progress")).toBeNull();
  });

  it("completes a history navigation when its query uses encoded spaces", () => {
    navigateTo("/rooms");
    const rendered = renderShell();

    act(() => {
      window.history.replaceState({}, "", "/calendar-periods?q=a%20b");
      window.dispatchEvent(new PopStateEvent("popstate"));
    });

    route.pathname = "/calendar-periods";
    route.search = "?q=a+b";
    rendered.rerender(
      <AppRouterContext.Provider value={appRouter}>
        <NavigationProgressProvider>
          <NavigationProgressBar />
        </NavigationProgressProvider>
      </AppRouterContext.Provider>,
    );

    expect(screen.queryByTestId("navigation-progress")).toBeNull();
  });

  it("completes a redirected history navigation at its canonical URL", () => {
    navigateTo("/rooms");
    const rendered = renderShell();

    act(() => {
      window.history.replaceState({}, "", "/staff/dienstplan");
      window.dispatchEvent(new PopStateEvent("popstate"));
    });

    expect(screen.getByTestId("navigation-progress")).toBeInTheDocument();

    navigateTo("/dienstplan");
    rendered.rerender(
      <AppRouterContext.Provider value={appRouter}>
        <NavigationProgressProvider>
          <NavigationProgressBar />
        </NavigationProgressProvider>
      </AppRouterContext.Provider>,
    );

    expect(screen.queryByTestId("navigation-progress")).toBeNull();
  });

  it("does not start progress when a history sentinel preserves an encoded URL", () => {
    navigateTo("/meal-plan?q=a%20b");
    renderShell();

    act(() => {
      window.dispatchEvent(new PopStateEvent("popstate"));
    });

    expect(screen.queryByTestId("navigation-progress")).toBeNull();
  });

  it("keeps progress visible until the final rapid history destination commits", () => {
    navigateTo("/rooms");
    const rendered = renderShell();

    act(() => {
      window.history.replaceState({}, "", "/calendar-periods");
      window.dispatchEvent(new PopStateEvent("popstate"));
      window.history.replaceState({}, "", "/staff");
      window.dispatchEvent(new PopStateEvent("popstate"));
    });

    route.pathname = "/calendar-periods";
    rendered.rerender(
      <AppRouterContext.Provider value={appRouter}>
        <NavigationProgressProvider>
          <NavigationProgressBar />
        </NavigationProgressProvider>
      </AppRouterContext.Provider>,
    );

    expect(screen.getByTestId("navigation-progress")).toBeInTheDocument();

    route.pathname = "/staff";
    rendered.rerender(
      <AppRouterContext.Provider value={appRouter}>
        <NavigationProgressProvider>
          <NavigationProgressBar />
        </NavigationProgressProvider>
      </AppRouterContext.Provider>,
    );

    expect(screen.queryByTestId("navigation-progress")).toBeNull();
  });

  it("completes a history navigation to a previously requested target", () => {
    function ProgrammaticNavigation() {
      const progressRouter = useProgressRouter();
      return (
        <button type="button" onClick={() => progressRouter?.push("/first")}>
          Erstes Ziel
        </button>
      );
    }

    navigateTo("/rooms");
    const rendered = renderShell(<ProgrammaticNavigation />);
    fireEvent.click(screen.getByRole("button", { name: "Erstes Ziel" }));
    expect(screen.getByTestId("navigation-progress")).toBeInTheDocument();

    act(() => {
      window.history.replaceState({}, "", "/first");
      window.dispatchEvent(new PopStateEvent("popstate"));
    });
    route.pathname = "/first";
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

  it("ends a programmatic navigation after a redirected destination is active", () => {
    function ProgrammaticNavigation() {
      const progressRouter = useProgressRouter();
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
      const progressRouter = useProgressRouter();
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
    renderShell();

    // Ein Bereich, der erst beim Wechsel eingehängt wird, wird von
    // Screenreadern nicht angesagt — er muss vorher da sein.
    expect(screen.getByRole("status")).toHaveAttribute("aria-live", "polite");
  });

  it("renders nothing outside a provider instead of throwing", () => {
    render(<NavigationProgressBar />);

    expect(screen.queryByTestId("navigation-progress")).toBeNull();
  });
});
