import * as React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

vi.mock("~/components/ui/drawer", () => ({
  Drawer: ({
    children,
    open,
    onOpenChange,
  }: {
    children: React.ReactNode;
    open: boolean;
    onOpenChange: (open: boolean) => void;
  }) => (
    <div data-testid="drawer" data-open={open}>
      <button
        type="button"
        data-testid="drawer-close"
        onClick={() => onOpenChange(false)}
      >
        close
      </button>
      {children}
    </div>
  ),
  DrawerContent: ({
    children,
    onInteractOutside,
    onEscapeKeyDown,
  }: {
    children: React.ReactNode;
    onInteractOutside?: (event: { preventDefault: () => void }) => void;
    onEscapeKeyDown?: (event: { preventDefault: () => void }) => void;
  }) => {
    const fireOutside = () => {
      const event = { defaultPrevented: false, preventDefault: vi.fn() };
      onInteractOutside?.(event);
      (
        globalThis as unknown as {
          __lastOutsideEvent: typeof event;
        }
      ).__lastOutsideEvent = event;
    };
    const fireEscape = () => {
      const event = { defaultPrevented: false, preventDefault: vi.fn() };
      onEscapeKeyDown?.(event);
      (
        globalThis as unknown as {
          __lastEscapeEvent: typeof event;
        }
      ).__lastEscapeEvent = event;
    };
    return (
      <div data-testid="drawer-content">
        <button
          type="button"
          data-testid="drawer-fire-outside"
          onClick={fireOutside}
        >
          outside
        </button>
        <button
          type="button"
          data-testid="drawer-fire-escape"
          onClick={fireEscape}
        >
          escape
        </button>
        {children}
      </div>
    );
  },
  DrawerHeader: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  DrawerTitle: ({ children }: { children: React.ReactNode }) => (
    <h2>{children}</h2>
  ),
}));

class MockResizeObserver {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
}
vi.stubGlobal("ResizeObserver", MockResizeObserver);

import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { ModalProvider, useModal } from "../dashboard/modal-context";
import { MasterDetailLayout } from "./master-detail-layout";

function ModalSwitch({ open }: { open: boolean }) {
  const { openModal, closeModal } = useModal();
  React.useEffect(() => {
    if (open) {
      openModal();
      return () => closeModal();
    }
  }, [open, openModal, closeModal]);
  return null;
}

describe("MasterDetailLayout", () => {
  beforeEach(() => {
    vi.mocked(useIsMobile).mockReturnValue(false);
  });

  describe("desktop", () => {
    it("renders list and detail side by side when selectedId set", () => {
      render(
        <MasterDetailLayout
          list={<div>List</div>}
          detail={<div>Detail</div>}
          selectedId="1"
          onDeselect={vi.fn()}
        />,
      );

      expect(screen.getByText("List")).toBeInTheDocument();
      expect(screen.getByText("Detail")).toBeInTheDocument();
    });

    it("renders detail slot when selectedId is null and behavior is placeholder", () => {
      render(
        <MasterDetailLayout
          list={<div>List</div>}
          detail={<div data-testid="empty">Empty</div>}
          selectedId={null}
          onDeselect={vi.fn()}
        />,
      );

      expect(screen.getByTestId("empty")).toBeInTheDocument();
    });

    it("hides detail pane when selectedId is null and behavior is expand", () => {
      render(
        <MasterDetailLayout
          list={<div>List</div>}
          detail={<div>Detail</div>}
          selectedId={null}
          onDeselect={vi.fn()}
          unselectedBehavior="expand"
        />,
      );

      expect(screen.queryByText("Detail")).not.toBeInTheDocument();
    });

    it("uses custom listWidth when detail visible", () => {
      const { container } = render(
        <MasterDetailLayout
          list={<div>List</div>}
          detail={<div>Detail</div>}
          selectedId="1"
          onDeselect={vi.fn()}
          listWidth={500}
        />,
      );
      const listPane = container.querySelector('[style*="width: 500px"]');
      expect(listPane).toBeInTheDocument();
    });

    it("applies className to root", () => {
      const { container } = render(
        <MasterDetailLayout
          list={<div>List</div>}
          detail={<div>Detail</div>}
          selectedId="1"
          onDeselect={vi.fn()}
          className="my-root"
        />,
      );
      expect(container.firstChild).toHaveClass("my-root");
    });

    // #3330: Höhe aus useFillHeight, Scrollen in der Karte. Den Browser-
    // Nachbau dieses Markups prüft e2e/layout/database-layouts.spec.ts.
    it("takes both cards out of the grow rule and keeps them in one row", () => {
      const { container } = render(
        <MasterDetailLayout
          list={<div>List</div>}
          detail={<div>Detail</div>}
          selectedId="1"
          onDeselect={vi.fn()}
        />,
      );

      const root = container.firstChild as HTMLElement;
      expect(root.style.height).toContain("100dvh");
      expect(root).toHaveClass("flex", "gap-4");
      const cards = root.querySelectorAll(":scope > .moto-content-surface");
      expect(cards).toHaveLength(2);
      expect(cards[0]).toHaveClass("moto-scroll-surface", "shrink-0");
      expect(cards[1]).toHaveClass("moto-scroll-surface", "min-w-0", "flex-1");
    });

    it("takes the lone list card out of the grow rule when nothing is selected", () => {
      const { container } = render(
        <MasterDetailLayout
          list={<div>List</div>}
          detail={<div>Detail</div>}
          selectedId={null}
          onDeselect={vi.fn()}
          unselectedBehavior="expand"
        />,
      );

      const cards = (container.firstChild as HTMLElement).querySelectorAll(
        ":scope > .moto-content-surface",
      );
      expect(cards).toHaveLength(1);
      expect(cards[0]).toHaveClass("moto-scroll-surface", "flex-1");
    });
  });

  describe("mobile", () => {
    beforeEach(() => {
      vi.mocked(useIsMobile).mockReturnValue(true);
    });

    it("renders list and drawer with detail when selectedId set", () => {
      render(
        <MasterDetailLayout
          list={<div>List</div>}
          detail={<div>Detail</div>}
          selectedId="1"
          onDeselect={vi.fn()}
        />,
      );

      expect(screen.getByText("List")).toBeInTheDocument();
      expect(screen.getByTestId("drawer")).toHaveAttribute("data-open", "true");
      expect(screen.getByText("Detail")).toBeInTheDocument();
    });

    // #3330: Auf dem Telefon scrollt die Seite. Keine feste Höhe, und die
    // Karte bleibt in der Wachstumsregel, damit sie mit der Liste wächst.
    it("lets the page scroll instead of fixing the list card's height", () => {
      const { container } = render(
        <MasterDetailLayout
          list={<div>List</div>}
          detail={<div>Detail</div>}
          selectedId={null}
          onDeselect={vi.fn()}
        />,
      );

      const root = container.firstChild as HTMLElement;
      expect(root.style.height).toBe("");
      const card = root.querySelector(":scope > .moto-content-surface");
      expect(card).not.toBeNull();
      expect(card).not.toHaveClass("moto-scroll-surface");
    });

    it("drawer is closed when selectedId is null", () => {
      render(
        <MasterDetailLayout
          list={<div>List</div>}
          detail={<div>Detail</div>}
          selectedId={null}
          onDeselect={vi.fn()}
        />,
      );

      expect(screen.getByTestId("drawer")).toHaveAttribute(
        "data-open",
        "false",
      );
    });

    it("calls onDeselect when drawer closes", () => {
      const onDeselect = vi.fn();
      render(
        <MasterDetailLayout
          list={<div>List</div>}
          detail={<div>Detail</div>}
          selectedId="1"
          onDeselect={onDeselect}
        />,
      );

      fireEvent.click(screen.getByTestId("drawer-close"));
      expect(onDeselect).toHaveBeenCalled();
    });

    it("preventsDefault on outside-interaction while a modal is open", () => {
      const onDeselect = vi.fn();
      render(
        <ModalProvider>
          <ModalSwitch open={true} />
          <MasterDetailLayout
            list={<div>List</div>}
            detail={<div>Detail</div>}
            selectedId="1"
            onDeselect={onDeselect}
          />
        </ModalProvider>,
      );

      fireEvent.click(screen.getByTestId("drawer-fire-outside"));
      const event = (
        globalThis as unknown as {
          __lastOutsideEvent: { preventDefault: ReturnType<typeof vi.fn> };
        }
      ).__lastOutsideEvent;
      expect(event.preventDefault).toHaveBeenCalled();
    });

    it("does not preventDefault on outside-interaction without an open modal", () => {
      const onDeselect = vi.fn();
      render(
        <ModalProvider>
          <ModalSwitch open={false} />
          <MasterDetailLayout
            list={<div>List</div>}
            detail={<div>Detail</div>}
            selectedId="1"
            onDeselect={onDeselect}
          />
        </ModalProvider>,
      );

      fireEvent.click(screen.getByTestId("drawer-fire-outside"));
      const event = (
        globalThis as unknown as {
          __lastOutsideEvent: { preventDefault: ReturnType<typeof vi.fn> };
        }
      ).__lastOutsideEvent;
      expect(event.preventDefault).not.toHaveBeenCalled();
    });

    it("preventsDefault on escape while a modal is open", () => {
      render(
        <ModalProvider>
          <ModalSwitch open={true} />
          <MasterDetailLayout
            list={<div>List</div>}
            detail={<div>Detail</div>}
            selectedId="1"
            onDeselect={vi.fn()}
          />
        </ModalProvider>,
      );

      fireEvent.click(screen.getByTestId("drawer-fire-escape"));
      const event = (
        globalThis as unknown as {
          __lastEscapeEvent: { preventDefault: ReturnType<typeof vi.fn> };
        }
      ).__lastEscapeEvent;
      expect(event.preventDefault).toHaveBeenCalled();
    });

    it("exposes drawer title via sr-only header", () => {
      render(
        <MasterDetailLayout
          list={<div>List</div>}
          detail={<div>Detail</div>}
          selectedId="1"
          onDeselect={vi.fn()}
          mobileDrawerTitle="Kinder Details"
        />,
      );

      expect(screen.getByText("Kinder Details")).toBeInTheDocument();
    });
  });
});
