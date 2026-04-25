import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

vi.mock("~/hooks/useIsMobile", () => ({
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
  DrawerContent: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="drawer-content">{children}</div>
  ),
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

import { useIsMobile } from "~/hooks/useIsMobile";
import { MasterDetailLayout } from "./master-detail-layout";

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

    it("exposes drawer title via sr-only header", () => {
      render(
        <MasterDetailLayout
          list={<div>List</div>}
          detail={<div>Detail</div>}
          selectedId="1"
          onDeselect={vi.fn()}
          mobileDrawerTitle="Schüler Details"
        />,
      );

      expect(screen.getByText("Schüler Details")).toBeInTheDocument();
    });
  });
});
