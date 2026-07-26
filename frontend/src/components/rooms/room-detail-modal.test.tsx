import { fireEvent, render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { RoomDetailModal } from "./room-detail-modal";

// ----------------------------------------------------------------------------
// Mocks: every dependency below is stubbed so the test only exercises this
// component's branching (mobile/desktop direction, roomId null, nested-modal
// guards), not the heavy room-detail body or session/auth machinery.
// ----------------------------------------------------------------------------

const mockUseIsMobile = vi.fn(() => false);
vi.mock("~/components/ui/hooks/useIsMobile", () => ({
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
    onSelectionActiveChange,
  }: {
    roomId: string;
    headerAction?: React.ReactNode;
    onSelectionActiveChange?: (active: boolean) => void;
  }) => (
    <div data-testid="room-detail-loader">
      loader for {roomId}
      {headerAction}
      <button
        type="button"
        onClick={() => onSelectionActiveChange?.(true)}
        data-testid="activate-selection"
      >
        activate selection
      </button>
    </div>
  ),
}));

vi.mock("./transit-students-section", () => ({
  TransitStudentsSection: ({
    onSelectionActiveChange,
  }: {
    onSelectionActiveChange?: (active: boolean) => void;
  }) => (
    <div data-testid="transit-students-section">
      transit students
      <button
        type="button"
        onClick={() => onSelectionActiveChange?.(true)}
        data-testid="activate-transit-selection"
      >
        activate transit selection
      </button>
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
  id?: string;
  className?: string;
  onInteractOutside?: (event: { preventDefault: () => void }) => void;
  onEscapeKeyDown?: (event: { preventDefault: () => void }) => void;
  children: React.ReactNode;
};
const drawerProps = vi.fn<(props: DrawerProps) => void>();
const drawerContentProps = vi.fn<(props: DrawerContentProps) => void>();

type SlideOverProps = {
  open: boolean;
  onOpenChange: (next: boolean) => void;
  shouldScaleBackground?: boolean;
  children: React.ReactNode;
};
type SlideOverContentProps = DrawerContentProps & {
  widthClass?: string;
};
const slideOverProps = vi.fn<(props: SlideOverProps) => void>();
const slideOverContentProps = vi.fn<(props: SlideOverContentProps) => void>();

vi.mock("~/components/ui/slide-over", () => ({
  SlideOver: (props: SlideOverProps) => {
    slideOverProps(props);
    return props.open ? (
      <div data-testid="slide-over-shell" data-direction="right">
        {props.children}
      </div>
    ) : null;
  },
  SlideOverContent: (props: SlideOverContentProps) => {
    slideOverContentProps(props);
    return <div data-testid="slide-over-content">{props.children}</div>;
  },
  SlideOverHeader: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="slide-over-header">{children}</div>
  ),
  SlideOverTitle: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="slide-over-title">{children}</div>
  ),
  SlideOverClose: ({
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
      data-testid="slide-over-close"
      aria-label={ariaLabel}
      className={className}
    >
      {children}
    </button>
  ),
}));

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
  // Vaul's Close primitive. Modal renders an X button via this on
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
    slideOverProps.mockReset();
    slideOverContentProps.mockReset();
  });

  describe("desktop branch (right-side slide-over)", () => {
    beforeEach(() => {
      mockUseIsMobile.mockReturnValue(false);
    });

    it("renders the SlideOver with the loader body when a roomId is provided", () => {
      const onClose = vi.fn();
      render(<RoomDetailModal roomId="42" onClose={onClose} />);

      const shell = screen.getByTestId("slide-over-shell");
      expect(shell).toBeInTheDocument();
      expect(shell.getAttribute("data-direction")).toBe("right");
      expect(screen.getByTestId("slide-over-title").textContent).toBe(
        "Raumdetails",
      );
      expect(screen.getByTestId("room-detail-loader").textContent).toContain(
        "loader for 42",
      );
    });

    it("does not opt into background scaling on the right-side panel", () => {
      render(<RoomDetailModal roomId="42" onClose={vi.fn()} />);
      const lastCall = slideOverProps.mock.calls.at(-1);
      expect(lastCall?.[0].shouldScaleBackground).toBeUndefined();
    });

    it("renders no shell when roomId is null (closed deep-link state)", () => {
      render(<RoomDetailModal roomId={null} onClose={vi.fn()} />);
      expect(screen.queryByTestId("slide-over-shell")).not.toBeInTheDocument();
      expect(
        screen.queryByTestId("room-detail-loader"),
      ).not.toBeInTheDocument();
      const lastCall = slideOverProps.mock.calls.at(-1);
      expect(lastCall?.[0].open).toBe(false);
    });

    it("forwards onClose via onOpenChange(false)", () => {
      const onClose = vi.fn();
      render(<RoomDetailModal roomId="42" onClose={onClose} />);
      const lastCall = slideOverProps.mock.calls.at(-1);
      lastCall![0].onOpenChange(false);
      expect(onClose).toHaveBeenCalledTimes(1);
    });

    it("renders an explicit X close button with an aria-label on desktop", () => {
      render(<RoomDetailModal roomId="42" onClose={vi.fn()} />);
      const close = screen.getByTestId("slide-over-close");
      expect(close).toBeInTheDocument();
      expect(close.getAttribute("aria-label")).toBe("Raumdetails schließen");
    });

    it("renders transit content on desktop without fetching room details", () => {
      render(<RoomDetailModal roomId="__transit__" onClose={vi.fn()} />);

      expect(
        screen.getByRole("heading", { name: "Unterwegs" }),
      ).toBeInTheDocument();
      expect(
        screen.getByTestId("transit-students-section"),
      ).toBeInTheDocument();
      expect(
        screen.queryByTestId("room-detail-loader"),
      ).not.toBeInTheDocument();
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
      expect(screen.getByTestId("room-detail-loader").textContent).toContain(
        "loader for 7",
      );
      expect(screen.getByTestId("drawer-title").textContent).toBe(
        "Raumdetails",
      );
    });

    it("renders an explicit X close button on mobile too", () => {
      render(<RoomDetailModal roomId="42" onClose={vi.fn()} />);
      expect(screen.getByTestId("drawer-close")).toHaveAttribute(
        "aria-label",
        "Raumdetails schließen",
      );
    });

    it("renders transit content on mobile without fetching room details", () => {
      render(<RoomDetailModal roomId="__transit__" onClose={vi.fn()} />);

      expect(
        screen.getByRole("heading", { name: "Unterwegs" }),
      ).toBeInTheDocument();
      expect(
        screen.getByTestId("transit-students-section"),
      ).toBeInTheDocument();
      expect(
        screen.queryByTestId("room-detail-loader"),
      ).not.toBeInTheDocument();
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

        const lastContentCall = isMobile
          ? drawerContentProps.mock.calls.at(-1)
          : slideOverContentProps.mock.calls.at(-1);
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

      const lastContentCall = slideOverContentProps.mock.calls.at(-1);
      const outsideEvent = { preventDefault: vi.fn() };
      lastContentCall![0].onInteractOutside!(outsideEvent);
      expect(outsideEvent.preventDefault).not.toHaveBeenCalled();

      const escapeEvent = { preventDefault: vi.fn() };
      lastContentCall![0].onEscapeKeyDown!(escapeEvent);
      expect(escapeEvent.preventDefault).not.toHaveBeenCalled();
    });

    it("swallows outside-click and Escape while child selection is active", () => {
      render(<RoomDetailModal roomId="42" onClose={vi.fn()} />);
      fireEvent.click(screen.getByTestId("activate-selection"));

      const lastContentCall = slideOverContentProps.mock.calls.at(-1);
      const outsideEvent = { preventDefault: vi.fn() };
      lastContentCall![0].onInteractOutside!(outsideEvent);
      expect(outsideEvent.preventDefault).toHaveBeenCalledTimes(1);

      const escapeEvent = { preventDefault: vi.fn() };
      lastContentCall![0].onEscapeKeyDown!(escapeEvent);
      expect(escapeEvent.preventDefault).toHaveBeenCalledTimes(1);
    });

    it("swallows outside-click and Escape while transit selection is active", () => {
      render(<RoomDetailModal roomId="__transit__" onClose={vi.fn()} />);
      fireEvent.click(screen.getByTestId("activate-transit-selection"));

      const lastContentCall = slideOverContentProps.mock.calls.at(-1);
      const outsideEvent = { preventDefault: vi.fn() };
      lastContentCall![0].onInteractOutside!(outsideEvent);
      expect(outsideEvent.preventDefault).toHaveBeenCalledTimes(1);

      const escapeEvent = { preventDefault: vi.fn() };
      lastContentCall![0].onEscapeKeyDown!(escapeEvent);
      expect(escapeEvent.preventDefault).toHaveBeenCalledTimes(1);
    });
  });
});
