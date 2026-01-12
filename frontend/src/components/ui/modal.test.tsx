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

  it("should render footer when provided", async () => {
    render(
      <TestWrapper>
        <Modal
          isOpen={true}
          onClose={vi.fn()}
          title="Test"
          footer={<button>Footer Button</button>}
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

    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Bestätigen" })).toBeInTheDocument();
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
