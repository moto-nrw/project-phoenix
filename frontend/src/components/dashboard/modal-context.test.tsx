import { describe, it, expect } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { ModalProvider, useModal } from "./modal-context";

// Wrapper component
function ModalProviderWrapper({ children }: { children: ReactNode }) {
  return <ModalProvider>{children}</ModalProvider>;
}

describe("ModalProvider", () => {
  it("should render children", () => {
    render(
      <ModalProvider>
        <div data-testid="child">Child content</div>
      </ModalProvider>,
    );

    expect(screen.getByTestId("child")).toBeInTheDocument();
    expect(screen.getByText("Child content")).toBeInTheDocument();
  });

  it("should provide initial state with isModalOpen as false", () => {
    const { result } = renderHook(() => useModal(), {
      wrapper: ModalProviderWrapper,
    });

    expect(result.current.isModalOpen).toBe(false);
  });
});

describe("useModal", () => {
  describe("with ModalProvider", () => {
    it("should return context values", () => {
      const { result } = renderHook(() => useModal(), {
        wrapper: ModalProviderWrapper,
      });

      expect(result.current).toHaveProperty("isModalOpen");
      expect(result.current).toHaveProperty("openModal");
      expect(result.current).toHaveProperty("closeModal");
    });

    it("should open modal when openModal is called", () => {
      const { result } = renderHook(() => useModal(), {
        wrapper: ModalProviderWrapper,
      });

      expect(result.current.isModalOpen).toBe(false);

      act(() => {
        result.current.openModal();
      });

      expect(result.current.isModalOpen).toBe(true);
    });

    it("should close modal when closeModal is called", () => {
      const { result } = renderHook(() => useModal(), {
        wrapper: ModalProviderWrapper,
      });

      // First open
      act(() => {
        result.current.openModal();
      });
      expect(result.current.isModalOpen).toBe(true);

      // Then close
      act(() => {
        result.current.closeModal();
      });
      expect(result.current.isModalOpen).toBe(false);
    });

    it("should have stable openModal and closeModal references", () => {
      const { result, rerender } = renderHook(() => useModal(), {
        wrapper: ModalProviderWrapper,
      });

      const initialOpenModal = result.current.openModal;
      const initialCloseModal = result.current.closeModal;

      rerender();

      expect(result.current.openModal).toBe(initialOpenModal);
      expect(result.current.closeModal).toBe(initialCloseModal);
    });

    it("should handle multiple open/close cycles", () => {
      const { result } = renderHook(() => useModal(), {
        wrapper: ModalProviderWrapper,
      });

      expect(result.current.isModalOpen).toBe(false);

      act(() => {
        result.current.openModal();
      });
      expect(result.current.isModalOpen).toBe(true);

      act(() => {
        result.current.closeModal();
      });
      expect(result.current.isModalOpen).toBe(false);

      act(() => {
        result.current.openModal();
      });
      expect(result.current.isModalOpen).toBe(true);

      act(() => {
        result.current.closeModal();
      });
      expect(result.current.isModalOpen).toBe(false);
    });
  });

  describe("without ModalProvider (fallback)", () => {
    it("should return fallback context", () => {
      const { result } = renderHook(() => useModal());

      expect(result.current.isModalOpen).toBe(false);
      expect(typeof result.current.openModal).toBe("function");
      expect(typeof result.current.closeModal).toBe("function");
    });

    it("should have no-op openModal function", () => {
      const { result } = renderHook(() => useModal());

      // Should not throw
      expect(() => {
        result.current.openModal();
      }).not.toThrow();

      // Should remain false (no-op)
      expect(result.current.isModalOpen).toBe(false);
    });

    it("should have no-op closeModal function", () => {
      const { result } = renderHook(() => useModal());

      // Should not throw
      expect(() => {
        result.current.closeModal();
      }).not.toThrow();

      // Should remain false (no-op)
      expect(result.current.isModalOpen).toBe(false);
    });

    it("should return stable fallback references", () => {
      const { result, rerender } = renderHook(() => useModal());

      const initialOpenModal = result.current.openModal;
      const initialCloseModal = result.current.closeModal;

      rerender();

      expect(result.current.openModal).toBe(initialOpenModal);
      expect(result.current.closeModal).toBe(initialCloseModal);
    });
  });
});
