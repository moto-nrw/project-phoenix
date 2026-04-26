import { describe, it, expect, vi } from "vitest";
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

describe("SchoolCheckinFab", () => {
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
        screen.getByRole("button", { name: /Schüler an- und abmelden/i }),
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
          isActive={false}
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
        />,
      );

      const wrapper = container.firstChild as HTMLElement;
      expect(wrapper.className).toContain("fixed");
      expect(wrapper.className).toContain("right-4");
      expect(wrapper.className).toContain(
        "bottom-[max(1rem,env(safe-area-inset-bottom))]",
      );
    });

    it("exposes the resolved variant on a data attribute for E2E selectors", () => {
      render(
        <SchoolCheckinFab
          variant="floating"
          isActive={false}
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
        />,
      );

      const button = screen.getByRole("button");
      expect(button).toHaveAttribute("data-checkin-fab-variant", "floating");
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
