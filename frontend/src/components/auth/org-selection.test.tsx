/**
 * Tests for OrgSelection component
 *
 * This file tests:
 * - Component rendering
 * - Search functionality
 * - Keyboard navigation
 * - Organization selection and redirect
 * - URL building for subdomains
 * - Outside click handling
 * - Loading states
 */

import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { OrgSelection } from "./org-selection";

// Mock next/image
vi.mock("next/image", () => ({
  default: function MockImage({
    alt,
    src,
  }: {
    alt: string;
    src: string;
    width?: number;
    height?: number;
    priority?: boolean;
  }) {
    // eslint-disable-next-line @next/next/no-img-element
    return <img alt={alt} src={src} />;
  },
}));

// Mock fetch
const mockFetch = vi.fn();
global.fetch = mockFetch;

// Mock window.location
const originalLocation = window.location;

beforeEach(() => {
  vi.clearAllMocks();

  // Reset location mock
  Object.defineProperty(window, "location", {
    value: {
      ...originalLocation,
      href: "",
      protocol: "http:",
      host: "localhost:3000",
    },
    writable: true,
    configurable: true,
  });

  // Default successful fetch response
  mockFetch.mockResolvedValue({
    ok: true,
    json: () =>
      Promise.resolve([
        { id: "org-1", name: "Test Organization", slug: "test-org" },
        { id: "org-2", name: "Another Org", slug: "another-org" },
      ]),
  });
});

afterEach(() => {
  vi.useRealTimers();
  Object.defineProperty(window, "location", {
    value: originalLocation,
    writable: true,
    configurable: true,
  });
});

describe("OrgSelection", () => {
  // =============================================================================
  // Rendering Tests
  // =============================================================================

  describe("rendering", () => {
    it("renders logo", async () => {
      render(<OrgSelection />);

      const logo = screen.getByAltText("MOTO Logo");
      expect(logo).toBeInTheDocument();
    });

    it("renders welcome message", async () => {
      render(<OrgSelection />);

      expect(screen.getByText("Willkommen bei moto!")).toBeInTheDocument();
      expect(
        screen.getByText("Wählen Sie Ihre Einrichtung"),
      ).toBeInTheDocument();
    });

    it("renders search input", async () => {
      render(<OrgSelection />);

      expect(
        screen.getByPlaceholderText("Einrichtung suchen..."),
      ).toBeInTheDocument();
    });
  });

  // =============================================================================
  // Initial Load Tests
  // =============================================================================

  describe("initial load", () => {
    it("fetches organizations on mount", async () => {
      render(<OrgSelection />);

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalledWith(
          "/api/public/organizations?limit=10",
        );
      });
    });

    it("shows loading spinner while fetching", async () => {
      let resolvePromise: ((value: unknown) => void) | undefined;
      mockFetch.mockImplementation(
        () =>
          new Promise((resolve) => {
            resolvePromise = resolve;
          }),
      );

      render(<OrgSelection />);

      // Focus the input to show dropdown area
      const input = screen.getByPlaceholderText("Einrichtung suchen...");
      fireEvent.focus(input);

      // The loading spinner should be visible
      const spinner = document.querySelector(".animate-spin");
      expect(spinner).toBeInTheDocument();

      // Cleanup
      resolvePromise?.({ ok: true, json: () => Promise.resolve([]) });
    });
  });

  // =============================================================================
  // Search Functionality Tests
  // =============================================================================

  describe("search functionality", () => {
    it("debounces search requests", async () => {
      vi.useFakeTimers();
      render(<OrgSelection />);

      // Wait for initial load
      await vi.runAllTimersAsync();

      const input = screen.getByPlaceholderText("Einrichtung suchen...");

      // Clear initial calls
      mockFetch.mockClear();

      // Type quickly
      fireEvent.change(input, { target: { value: "t" } });
      fireEvent.change(input, { target: { value: "te" } });
      fireEvent.change(input, { target: { value: "tes" } });

      // Should not have called yet (debounce)
      expect(mockFetch).not.toHaveBeenCalled();

      // Advance timer past debounce
      await vi.advanceTimersByTimeAsync(300);

      // Now it should have been called with final value
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/public/organizations?search=tes&limit=10",
      );
    });

    it("shows dropdown when input is focused", async () => {
      render(<OrgSelection />);

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalled();
      });

      const input = screen.getByPlaceholderText("Einrichtung suchen...");
      fireEvent.focus(input);

      await waitFor(() => {
        expect(screen.getByText("Test Organization")).toBeInTheDocument();
      });
    });

    it("shows no results message when no organizations found", async () => {
      mockFetch.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve([]),
      });

      render(<OrgSelection />);

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalled();
      });

      const input = screen.getByPlaceholderText("Einrichtung suchen...");
      fireEvent.focus(input);
      fireEvent.change(input, { target: { value: "nonexistent" } });

      await waitFor(() => {
        expect(
          screen.getByText("Keine Einrichtung gefunden"),
        ).toBeInTheDocument();
      });
    });

    it("handles fetch error gracefully", async () => {
      const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {
        /* suppress */
      });
      mockFetch.mockRejectedValue(new Error("Network error"));

      render(<OrgSelection />);

      await waitFor(() => {
        expect(consoleSpy).toHaveBeenCalledWith(
          "Failed to search organizations:",
          expect.any(Error),
        );
      });

      consoleSpy.mockRestore();
    });

    it("handles non-ok response", async () => {
      mockFetch.mockResolvedValue({
        ok: false,
      });

      render(<OrgSelection />);

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalled();
      });

      // Should not crash, just empty results
      const input = screen.getByPlaceholderText("Einrichtung suchen...");
      fireEvent.focus(input);

      // No organizations should be shown
      expect(screen.queryByRole("listitem")).not.toBeInTheDocument();
    });
  });

  // =============================================================================
  // Keyboard Navigation Tests
  // =============================================================================

  describe("keyboard navigation", () => {
    it("navigates down with ArrowDown key", async () => {
      render(<OrgSelection />);

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalled();
      });

      const input = screen.getByPlaceholderText("Einrichtung suchen...");
      fireEvent.focus(input);

      await waitFor(() => {
        expect(screen.getByText("Test Organization")).toBeInTheDocument();
      });

      // Press ArrowDown
      fireEvent.keyDown(input, { key: "ArrowDown" });

      // First item should be highlighted - the class is on the button, not parent
      const firstItem = screen.getByText("Test Organization").closest("button");
      expect(firstItem).toHaveClass("bg-blue-100");
    });

    it("navigates up with ArrowUp key", async () => {
      render(<OrgSelection />);

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalled();
      });

      const input = screen.getByPlaceholderText("Einrichtung suchen...");
      fireEvent.focus(input);

      await waitFor(() => {
        expect(screen.getByText("Test Organization")).toBeInTheDocument();
      });

      // Go down twice
      fireEvent.keyDown(input, { key: "ArrowDown" });
      fireEvent.keyDown(input, { key: "ArrowDown" });

      // Go up once
      fireEvent.keyDown(input, { key: "ArrowUp" });

      // First item should be highlighted - the class is on the button, not parent
      const firstItem = screen.getByText("Test Organization").closest("button");
      expect(firstItem).toHaveClass("bg-blue-100");
    });

    it("selects item with Enter key", async () => {
      render(<OrgSelection />);

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalled();
      });

      const input = screen.getByPlaceholderText("Einrichtung suchen...");
      fireEvent.focus(input);

      await waitFor(() => {
        expect(screen.getByText("Test Organization")).toBeInTheDocument();
      });

      // Go down to first item
      fireEvent.keyDown(input, { key: "ArrowDown" });

      // Press Enter to select
      fireEvent.keyDown(input, { key: "Enter" });

      // Should redirect
      expect(window.location.href).toBe("http://test-org.localhost:3000/login");
    });

    it("closes dropdown with Escape key", async () => {
      render(<OrgSelection />);

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalled();
      });

      const input = screen.getByPlaceholderText("Einrichtung suchen...");
      fireEvent.focus(input);

      await waitFor(() => {
        expect(screen.getByText("Test Organization")).toBeInTheDocument();
      });

      // Press Escape
      fireEvent.keyDown(input, { key: "Escape" });

      // Dropdown should be closed
      expect(screen.queryByText("Test Organization")).not.toBeInTheDocument();
    });

    it("does not navigate when dropdown is closed", async () => {
      render(<OrgSelection />);

      const input = screen.getByPlaceholderText("Einrichtung suchen...");

      // Try keyboard navigation without opening dropdown
      fireEvent.keyDown(input, { key: "ArrowDown" });

      // Should not crash, nothing happens
      expect(input).toBeInTheDocument();
    });

    it("does not navigate when no organizations", async () => {
      mockFetch.mockResolvedValue({
        ok: true,
        json: () => Promise.resolve([]),
      });

      render(<OrgSelection />);

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalled();
      });

      const input = screen.getByPlaceholderText("Einrichtung suchen...");
      fireEvent.focus(input);

      // Try keyboard navigation with empty list
      fireEvent.keyDown(input, { key: "ArrowDown" });
      fireEvent.keyDown(input, { key: "Enter" });

      // Should not crash, nothing happens
      expect(window.location.href).toBe("");
    });

    it("does not go below last item", async () => {
      render(<OrgSelection />);

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalled();
      });

      const input = screen.getByPlaceholderText("Einrichtung suchen...");
      fireEvent.focus(input);

      await waitFor(() => {
        expect(screen.getByText("Test Organization")).toBeInTheDocument();
      });

      // Press ArrowDown many times
      for (let i = 0; i < 10; i++) {
        fireEvent.keyDown(input, { key: "ArrowDown" });
      }

      // Second item should be highlighted (last one) - the class is on the button
      const secondItem = screen.getByText("Another Org").closest("button");
      expect(secondItem).toHaveClass("bg-blue-100");
    });

    it("does not go above first item", async () => {
      render(<OrgSelection />);

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalled();
      });

      const input = screen.getByPlaceholderText("Einrichtung suchen...");
      fireEvent.focus(input);

      await waitFor(() => {
        expect(screen.getByText("Test Organization")).toBeInTheDocument();
      });

      // Go down first
      fireEvent.keyDown(input, { key: "ArrowDown" });

      // Press ArrowUp many times
      for (let i = 0; i < 10; i++) {
        fireEvent.keyDown(input, { key: "ArrowUp" });
      }

      // First item should still be highlighted - the class is on the button
      const firstItem = screen.getByText("Test Organization").closest("button");
      expect(firstItem).toHaveClass("bg-blue-100");
    });
  });

  // =============================================================================
  // Organization Selection Tests
  // =============================================================================

  describe("organization selection", () => {
    it("redirects to subdomain on click", async () => {
      render(<OrgSelection />);

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalled();
      });

      const input = screen.getByPlaceholderText("Einrichtung suchen...");
      fireEvent.focus(input);

      await waitFor(() => {
        expect(screen.getByText("Test Organization")).toBeInTheDocument();
      });

      const orgButton = screen.getByText("Test Organization").closest("button");
      fireEvent.click(orgButton!);

      expect(window.location.href).toBe("http://test-org.localhost:3000/login");
    });

    it("highlights item on mouse enter", async () => {
      render(<OrgSelection />);

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalled();
      });

      const input = screen.getByPlaceholderText("Einrichtung suchen...");
      fireEvent.focus(input);

      await waitFor(() => {
        expect(screen.getByText("Another Org")).toBeInTheDocument();
      });

      const secondOrgButton = screen.getByText("Another Org").closest("button");
      fireEvent.mouseEnter(secondOrgButton!);

      // The class is on the button itself, not the parent
      expect(secondOrgButton).toHaveClass("bg-blue-100");
    });
  });

  // =============================================================================
  // Outside Click Tests
  // =============================================================================

  describe("outside click handling", () => {
    it("closes dropdown when clicking outside", async () => {
      render(
        <div>
          <div data-testid="outside">Outside</div>
          <OrgSelection />
        </div>,
      );

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalled();
      });

      const input = screen.getByPlaceholderText("Einrichtung suchen...");
      fireEvent.focus(input);

      await waitFor(() => {
        expect(screen.getByText("Test Organization")).toBeInTheDocument();
      });

      // Click outside
      const outside = screen.getByTestId("outside");
      fireEvent.mouseDown(outside);

      // Dropdown should be closed
      await waitFor(() => {
        expect(screen.queryByText("Test Organization")).not.toBeInTheDocument();
      });
    });
  });

  // =============================================================================
  // URL Building Tests
  // =============================================================================

  describe("URL building", () => {
    it("builds correct URL for localhost", async () => {
      Object.defineProperty(window, "location", {
        value: {
          protocol: "http:",
          host: "localhost:3000",
          href: "",
        },
        writable: true,
        configurable: true,
      });

      render(<OrgSelection />);

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalled();
      });

      const input = screen.getByPlaceholderText("Einrichtung suchen...");
      fireEvent.focus(input);

      await waitFor(() => {
        expect(screen.getByText("Test Organization")).toBeInTheDocument();
      });

      const orgButton = screen.getByText("Test Organization").closest("button");
      fireEvent.click(orgButton!);

      expect(window.location.href).toBe("http://test-org.localhost:3000/login");
    });

    it("builds correct URL for production domain", async () => {
      Object.defineProperty(window, "location", {
        value: {
          protocol: "https:",
          host: "moto-app.de",
          href: "",
        },
        writable: true,
        configurable: true,
      });

      render(<OrgSelection />);

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalled();
      });

      const input = screen.getByPlaceholderText("Einrichtung suchen...");
      fireEvent.focus(input);

      await waitFor(() => {
        expect(screen.getByText("Test Organization")).toBeInTheDocument();
      });

      const orgButton = screen.getByText("Test Organization").closest("button");
      fireEvent.click(orgButton!);

      expect(window.location.href).toBe("https://test-org.moto-app.de/login");
    });
  });

  // =============================================================================
  // Input Change Tests
  // =============================================================================

  describe("input change handling", () => {
    it("opens dropdown and resets selected index on input change", async () => {
      render(<OrgSelection />);

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalled();
      });

      const input = screen.getByPlaceholderText("Einrichtung suchen...");
      fireEvent.focus(input);

      await waitFor(() => {
        expect(screen.getByText("Test Organization")).toBeInTheDocument();
      });

      // Select first item
      fireEvent.keyDown(input, { key: "ArrowDown" });

      // Type something new - this resets selectedIndex to -1
      fireEvent.change(input, { target: { value: "new search" } });

      // Selection should be reset to -1 (nothing selected)
      // This is verified by trying Enter which shouldn't select anything
      fireEvent.keyDown(input, { key: "Enter" });

      // Should not have redirected (no item selected)
      expect(window.location.href).toBe("");
    });
  });
});
