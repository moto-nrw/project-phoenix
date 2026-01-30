import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, render, waitFor, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { useEffect } from "react";
import { ToastProvider, useToast } from "./ToastContext";

// Helper to create a wrapper with ToastProvider
function createWrapper() {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <ToastProvider>{children}</ToastProvider>;
  };
}

describe("ToastContext", () => {
  beforeEach(() => {
    // Mock matchMedia for toast visibility logic
    Object.defineProperty(globalThis, "matchMedia", {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: query === "(min-width: 768px)", // Desktop by default
        media: query,
        onchange: null,
        addListener: vi.fn(), // Deprecated
        removeListener: vi.fn(), // Deprecated
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe("useToast hook", () => {
    it("throws error when used outside ToastProvider", () => {
      // Suppress console.error for this test
      const consoleError = vi
        .spyOn(console, "error")
        .mockImplementation(() => undefined);

      expect(() => {
        renderHook(() => useToast());
      }).toThrow("useToast must be used within ToastProvider");

      consoleError.mockRestore();
    });

    it("returns toast API when used inside ToastProvider", () => {
      const { result } = renderHook(() => useToast(), {
        wrapper: createWrapper(),
      });

      expect(result.current).toHaveProperty("success");
      expect(result.current).toHaveProperty("error");
      expect(result.current).toHaveProperty("info");
      expect(result.current).toHaveProperty("warning");
      expect(result.current).toHaveProperty("remove");
    });
  });

  describe("Toast functionality", () => {
    it("ignores empty messages", async () => {
      function TestComponent() {
        const toast = useToast();

        useEffect(() => {
          toast.success("");
        }, [toast]);

        return <div data-testid="test-component">Test</div>;
      }

      const { container } = render(
        <ToastProvider>
          <TestComponent />
        </ToastProvider>,
      );

      // Wait for component to render
      await waitFor(() => {
        expect(screen.getByTestId("test-component")).toBeInTheDocument();
      });

      // No toasts should be rendered when message is empty
      const toastContainer = container.querySelector('[class*="fixed"]');
      const toasts = toastContainer?.querySelectorAll("button, output");
      expect(toasts?.length ?? 0).toBe(0);
    });

    it("renders mobile and desktop versions of toasts", async () => {
      function TestComponent() {
        const toast = useToast();

        useEffect(() => {
          toast.success("Test message");
        }, [toast]);

        return null;
      }

      render(
        <ToastProvider>
          <TestComponent />
        </ToastProvider>,
      );

      await waitFor(
        () => {
          const messages = screen.getAllByText("Test message");
          // Should have both mobile and desktop versions
          expect(messages.length).toBe(2);
        },
        { timeout: 3000 },
      );
    });
  });

  describe("Accessibility", () => {
    it("has proper ARIA attributes", async () => {
      function TestComponent() {
        const toast = useToast();

        useEffect(() => {
          toast.success("Test message");
        }, [toast]);

        return null;
      }

      render(
        <ToastProvider>
          <TestComponent />
        </ToastProvider>,
      );

      await waitFor(
        () => {
          const outputs = screen.getAllByRole("status");
          expect(outputs.length).toBeGreaterThan(0);

          outputs.forEach((output) => {
            expect(output).toHaveAttribute("aria-live", "polite");
            expect(output).toHaveAttribute("aria-atomic", "true");
          });
        },
        { timeout: 3000 },
      );
    });

    it("has close button with aria-label on desktop version", async () => {
      function TestComponent() {
        const toast = useToast();

        useEffect(() => {
          toast.success("Test message");
        }, [toast]);

        return null;
      }

      render(
        <ToastProvider>
          <TestComponent />
        </ToastProvider>,
      );

      await waitFor(
        () => {
          const closeButtons = screen.getAllByLabelText("Schließen");
          expect(closeButtons.length).toBeGreaterThan(0);
        },
        { timeout: 3000 },
      );
    });
  });
});
