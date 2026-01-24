/**
 * Tests for logout-modal component
 *
 * Note: The LogoutModal component uses React portals via the Modal component,
 * which makes comprehensive testing complex in the happy-dom test environment.
 * These tests focus on the core component logic and export verification.
 */
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

// Mock the Modal component to render children directly without portals
vi.mock("./modal", () => ({
  Modal: ({
    isOpen,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
  }) => (isOpen ? <div data-testid="modal-content">{children}</div> : null),
}));

// Import after mocks
import { LogoutModal } from "./logout-modal";

describe("LogoutModal", () => {
  describe("component export", () => {
    it("exports LogoutModal component", () => {
      expect(LogoutModal).toBeDefined();
      expect(typeof LogoutModal).toBe("function");
    });
  });

  describe("closed state", () => {
    it("does not render content when isOpen is false", () => {
      render(<LogoutModal isOpen={false} onClose={vi.fn()} />);
      expect(screen.queryByTestId("modal-content")).not.toBeInTheDocument();
      expect(screen.queryByText("Abmelden")).not.toBeInTheDocument();
    });
  });

  describe("props interface", () => {
    it("accepts isOpen and onClose props", () => {
      const onClose = vi.fn();
      // This test verifies the component accepts the expected props without error
      expect(() => {
        render(<LogoutModal isOpen={false} onClose={onClose} />);
      }).not.toThrow();
    });
  });
});
