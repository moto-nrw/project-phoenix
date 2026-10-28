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

    it("shows no status text before the first action", () => {
      render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
          onSelectionActiveChange={() => undefined}
        />,
      );

      // Anker: die aktive Leiste ist gerendert.
      expect(screen.getByText("An- und Abmelde-Modus aktiv")).toBeVisible();
      expect(screen.queryByText("Bereit")).not.toBeInTheDocument();
      expect(screen.queryByText(/bearbeitet|laufen/)).not.toBeInTheDocument();
    });

    it("shows only the pending count while the first taps are in flight", () => {
      render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={() => undefined}
          successCount={0}
          pendingCount={2}
        />,
      );

      expect(screen.getByText("2 laufen")).toBeInTheDocument();
    });

    it("names the active mode and explains Direkt taps", () => {
      render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
          onSelectionActiveChange={() => undefined}
        />,
      );

      expect(screen.getByText("An- und Abmelde-Modus aktiv")).toBeVisible();
      expect(
        screen.getByText("Jeder Tipp meldet sofort an oder ab."),
      ).toBeVisible();
    });

    it("keeps the mode name but drops the Direkt hint in Mehrere", () => {
      render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
          selectionActive
          selectedCount={0}
          onSelectionActiveChange={() => undefined}
        />,
      );

      expect(screen.getByText("An- und Abmelde-Modus aktiv")).toBeVisible();
      expect(
        screen.queryByText("Jeder Tipp meldet sofort an oder ab."),
      ).not.toBeInTheDocument();
    });

    it("names the mode without a sub-mode switch (group page)", () => {
      render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
        />,
      );

      expect(screen.getByText("An- und Abmelde-Modus aktiv")).toBeVisible();
      expect(
        screen.getByText("Jeder Tipp meldet sofort an oder ab."),
      ).toBeVisible();
      expect(
        screen.queryByRole("button", { name: "Mehrere" }),
      ).not.toBeInTheDocument();
    });

    it("renders Fertig as the dark primary action", () => {
      render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
        />,
      );

      const finish = screen.getByRole("button", {
        name: "Fertig, An- und Abmelde-Modus beenden",
      });
      expect(finish).toHaveClass("bg-gray-900", "text-white");
      expect(finish).not.toHaveClass("rounded-full");
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
      expect(checkIn).toHaveClass(
        "bg-moto-green-fill",
        "text-white",
        "rounded-lg",
      );
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
      ).toBe("7.25rem");

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
      ).toBe("9.5rem");
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

  describe("Alle auswählen (#4762)", () => {
    function renderSelection(
      selectAll: Partial<{
        count: number;
        leftOutCount: number;
        allSelected: boolean;
        onSelectAll: () => void;
      }> = {},
    ) {
      const props = {
        count: 8,
        leftOutCount: 2,
        allSelected: false,
        onSelectAll: vi.fn(),
        ...selectAll,
      };
      render(
        <SchoolCheckinModeMobile
          isActive
          onToggle={() => undefined}
          successCount={0}
          pendingCount={0}
          selectionActive
          onSelectionActiveChange={() => undefined}
          selectedCount={0}
          onClearSelection={() => undefined}
          onBulkAction={() => undefined}
          selectAll={props}
        />,
      );
      return props;
    }

    it("marks the children on screen and names the left-out ones", () => {
      const props = renderSelection();
      fireEvent.click(screen.getByRole("button", { name: "Alle 8 auswählen" }));
      expect(props.onSelectAll).toHaveBeenCalledOnce();
      expect(
        screen.getByText(
          "2 ausgelassen: krank, entschuldigt, auf Klassenfahrt oder „Kommt heute nicht“",
        ),
      ).toBeInTheDocument();
    });

    it("reserves room for the left-out line", () => {
      renderSelection();
      expect(
        document.documentElement.style.getPropertyValue(
          "--moto-checkin-bar-offset",
        ),
      ).toBe("11.75rem");
    });

    it("keeps the selection height when nobody is left out", () => {
      renderSelection({ leftOutCount: 0 });
      expect(
        screen.getByRole("button", { name: "Alle 8 auswählen" }),
      ).toBeEnabled();
      expect(screen.queryByText(/ausgelassen/)).not.toBeInTheDocument();
      expect(
        document.documentElement.style.getPropertyValue(
          "--moto-checkin-bar-offset",
        ),
      ).toBe("9.5rem");
    });

    it("is disabled once every child it would mark is marked", () => {
      renderSelection({ allSelected: true });
      expect(
        screen.getByRole("button", { name: "Alle 8 auswählen" }),
      ).toBeDisabled();
    });
  });
});
