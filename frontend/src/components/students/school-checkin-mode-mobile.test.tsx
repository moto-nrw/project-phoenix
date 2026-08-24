import { afterEach, beforeEach, describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

import { SchoolCheckinModeMobile } from "./school-checkin-mode-mobile";

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

describe("SchoolCheckinModeMobile", () => {
  beforeEach(() => {
    stubMatchMedia(true);
    document.documentElement.style.removeProperty("--moto-checkin-bar-offset");
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    document.documentElement.style.removeProperty("--moto-checkin-bar-offset");
  });

  describe("inactive (OFF) state", () => {
    it("renders an inline pill with the idle label", () => {
      render(
        <SchoolCheckinModeMobile
          isActive={false}
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
        />,
      );

      const button = screen.getByRole("button", {
        name: /Kinder an- und abmelden/i,
      });
      expect(button).toBeInTheDocument();
      expect(button).toHaveAttribute("aria-pressed", "false");
      expect(button.className).not.toContain("fixed");
    });

    it("calls onToggle when the inline pill is clicked", () => {
      const onToggle = vi.fn();
      render(
        <SchoolCheckinModeMobile
          isActive={false}
          onToggle={onToggle}
          successCount={0}
          pendingCount={0}
        />,
      );
      fireEvent.click(screen.getByRole("button"));
      expect(onToggle).toHaveBeenCalledOnce();
    });
  });

  describe("active (ON) state", () => {
    it("renders a fixed sticky bar above the mobile nav", () => {
      const { container } = render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
        />,
      );

      const wrapper = container.querySelector(
        '[data-checkin-mode-mobile="active"]',
      );
      expect(wrapper).toBeInTheDocument();
      // jsdom strips `env()` from inline styles, so we can't assert on the
      // exact bottom value here — the visual offset is verified manually.
      // What we *can* check: the wrapper is fixed-positioned at the
      // bottom of the viewport (left/right pinned, fixed class set).
      expect(wrapper?.className).toContain("fixed");
      expect(wrapper?.className).toContain("right-0");
      expect(wrapper?.className).toContain("left-0");
    });

    it("shows a compact ready state before the first action", () => {
      render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
        />,
      );

      expect(screen.getByText("Bereit")).toBeInTheDocument();
    });

    it("surfaces the success count once the user has toggled at least one", () => {
      render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={() => undefined}
          successCount={3}
          pendingCount={0}
        />,
      );

      expect(screen.getByText("3 bearbeitet")).toBeInTheDocument();
    });

    it("surfaces a pending hint while API calls are in flight", () => {
      render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={() => undefined}
          successCount={2}
          pendingCount={1}
        />,
      );

      // Pending text is rendered alongside the count in the same line.
      const counter = screen.getByText(/2 bearbeitet/i);
      expect(counter.parentElement?.textContent).toContain("1 laufen");
    });

    it("calls onToggle when the Fertig button is clicked", () => {
      const onToggle = vi.fn();
      render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={onToggle}
          successCount={5}
          pendingCount={0}
        />,
      );
      fireEvent.click(
        screen.getByRole("button", {
          name: /An- und Abmelde-Modus beenden/i,
        }),
      );
      expect(onToggle).toHaveBeenCalledOnce();
    });

    it("locks the Fertig button while disabled (bulk request in flight)", () => {
      const onToggle = vi.fn();
      render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={onToggle}
          successCount={0}
          pendingCount={2}
          selectionActive
          selectedCount={2}
          disabled
        />,
      );

      const fertig = screen.getByRole("button", {
        name: /An- und Abmelde-Modus beenden/i,
      });
      expect(fertig).toBeDisabled();
      fireEvent.click(fertig);
      expect(onToggle).not.toHaveBeenCalled();
    });

    it("shows compact bulk actions in the selection mode", () => {
      const onBulkAction = vi.fn();
      const onClearSelection = vi.fn();
      render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
          selectionActive
          selectedCount={3}
          onBulkAction={onBulkAction}
          onClearSelection={onClearSelection}
        />,
      );

      expect(screen.getByText("3 ausgewählt")).toBeInTheDocument();
      const checkIn = screen.getByRole("button", { name: "Anmelden" });
      const checkOut = screen.getByRole("button", { name: "Abmelden" });
      expect(checkIn).toHaveClass("bg-moto-green", "text-white", "rounded-lg");
      expect(checkOut).toHaveClass("bg-moto-red", "text-white", "rounded-lg");

      fireEvent.click(checkIn);
      fireEvent.click(checkOut);
      fireEvent.click(screen.getByRole("button", { name: "Auswahl aufheben" }));

      expect(onBulkAction).toHaveBeenNthCalledWith(1, "in");
      expect(onBulkAction).toHaveBeenNthCalledWith(2, "out");
      expect(onClearSelection).toHaveBeenCalledOnce();
    });

    it("shows the running state only on the selected bulk action", () => {
      render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
          selectionActive
          selectedCount={3}
          runningAction="in"
          disabled
        />,
      );

      expect(
        screen
          .getByRole("button", { name: "Anmelden" })
          .querySelector(".animate-spin"),
      ).toBeInTheDocument();
      expect(
        screen
          .getByRole("button", { name: "Abmelden" })
          .querySelector(".animate-spin"),
      ).not.toBeInTheDocument();
    });

    it("disables selection actions when their callbacks are missing", () => {
      render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
          selectionActive
          selectedCount={3}
        />,
      );

      expect(screen.getByRole("button", { name: "Anmelden" })).toBeDisabled();
      expect(screen.getByRole("button", { name: "Abmelden" })).toBeDisabled();
      expect(
        screen.getByRole("button", { name: "Auswahl aufheben" }),
      ).toBeDisabled();
    });

    it("switches between direct and multiple actions in the bottom bar", () => {
      const onSelectionActiveChange = vi.fn();
      render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
          onSelectionActiveChange={onSelectionActiveChange}
        />,
      );

      fireEvent.click(screen.getByRole("button", { name: "Mehrere" }));
      expect(onSelectionActiveChange).toHaveBeenCalledWith(true);
    });

    it("publishes install-hint clearance only below md", () => {
      const { unmount } = render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
        />,
      );

      expect(window.matchMedia).toHaveBeenCalledWith("(max-width: 767px)");
      expect(
        document.documentElement.style.getPropertyValue(
          "--moto-checkin-bar-offset",
        ),
      ).toBe("4.5rem");

      unmount();
      expect(
        document.documentElement.style.getPropertyValue(
          "--moto-checkin-bar-offset",
        ),
      ).toBe("");
    });

    it("reserves more space for the selection toolbar", () => {
      render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
          selectionActive
          selectedCount={2}
        />,
      );

      expect(
        document.documentElement.style.getPropertyValue(
          "--moto-checkin-bar-offset",
        ),
      ).toBe("6.75rem");
    });

    it("does not publish install-hint clearance at md and above", () => {
      stubMatchMedia(false);
      render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
        />,
      );

      expect(
        document.documentElement.style.getPropertyValue(
          "--moto-checkin-bar-offset",
        ),
      ).toBe("");
    });
  });
});
