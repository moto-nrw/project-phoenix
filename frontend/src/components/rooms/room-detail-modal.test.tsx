import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { RoomDetailModal } from "./room-detail-modal";

// ----------------------------------------------------------------------------
// Mocks — every dependency below is stubbed so the test only exercises this
// component's branching (mobile/desktop direction, roomId null, nested-modal
// guards), not the heavy room-detail body or session/auth machinery.
// ----------------------------------------------------------------------------

const mockUseIsMobile = vi.fn(() => false);
vi.mock("~/hooks/useIsMobile", () => ({
  useIsMobile: () => mockUseIsMobile(),
}));

const mockUseModal = vi.fn(() => ({
  isModalOpen: false,
  openModal: vi.fn(),
  closeModal: vi.fn(),
}));
vi.mock("~/components/dashboard/modal-context", () => ({
  useModal: () => mockUseModal(),
}));

vi.mock("./room-detail-content", () => ({
  // The modal now passes its X close button into RoomDetailLoader via
  // `headerAction`. Render that slot inside the mock so the close-button
  // tests below can find the X by testid.
  RoomDetailLoader: ({
    roomId,
    headerAction,
  }: {
    roomId: string;
    headerAction?: React.ReactNode;
  }) => (
    <div data-testid="room-detail-loader">
      loader for {roomId}
      {headerAction}
    </div>
  ),
}));

// Capture every prop passed to the Drawer primitives so we can drive
// onOpenChange / onInteractOutside / onEscapeKeyDown without rendering
// the real vaul implementation (which needs portals and document setup).
type DrawerProps = {
  open: boolean;
  onOpenChange: (next: boolean) => void;
  direction?: "top" | "bottom" | "left" | "right";
  shouldScaleBackground?: boolean;
  children: React.ReactNode;
};
type DrawerContentProps = {
  className?: string;
  onInteractOutside?: (event: { preventDefault: () => void }) => void;
  onEscapeKeyDown?: (event: { preventDefault: () => void }) => void;
  children: React.ReactNode;
};
const drawerProps = vi.fn<(props: DrawerProps) => void>();
const drawerContentProps = vi.fn<(props: DrawerContentProps) => void>();
vi.mock("~/components/ui/drawer", () => ({
  Drawer: (props: DrawerProps) => {
    drawerProps(props);
    return props.open ? (
      <div data-testid="drawer-shell" data-direction={props.direction}>
        {props.children}
      </div>
    ) : null;
  },
  DrawerContent: (props: DrawerContentProps) => {
    drawerContentProps(props);
    return <div data-testid="drawer-content">{props.children}</div>;
  },
  DrawerHeader: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="drawer-header">{children}</div>
  ),
  DrawerTitle: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="drawer-title">{children}</div>
  ),
  // Vaul's Close primitive — modal renders an X button via this on
  // desktop. Mock it as a plain button so the close-button test below
  // can assert presence and click without portals/dialogs.
  DrawerClose: ({
    children,
    className,
    "aria-label": ariaLabel,
  }: {
    children?: React.ReactNode;
    className?: string;
    "aria-label"?: string;
  }) => (
    <button
      type="button"
      data-testid="drawer-close"
      aria-label={ariaLabel}
      className={className}
    >
      {children}
    </button>
  ),
}));

// ----------------------------------------------------------------------------
// Tests
// ----------------------------------------------------------------------------

describe("RoomDetailModal", () => {
  beforeEach(() => {
    mockUseIsMobile.mockReset().mockReturnValue(false);
    mockUseModal.mockReset().mockReturnValue({
      isModalOpen: false,
      openModal: vi.fn(),
      closeModal: vi.fn(),
    });
    drawerProps.mockReset();
    drawerContentProps.mockReset();
  });

  describe("desktop branch (right-side slide-over)", () => {
    beforeEach(() => {
      mockUseIsMobile.mockReturnValue(false);
    });

    it("renders the Drawer with direction=right and the loader body when a roomId is provided", () => {
      const onClose = vi.fn();
      render(<RoomDetailModal roomId="42" onClose={onClose} />);

      const shell = screen.getByTestId("drawer-shell");
      expect(shell).toBeInTheDocument();
      expect(shell.getAttribute("data-direction")).toBe("right");
      // Title is sr-only but must still be present for accessibility.
      expect(screen.getByTestId("drawer-title").textContent).toBe(
        "Raumdetails",
      );
      expect(screen.getByTestId("room-detail-loader").textContent).toBe(
        "loader for 42",
      );
    });

    it("disables shouldScaleBackground on the right-side panel (only applies to bottom sheet)", () => {
      render(<RoomDetailModal roomId="42" onClose={vi.fn()} />);
      const lastCall = drawerProps.mock.calls.at(-1);
      expect(lastCall?.[0].shouldScaleBackground).toBe(false);
    });

    it("renders no shell when roomId is null (closed deep-link state)", () => {
      render(<RoomDetailModal roomId={null} onClose={vi.fn()} />);
      expect(screen.queryByTestId("drawer-shell")).not.toBeInTheDocument();
      expect(
        screen.queryByTestId("room-detail-loader"),
      ).not.toBeInTheDocument();
      const lastCall = drawerProps.mock.calls.at(-1);
      expect(lastCall?.[0].open).toBe(false);
    });

    it("forwards onClose via onOpenChange(false)", () => {
      const onClose = vi.fn();
      render(<RoomDetailModal roomId="42" onClose={onClose} />);
      const lastCall = drawerProps.mock.calls.at(-1);
      lastCall![0].onOpenChange(false);
      expect(onClose).toHaveBeenCalledTimes(1);
    });

    it("renders an explicit X close button with an aria-label on desktop", () => {
      // Reviewer asked for an X — Esc + outside-click alone aren't
      // discoverable on desktop, especially for keyboard-aware users.
      // The button is a Vaul DrawerClose so vaul handles dismissal
      // automatically (no extra onClick wiring needed).
      render(<RoomDetailModal roomId="42" onClose={vi.fn()} />);
      const close = screen.getByTestId("drawer-close");
      expect(close).toBeInTheDocument();
      expect(close.getAttribute("aria-label")).toBe("Raumdetails schließen");
    });
  });

  describe("mobile branch (bottom drawer)", () => {
    beforeEach(() => {
      mockUseIsMobile.mockReturnValue(true);
    });

    it("renders the Drawer with direction=bottom and the loader when a roomId is provided", () => {
      const onClose = vi.fn();
      render(<RoomDetailModal roomId="7" onClose={onClose} />);

      const shell = screen.getByTestId("drawer-shell");
      expect(shell).toBeInTheDocument();
      expect(shell.getAttribute("data-direction")).toBe("bottom");
      expect(screen.getByTestId("room-detail-loader").textContent).toBe(
        "loader for 7",
      );
      expect(screen.getByTestId("drawer-title").textContent).toBe(
        "Raumdetails",
      );
    });

    it("hides the X close button on mobile (drag handle / swipe carries the affordance)", () => {
      render(<RoomDetailModal roomId="42" onClose={vi.fn()} />);
      expect(screen.queryByTestId("drawer-close")).not.toBeInTheDocument();
    });

    it("keeps shouldScaleBackground on the bottom sheet (iOS-style scale animation)", () => {
      render(<RoomDetailModal roomId="42" onClose={vi.fn()} />);
      const lastCall = drawerProps.mock.calls.at(-1);
      expect(lastCall?.[0].shouldScaleBackground).toBe(true);
    });

    it("renders no shell when roomId is null", () => {
      render(<RoomDetailModal roomId={null} onClose={vi.fn()} />);
      expect(screen.queryByTestId("drawer-shell")).not.toBeInTheDocument();
      expect(
        screen.queryByTestId("room-detail-loader"),
      ).not.toBeInTheDocument();
    });

    it("calls onClose when the Drawer signals dismissal via onOpenChange(false)", () => {
      const onClose = vi.fn();
      render(<RoomDetailModal roomId="42" onClose={onClose} />);

      const lastDrawerCall = drawerProps.mock.calls.at(-1);
      expect(lastDrawerCall).toBeDefined();
      lastDrawerCall![0].onOpenChange(false);
      expect(onClose).toHaveBeenCalledTimes(1);
    });

    it("does NOT call onClose when onOpenChange(true) fires (Drawer opening)", () => {
      const onClose = vi.fn();
      render(<RoomDetailModal roomId="42" onClose={onClose} />);

      const lastDrawerCall = drawerProps.mock.calls.at(-1);
      lastDrawerCall![0].onOpenChange(true);
      expect(onClose).not.toHaveBeenCalled();
    });
  });

  describe("nested-modal guards (apply on both branches)", () => {
    it.each([
      ["desktop", false],
      ["mobile", true],
    ])(
      "swallows outside-click and Escape on %s when a nested modal is open",
      (_label, isMobile) => {
        mockUseIsMobile.mockReturnValue(isMobile);
        mockUseModal.mockReturnValue({
          isModalOpen: true,
          openModal: vi.fn(),
          closeModal: vi.fn(),
        });

        render(<RoomDetailModal roomId="42" onClose={vi.fn()} />);

        const lastContentCall = drawerContentProps.mock.calls.at(-1);
        expect(lastContentCall).toBeDefined();

        const outsideEvent = { preventDefault: vi.fn() };
        lastContentCall![0].onInteractOutside!(outsideEvent);
        expect(outsideEvent.preventDefault).toHaveBeenCalledTimes(1);

        const escapeEvent = { preventDefault: vi.fn() };
        lastContentCall![0].onEscapeKeyDown!(escapeEvent);
        expect(escapeEvent.preventDefault).toHaveBeenCalledTimes(1);
      },
    );

    it("lets outside-click and Escape pass through when no nested modal is open", () => {
      mockUseModal.mockReturnValue({
        isModalOpen: false,
        openModal: vi.fn(),
        closeModal: vi.fn(),
      });

      render(<RoomDetailModal roomId="42" onClose={vi.fn()} />);

      const lastContentCall = drawerContentProps.mock.calls.at(-1);
      const outsideEvent = { preventDefault: vi.fn() };
      lastContentCall![0].onInteractOutside!(outsideEvent);
      expect(outsideEvent.preventDefault).not.toHaveBeenCalled();

      const escapeEvent = { preventDefault: vi.fn() };
      lastContentCall![0].onEscapeKeyDown!(escapeEvent);
      expect(escapeEvent.preventDefault).not.toHaveBeenCalled();
    });
  });
});
