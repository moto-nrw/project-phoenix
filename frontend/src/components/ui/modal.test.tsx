import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import type { ReactNode } from "react";
import { Modal, ConfirmationModal } from "./modal";
import { ModalProvider } from "../dashboard/modal-context";

// Wrapper component with portal target
function TestWrapper({ children }: { children: ReactNode }) {
  return <ModalProvider>{children}</ModalProvider>;
}

describe("Modal", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("should not render when isOpen is false", () => {
    render(
      <TestWrapper>
        <Modal isOpen={false} onClose={vi.fn()} title="Test Modal">
          <p>Modal content</p>
        </Modal>
      </TestWrapper>,
    );

    expect(screen.queryByText("Test Modal")).not.toBeInTheDocument();
    expect(screen.queryByText("Modal content")).not.toBeInTheDocument();
  });

  it("should render when isOpen is true", async () => {
    render(
      <TestWrapper>
        <Modal isOpen={true} onClose={vi.fn()} title="Test Modal">
          <p>Modal content</p>
        </Modal>
      </TestWrapper>,
    );

    // Advance timers for animation
    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(screen.getByText("Test Modal")).toBeInTheDocument();
    expect(screen.getByText("Modal content")).toBeInTheDocument();
  });

  it("should render with title in header", async () => {
    render(
      <TestWrapper>
        <Modal isOpen={true} onClose={vi.fn()} title="My Title">
          <p>Content</p>
        </Modal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(screen.getByRole("heading", { level: 3 })).toHaveTextContent(
      "My Title",
    );
  });

  it("gives a 255-character title wrapping space without shrinking the close target", async () => {
    const longTitle = "T".repeat(255);

    render(
      <TestWrapper>
        <Modal isOpen={true} onClose={vi.fn()} title={longTitle}>
          <p>Content</p>
        </Modal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(longTitle).toHaveLength(255);
    expect(screen.getByRole("heading", { name: longTitle })).toHaveClass(
      "min-w-0",
      "flex-1",
      "wrap-anywhere",
    );
    expect(screen.getByRole("button", { name: "Modal schließen" })).toHaveClass(
      "size-11",
      "shrink-0",
    );
  });

  it("should render close button in header when title is provided", async () => {
    render(
      <TestWrapper>
        <Modal isOpen={true} onClose={vi.fn()} title="Test">
          <p>Content</p>
        </Modal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    // Get all close buttons (header button + backdrop)
    const closeButtons = screen.getAllByRole("button", { name: /schließen/i });
    // Should have the header close button
    expect(closeButtons.length).toBeGreaterThanOrEqual(1);
  });

  it("should render close button absolutely positioned when no title", async () => {
    render(
      <TestWrapper>
        <Modal isOpen={true} onClose={vi.fn()} title="">
          <p>Content</p>
        </Modal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    // When no title, the close button should be positioned absolutely
    const closeButtons = screen.getAllByRole("button", { name: /schließen/i });
    const absoluteButton = closeButtons.find((btn) =>
      btn.classList.contains("absolute"),
    );
    expect(absoluteButton).toBeInTheDocument();
  });

  it("should call onClose when close button is clicked", async () => {
    const onClose = vi.fn();
    render(
      <TestWrapper>
        <Modal isOpen={true} onClose={onClose} title="Test">
          <p>Content</p>
        </Modal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    // Get the close button in the header (not the backdrop)
    const closeButtons = screen.getAllByRole("button", { name: /schließen/i });
    const headerCloseButton = closeButtons.find(
      (btn) => !btn.classList.contains("absolute") && btn.closest(".flex"),
    );
    if (headerCloseButton) {
      fireEvent.click(headerCloseButton);
    } else {
      // Fallback: click the first close button
      fireEvent.click(closeButtons[0]!);
    }

    // Wait for exit animation
    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("removes the backdrop from the tab order so FocusScope cannot land on it", async () => {
    render(
      <TestWrapper>
        <Modal isOpen={true} onClose={vi.fn()} title="Test">
          <input data-testid="first-field" />
        </Modal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    const backdrop = screen.getByRole("button", {
      name: /hintergrund.*schließen/i,
    });
    expect(backdrop).toHaveAttribute("tabIndex", "-1");
  });

  it("should call onClose when backdrop is clicked", async () => {
    const onClose = vi.fn();
    render(
      <TestWrapper>
        <Modal isOpen={true} onClose={onClose} title="Test">
          <p>Content</p>
        </Modal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    const backdrop = screen.getByRole("button", {
      name: /hintergrund.*schließen/i,
    });
    fireEvent.click(backdrop);

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("can keep typed work open when only the backdrop is clicked", async () => {
    const onClose = vi.fn();
    render(
      <TestWrapper>
        <Modal
          isOpen={true}
          onClose={onClose}
          title="Test"
          isBackdropDismissDisabled
        >
          <textarea defaultValue="Noch nicht gespeichert" />
        </Modal>
      </TestWrapper>,
    );

    const backdrop = screen.getByRole("button", {
      name: /hintergrund.*schließen/i,
    });
    expect(backdrop).toBeDisabled();
    fireEvent.click(backdrop);

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(onClose).not.toHaveBeenCalled();
  });

  it("keeps the obscured page blurred behind the shared backdrop", async () => {
    render(
      <TestWrapper>
        <Modal isOpen={true} onClose={vi.fn()} title="Test">
          <p>Content</p>
        </Modal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(
      screen.getByRole("button", { name: /hintergrund.*schließen/i }),
    ).toHaveClass("backdrop-blur-sm");
  });

  it("should call onClose when Escape key is pressed", async () => {
    const onClose = vi.fn();
    render(
      <TestWrapper>
        <Modal isOpen={true} onClose={onClose} title="Test">
          <p>Content</p>
        </Modal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    fireEvent.keyDown(document, { key: "Escape" });

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("cancels a pending dismissal when isDismissDisabled becomes true during the exit animation", async () => {
    const onClose = vi.fn();
    const { rerender } = render(
      <TestWrapper>
        <Modal isOpen={true} onClose={onClose} title="Test">
          <p>Content</p>
        </Modal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    // Dismissal starts (backdrop) and queues onClose behind the exit animation
    fireEvent.click(
      screen.getByRole("button", { name: /hintergrund.*schließen/i }),
    );

    // Confirmation begins during the 250ms window and locks dismissal
    rerender(
      <TestWrapper>
        <Modal
          isOpen={true}
          onClose={onClose}
          title="Test"
          isDismissDisabled={true}
        >
          <p>Content</p>
        </Modal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByText("Content")).toBeInTheDocument();
  });

  it("should render footer when provided", async () => {
    render(
      <TestWrapper>
        <Modal
          isOpen={true}
          onClose={vi.fn()}
          title="Test"
          footer={<button type="button">Footer Button</button>}
        >
          <p>Content</p>
        </Modal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(
      screen.getByRole("button", { name: "Footer Button" }),
    ).toBeInTheDocument();
  });

  it("bounds the whole dialog to the viewport and scrolls only the content", async () => {
    render(
      <TestWrapper>
        <Modal
          isOpen={true}
          onClose={vi.fn()}
          title="Scrollable modal"
          footer={<button type="button">Footer Button</button>}
        >
          <div>Long content</div>
        </Modal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    const dialog = screen.getByRole("dialog", { name: "Scrollable modal" });
    const content = document.querySelector('[data-modal-content="true"]');

    expect(dialog).toHaveClass("flex", "max-h-[calc(100dvh-2rem)]");
    expect(content).toHaveClass(
      "min-h-0",
      "flex-1",
      "overflow-y-auto",
      "overscroll-contain",
    );
    expect(screen.getByRole("heading").parentElement).toHaveClass("shrink-0");
    expect(
      screen.getByRole("button", { name: "Footer Button" }).parentElement,
    ).toHaveClass("shrink-0");
  });

  it("should have data-modal-content attribute for scroll lock", async () => {
    render(
      <TestWrapper>
        <Modal isOpen={true} onClose={vi.fn()} title="Test">
          <p>Content</p>
        </Modal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    const content = document.querySelector('[data-modal-content="true"]');
    expect(content).toBeInTheDocument();
  });

  it("should handle transition from open to closed", async () => {
    const onClose = vi.fn();
    const { rerender } = render(
      <TestWrapper>
        <Modal isOpen={true} onClose={onClose} title="Test">
          <p>Content</p>
        </Modal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(screen.getByText("Test")).toBeInTheDocument();

    rerender(
      <TestWrapper>
        <Modal isOpen={false} onClose={onClose} title="Test">
          <p>Content</p>
        </Modal>
      </TestWrapper>,
    );

    expect(screen.queryByText("Test")).not.toBeInTheDocument();
  });

  it("should stay stable when onClose identity changes while open", async () => {
    const onClose1 = vi.fn();
    const onClose2 = vi.fn();

    const { rerender } = render(
      <TestWrapper>
        <Modal isOpen={true} onClose={onClose1} title="">
          <p>Step 1</p>
        </Modal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(screen.getByText("Step 1")).toBeInTheDocument();

    // Change onClose identity while modal stays open — should NOT flicker
    rerender(
      <TestWrapper>
        <Modal isOpen={true} onClose={onClose2} title="">
          <p>Step 2</p>
        </Modal>
      </TestWrapper>,
    );

    // Modal should stay open with new content, no animation reset
    expect(screen.getByText("Step 2")).toBeInTheDocument();
  });
});

describe("ConfirmationModal", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("should render with default button texts", async () => {
    render(
      <TestWrapper>
        <ConfirmationModal
          isOpen={true}
          onClose={vi.fn()}
          onConfirm={vi.fn()}
          title="Confirm Action"
        >
          <p>Are you sure?</p>
        </ConfirmationModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(
      screen.getByRole("button", { name: "Abbrechen" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Bestätigen" }),
    ).toBeInTheDocument();
  });

  it("should render with custom button texts", async () => {
    render(
      <TestWrapper>
        <ConfirmationModal
          isOpen={true}
          onClose={vi.fn()}
          onConfirm={vi.fn()}
          title="Delete"
          confirmText="Löschen"
          cancelText="Nein"
        >
          <p>Delete this?</p>
        </ConfirmationModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(screen.getByRole("button", { name: "Nein" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Löschen" })).toBeInTheDocument();
  });

  it("should call onClose when cancel button is clicked", async () => {
    const onClose = vi.fn();
    render(
      <TestWrapper>
        <ConfirmationModal
          isOpen={true}
          onClose={onClose}
          onConfirm={vi.fn()}
          title="Confirm"
        >
          <p>Sure?</p>
        </ConfirmationModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("should call onConfirm when confirm button is clicked", async () => {
    const onConfirm = vi.fn();
    render(
      <TestWrapper>
        <ConfirmationModal
          isOpen={true}
          onClose={vi.fn()}
          onConfirm={onConfirm}
          title="Confirm"
        >
          <p>Sure?</p>
        </ConfirmationModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("should show loading state when isConfirmLoading is true", async () => {
    render(
      <TestWrapper>
        <ConfirmationModal
          isOpen={true}
          onClose={vi.fn()}
          onConfirm={vi.fn()}
          title="Confirm"
          isConfirmLoading={true}
        >
          <p>Sure?</p>
        </ConfirmationModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(screen.getByText("Wird geladen...")).toBeInTheDocument();
  });

  it("should disable confirm button when loading", async () => {
    render(
      <TestWrapper>
        <ConfirmationModal
          isOpen={true}
          onClose={vi.fn()}
          onConfirm={vi.fn()}
          title="Confirm"
          isConfirmLoading={true}
        >
          <p>Sure?</p>
        </ConfirmationModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    const confirmButton = screen.getByRole("button", {
      name: /wird geladen/i,
    });
    expect(confirmButton).toBeDisabled();
  });

  it("should block every dismissal path when isDismissDisabled is set", async () => {
    const onClose = vi.fn();
    render(
      <TestWrapper>
        <ConfirmationModal
          isOpen={true}
          onClose={onClose}
          onConfirm={vi.fn()}
          title="Confirm"
          isConfirmLoading={true}
          isDismissDisabled={true}
        >
          <p>Sure?</p>
        </ConfirmationModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    const cancel = screen.getByRole("button", { name: "Abbrechen" });
    const close = screen.getByRole("button", { name: "Modal schließen" });
    const backdrop = screen.getByRole("button", {
      name: "Hintergrund - Klicken zum Schließen",
    });
    expect(cancel).toBeDisabled();
    expect(close).toBeDisabled();
    expect(backdrop).toBeDisabled();

    fireEvent.click(cancel);
    fireEvent.click(close);
    fireEvent.click(backdrop);
    fireEvent.keyDown(document, { key: "Escape" });
    await act(async () => {
      vi.advanceTimersByTime(300);
    });
    expect(onClose).not.toHaveBeenCalled();
  });

  it("should cancel a queued dismissal when the lock is set during the exit animation", async () => {
    const onClose = vi.fn();
    const { rerender } = render(
      <TestWrapper>
        <ConfirmationModal
          isOpen={true}
          onClose={onClose}
          onConfirm={vi.fn()}
          title="Confirm"
        >
          <p>Sure?</p>
        </ConfirmationModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    // Dismissal starts while unlocked — the exit animation is now running.
    fireEvent.click(
      screen.getByRole("button", {
        name: "Hintergrund - Klicken zum Schließen",
      }),
    );

    // The operation takes the lock inside the 250ms animation window.
    rerender(
      <TestWrapper>
        <ConfirmationModal
          isOpen={true}
          onClose={onClose}
          onConfirm={vi.fn()}
          title="Confirm"
          isConfirmLoading={true}
          isDismissDisabled={true}
        >
          <p>Sure?</p>
        </ConfirmationModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(onClose).not.toHaveBeenCalled();
    // ...and the dialog is visible again, not left mid-exit.
    expect(screen.getByText("Sure?")).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toHaveClass("animate-modalEnter");
  });

  it("should keep dismissal open while loading without isDismissDisabled", async () => {
    const onClose = vi.fn();
    render(
      <TestWrapper>
        <ConfirmationModal
          isOpen={true}
          onClose={onClose}
          onConfirm={vi.fn()}
          title="Confirm"
          isConfirmLoading={true}
        >
          <p>Sure?</p>
        </ConfirmationModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    const cancel = screen.getByRole("button", { name: "Abbrechen" });
    expect(cancel).not.toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Modal schließen" }),
    ).not.toBeDisabled();
    expect(
      screen.getByRole("button", {
        name: "Hintergrund - Klicken zum Schließen",
      }),
    ).not.toBeDisabled();

    fireEvent.click(cancel);
    await act(async () => {
      vi.advanceTimersByTime(300);
    });
    expect(onClose).toHaveBeenCalled();
  });

  it("should apply custom confirmButtonClass", async () => {
    render(
      <TestWrapper>
        <ConfirmationModal
          isOpen={true}
          onClose={vi.fn()}
          onConfirm={vi.fn()}
          title="Delete"
          confirmButtonClass="bg-red-600 hover:bg-red-700"
        >
          <p>Delete?</p>
        </ConfirmationModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    const confirmButton = screen.getByRole("button", { name: "Bestätigen" });
    expect(confirmButton).toHaveClass("bg-red-600");
  });

  it("renders an opted-in modal as a dismissible bottom sheet on phones", async () => {
    const onClose = vi.fn();
    const matchMedia = vi.spyOn(window, "matchMedia").mockImplementation(
      (query) =>
        ({
          matches: query === "(max-width: 639px)",
          media: query,
          onchange: null,
          addEventListener: vi.fn(),
          removeEventListener: vi.fn(),
          addListener: vi.fn(),
          removeListener: vi.fn(),
          dispatchEvent: vi.fn(),
        }) as MediaQueryList,
    );

    render(
      <TestWrapper>
        <ConfirmationModal
          mobileSheet
          isOpen={true}
          onClose={onClose}
          onConfirm={vi.fn()}
          title="Abwesenheit melden"
        >
          <p>Inhalt</p>
        </ConfirmationModal>
      </TestWrapper>,
    );

    await act(async () => undefined);

    expect(document.querySelector('[data-mobile-sheet="true"]')).toBeTruthy();
    expect(document.querySelector('[data-drawer-handle="true"]')).toBeTruthy();
    const closeButton = screen.getByRole("button", {
      name: "Modal schließen",
    });
    expect(closeButton).toHaveClass("size-11");
    expect(screen.getByRole("button", { name: "Abbrechen" })).toHaveClass(
      "hidden",
    );

    fireEvent.click(closeButton);
    expect(onClose).toHaveBeenCalledOnce();

    matchMedia.mockRestore();
  });

  it("should not render when isOpen is false", () => {
    render(
      <TestWrapper>
        <ConfirmationModal
          isOpen={false}
          onClose={vi.fn()}
          onConfirm={vi.fn()}
          title="Confirm"
        >
          <p>Sure?</p>
        </ConfirmationModal>
      </TestWrapper>,
    );

    expect(screen.queryByText("Confirm")).not.toBeInTheDocument();
  });
});
