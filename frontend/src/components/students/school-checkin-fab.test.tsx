import { afterEach, beforeEach, describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

vi.mock("framer-motion", () => ({
  AnimatePresence: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
  motion: new Proxy(
    {},
    {
      get:
        () =>
        ({
          children,
          ...props
        }: { children?: React.ReactNode } & Record<string, unknown>) => {
          // Strip motion-only props before forwarding to a real DOM node so
          // jsdom doesn't warn about unknown attributes.
          const { initial, animate, exit, transition, ...rest } = props as {
            initial?: unknown;
            animate?: unknown;
            exit?: unknown;
            transition?: unknown;
          };
          void initial;
          void animate;
          void exit;
          void transition;
          return (
            <span {...(rest as React.HTMLAttributes<HTMLElement>)}>
              {children}
            </span>
          );
        },
    },
  ),
  useReducedMotion: () => false,
}));

import { SchoolCheckinFab } from "./school-checkin-fab";

function stubMatchMedia(matches: boolean) {
  vi.stubGlobal(
    "matchMedia",
    vi.fn((query: string) => ({
      matches,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  );
}

describe("SchoolCheckinFab", () => {
  beforeEach(() => {
    stubMatchMedia(true);
    document.documentElement.style.removeProperty("--moto-floating-fab-offset");
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    document.documentElement.style.removeProperty("--moto-floating-fab-offset");
  });

  describe("inline variant", () => {
    it("renders the idle label when inactive", () => {
      render(
        <SchoolCheckinFab
          variant="inline"
          isActive={false}
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
        />,
      );

      expect(
        screen.getByRole("button", { name: /Kinder an- und abmelden/i }),
      ).toBeInTheDocument();
      expect(screen.getByText("An- & Abmelden")).toBeInTheDocument();
    });

    it("renders the active label and counter when active", () => {
      render(
        <SchoolCheckinFab
          variant="inline"
          isActive
          onToggle={() => undefined}
          successCount={3}
          pendingCount={0}
        />,
      );

      const button = screen.getByRole("button", {
        name: /An- und Abmelde-Modus beenden/i,
      });
      expect(button).toHaveAttribute("aria-pressed", "true");
      expect(screen.getByText("Fertig")).toBeInTheDocument();
      expect(screen.getByText("· 3")).toBeInTheDocument();
    });

    it("calls onToggle when clicked", () => {
      const onToggle = vi.fn();
      render(
        <SchoolCheckinFab
          variant="inline"
          isActive={false}
          onToggle={onToggle}
          successCount={0}
          pendingCount={0}
        />,
      );

      fireEvent.click(screen.getByRole("button"));
      expect(onToggle).toHaveBeenCalledOnce();
    });

    it("locks the trigger while disabled (bulk request in flight)", () => {
      const onToggle = vi.fn();
      render(
        <SchoolCheckinFab
          variant="inline"
          isActive
          onToggle={onToggle}
          successCount={0}
          pendingCount={1}
          disabled
        />,
      );

      const button = screen.getByRole("button", {
        name: /An- und Abmelde-Modus beenden/i,
      });
      expect(button).toBeDisabled();
      fireEvent.click(button);
      expect(onToggle).not.toHaveBeenCalled();
    });

    it("does not render the floating positioning wrapper", () => {
      const { container } = render(
        <SchoolCheckinFab
          variant="inline"
          isActive={false}
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
        />,
      );

      const wrapper = container.firstChild as HTMLElement;
      expect(wrapper.className).not.toContain("fixed");
    });
  });

  describe("floating variant", () => {
    it("anchors itself bottom-right with safe-area padding", () => {
      const { container } = render(
        <SchoolCheckinFab
          variant="floating"
          floatingUntil="lg"
          isActive={false}
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
        />,
      );

      const wrapper = container.firstChild as HTMLElement;
      expect(wrapper.className).toContain("fixed");
      expect(wrapper.className).toContain("right-4");
      // Bottom offset clears the mobile nav (~4.75rem visible) + safe-area
      // with a small breathing gap.
      expect(wrapper.className).toContain(
        "bottom-[calc(5.5rem+env(safe-area-inset-bottom))]",
      );
    });

    it("exposes the resolved variant on a data attribute for E2E selectors", () => {
      render(
        <SchoolCheckinFab
          variant="floating"
          floatingUntil="lg"
          isActive={false}
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
        />,
      );

      const button = screen.getByRole("button");
      expect(button).toHaveAttribute("data-checkin-fab-variant", "floating");
    });

    it("publishes stack clearance only within its visible breakpoint range", () => {
      const { unmount } = render(
        <SchoolCheckinFab
          variant="floating"
          floatingUntil="xl"
          isActive={false}
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
        />,
      );

      expect(window.matchMedia).toHaveBeenCalledWith(
        "(min-width: 768px) and (max-width: 1279px)",
      );
      expect(
        document.documentElement.style.getPropertyValue(
          "--moto-floating-fab-offset",
        ),
      ).toBe("4.5rem");

      unmount();
      expect(
        document.documentElement.style.getPropertyValue(
          "--moto-floating-fab-offset",
        ),
      ).toBe("");
    });

    it("does not publish stack clearance while CSS hides the FAB", () => {
      stubMatchMedia(false);
      render(
        <SchoolCheckinFab
          variant="floating"
          floatingUntil="lg"
          isActive={false}
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
        />,
      );

      expect(
        document.documentElement.style.getPropertyValue(
          "--moto-floating-fab-offset",
        ),
      ).toBe("");
    });
  });

  describe("pending badge", () => {
    it("renders a pending spinner when pendingCount > 0", () => {
      render(
        <SchoolCheckinFab
          variant="inline"
          isActive
          onToggle={() => undefined}
          successCount={1}
          pendingCount={2}
        />,
      );

      expect(screen.getByLabelText(/2 Aktionen laufen/i)).toBeInTheDocument();
    });

    it("hides the pending spinner when pendingCount is 0", () => {
      render(
        <SchoolCheckinFab
          variant="inline"
          isActive
          onToggle={() => undefined}
          successCount={1}
          pendingCount={0}
        />,
      );

      expect(screen.queryByLabelText(/Aktionen laufen/i)).toBeNull();
    });
  });
});
