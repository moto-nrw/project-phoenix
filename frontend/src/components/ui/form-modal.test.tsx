import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  render,
  screen,
  fireEvent,
  act,
} from "@testing-library/react";
import type { ReactNode } from "react";
import { FormModal } from "./form-modal";
import { ModalProvider } from "../dashboard/modal-context";

// Wrapper component with portal target
function TestWrapper({ children }: { children: ReactNode }) {
  return <ModalProvider>{children}</ModalProvider>;
}

describe("FormModal", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    document.body.className = "";
    document.documentElement.className = "";
  });

  it("should not render when isOpen is false", () => {
    render(
      <TestWrapper>
        <FormModal isOpen={false} onClose={vi.fn()} title="Test Modal">
          <p>Modal content</p>
        </FormModal>
      </TestWrapper>,
    );

    expect(screen.queryByText("Test Modal")).not.toBeInTheDocument();
  });

  it("should render when isOpen is true", async () => {
    render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={vi.fn()} title="Test Modal">
          <p>Modal content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(screen.getByText("Test Modal")).toBeInTheDocument();
    expect(screen.getByText("Modal content")).toBeInTheDocument();
  });

  it("should call onClose when close button is clicked", async () => {
    const onClose = vi.fn();
    render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={onClose} title="Test">
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    const closeButton = screen.getAllByRole("button", { name: /schließen/i })[0];
    fireEvent.click(closeButton!);

    await act(async () => {
      vi.advanceTimersByTime(300);
    });

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("should call onClose when backdrop is clicked", async () => {
    const onClose = vi.fn();
    render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={onClose} title="Test">
          <p>Content</p>
        </FormModal>
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
        <FormModal isOpen={true} onClose={onClose} title="Test">
          <p>Content</p>
        </FormModal>
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
        <FormModal
          isOpen={true}
          onClose={vi.fn()}
          title="Test"
          footer={<button>Submit</button>}
        >
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(screen.getByRole("button", { name: "Submit" })).toBeInTheDocument();
  });

  it("should add modal-open class when open", async () => {
    render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={vi.fn()} title="Test">
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(document.documentElement.classList.contains("modal-open")).toBe(true);
  });

  it("should remove modal-open class when closed", async () => {
    const { rerender } = render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={vi.fn()} title="Test">
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(document.documentElement.classList.contains("modal-open")).toBe(true);

    rerender(
      <TestWrapper>
        <FormModal isOpen={false} onClose={vi.fn()} title="Test">
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(document.documentElement.classList.contains("modal-open")).toBe(false);
  });

  describe("size prop", () => {
    it("should apply lg size by default", async () => {
      render(
        <TestWrapper>
          <FormModal isOpen={true} onClose={vi.fn()} title="Test">
            <p>Content</p>
          </FormModal>
        </TestWrapper>,
      );

      await act(async () => {
        vi.advanceTimersByTime(20);
      });

      const dialog = document.querySelector('[role="dialog"]');
      expect(dialog).toHaveClass("max-w-2xl");
    });

    it("should apply sm size when specified", async () => {
      render(
        <TestWrapper>
          <FormModal isOpen={true} onClose={vi.fn()} title="Test" size="sm">
            <p>Content</p>
          </FormModal>
        </TestWrapper>,
      );

      await act(async () => {
        vi.advanceTimersByTime(20);
      });

      const dialog = document.querySelector('[role="dialog"]');
      expect(dialog).toHaveClass("max-w-md");
    });

    it("should apply xl size when specified", async () => {
      render(
        <TestWrapper>
          <FormModal isOpen={true} onClose={vi.fn()} title="Test" size="xl">
            <p>Content</p>
          </FormModal>
        </TestWrapper>,
      );

      await act(async () => {
        vi.advanceTimersByTime(20);
      });

      const dialog = document.querySelector('[role="dialog"]');
      expect(dialog).toHaveClass("max-w-4xl");
    });
  });

  describe("mobilePosition prop", () => {
    it("should use bottom position by default", async () => {
      render(
        <TestWrapper>
          <FormModal isOpen={true} onClose={vi.fn()} title="Test">
            <p>Content</p>
          </FormModal>
        </TestWrapper>,
      );

      await act(async () => {
        vi.advanceTimersByTime(20);
      });

      const dialog = document.querySelector('[role="dialog"]');
      expect(dialog).toHaveClass("rounded-t-2xl");
    });

    it("should use center position when specified", async () => {
      render(
        <TestWrapper>
          <FormModal
            isOpen={true}
            onClose={vi.fn()}
            title="Test"
            mobilePosition="center"
          >
            <p>Content</p>
          </FormModal>
        </TestWrapper>,
      );

      await act(async () => {
        vi.advanceTimersByTime(20);
      });

      const dialog = document.querySelector('[role="dialog"]');
      expect(dialog).toHaveClass("rounded-2xl");
    });
  });

  it("should dispatch mobile-modal-open event when opened", async () => {
    const eventSpy = vi.fn();
    globalThis.addEventListener("mobile-modal-open", eventSpy);

    render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={vi.fn()} title="Test">
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(eventSpy).toHaveBeenCalled();

    globalThis.removeEventListener("mobile-modal-open", eventSpy);
  });

  it("should dispatch mobile-modal-close event when closed", async () => {
    const eventSpy = vi.fn();
    globalThis.addEventListener("mobile-modal-close", eventSpy);

    const { rerender } = render(
      <TestWrapper>
        <FormModal isOpen={true} onClose={vi.fn()} title="Test">
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    rerender(
      <TestWrapper>
        <FormModal isOpen={false} onClose={vi.fn()} title="Test">
          <p>Content</p>
        </FormModal>
      </TestWrapper>,
    );

    await act(async () => {
      vi.advanceTimersByTime(20);
    });

    expect(eventSpy).toHaveBeenCalled();

    globalThis.removeEventListener("mobile-modal-close", eventSpy);
  });
});
