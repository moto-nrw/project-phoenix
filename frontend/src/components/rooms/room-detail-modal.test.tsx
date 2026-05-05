import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { RoomDetailModal } from "./room-detail-modal";

// ----------------------------------------------------------------------------
// Mocks — every dependency below is stubbed so the test only exercises this
// component's branching (mobile/desktop, roomId null, nested-modal guards),
// not the heavy room-detail body or session/auth machinery.
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
  RoomDetailLoader: ({ roomId }: { roomId: string }) => (
    <div data-testid="room-detail-loader">loader for {roomId}</div>
  ),
}));

// Capture every prop passed to <Modal> so we can assert title, widthClass,
// and verify onClose by calling the captured handler directly.
type ModalProps = {
  isOpen: boolean;
  onClose: () => void;
  title: string;
  widthClass?: string;
  children: React.ReactNode;
};
const modalProps = vi.fn<(props: ModalProps) => void>();
vi.mock("~/components/ui/modal", () => ({
  Modal: (props: ModalProps) => {
    modalProps(props);
    return props.isOpen ? (
      <div data-testid="modal-shell" data-width={props.widthClass}>
        <button type="button" onClick={props.onClose} data-testid="modal-close">
          Close
        </button>
        <div data-testid="modal-title">{props.title}</div>
        {props.children}
      </div>
    ) : null;
  },
}));

// Capture every prop passed to the Drawer primitives so we can drive
// onOpenChange / onInteractOutside / onEscapeKeyDown without rendering
// the real vaul implementation (which needs portals and document setup).
type DrawerProps = {
  open: boolean;
  onOpenChange: (next: boolean) => void;
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
      <div data-testid="drawer-shell">{props.children}</div>
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
    modalProps.mockReset();
    drawerProps.mockReset();
    drawerContentProps.mockReset();
  });

  describe("desktop branch (Modal)", () => {
    beforeEach(() => {
      mockUseIsMobile.mockReturnValue(false);
    });

    it("renders the Modal with the wide width class and the loader body when a roomId is provided", () => {
      const onClose = vi.fn();
      render(<RoomDetailModal roomId="42" onClose={onClose} />);

      const shell = screen.getByTestId("modal-shell");
      expect(shell).toBeInTheDocument();
      // The detail view needs more horizontal space than the default form modal.
      expect(shell.getAttribute("data-width")).toContain("max-w-4xl");
      // A non-empty title pushes the close button into its own header bar
      // so it does not collide with the room name in the body.
      expect(screen.getByTestId("modal-title").textContent).toBe("Raumdetails");
      expect(screen.getByTestId("room-detail-loader").textContent).toBe(
        "loader for 42",
      );
    });

    it("renders the Modal but no body when roomId is null (closed deep-link state)", () => {
      const onClose = vi.fn();
      render(<RoomDetailModal roomId={null} onClose={onClose} />);

      // open=false → mocked Modal returns null entirely.
      expect(screen.queryByTestId("modal-shell")).not.toBeInTheDocument();
      // And the loader is not invoked when roomId is null even if open were
      // true — verified by absence here.
      expect(
        screen.queryByTestId("room-detail-loader"),
      ).not.toBeInTheDocument();
      const lastCall = modalProps.mock.calls.at(-1);
      expect(lastCall?.[0].isOpen).toBe(false);
    });

    it("forwards onClose so the Modal's close button triggers the callback", () => {
      const onClose = vi.fn();
      render(<RoomDetailModal roomId="42" onClose={onClose} />);
      screen.getByTestId("modal-close").click();
      expect(onClose).toHaveBeenCalledTimes(1);
    });
  });

  describe("mobile branch (Drawer)", () => {
    beforeEach(() => {
      mockUseIsMobile.mockReturnValue(true);
    });

    it("renders the Drawer shell with the loader when a roomId is provided", () => {
      const onClose = vi.fn();
      render(<RoomDetailModal roomId="7" onClose={onClose} />);

      expect(screen.getByTestId("drawer-shell")).toBeInTheDocument();
      expect(screen.getByTestId("room-detail-loader").textContent).toBe(
        "loader for 7",
      );
      // Title is sr-only but must still be present for accessibility.
      expect(screen.getByTestId("drawer-title").textContent).toBe(
        "Raumdetails",
      );
    });

    it("renders the Drawer body but no loader when roomId is null", () => {
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

    it("swallows outside-click and Escape when a nested modal is open (prevents drawer dismissal under it)", () => {
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
    });

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
