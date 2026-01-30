/**
 * Tests for LogoutModal Component
 * Tests the rendering and logout functionality
 */
import { render, screen, fireEvent, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { LogoutModal } from "./logout-modal";

// Mock next-auth/react
const mockSignOut = vi.fn();
vi.mock("next-auth/react", () => ({
  // eslint-disable-next-line @typescript-eslint/no-unsafe-return
  signOut: () => mockSignOut(),
}));

// Mock Modal component
vi.mock("./modal", () => ({
  Modal: ({
    isOpen,
    onClose,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <button onClick={onClose} data-testid="modal-close">
          Close
        </button>
        {children}
      </div>
    ) : null,
}));

// Mock Element.animate for confetti
const mockAnimate = vi.fn(() => ({
  onfinish: null,
  cancel: vi.fn(),
})) as unknown as typeof Element.prototype.animate;

describe("LogoutModal", () => {
  const mockOnClose = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
    mockSignOut.mockResolvedValue(undefined);
    Element.prototype.animate = mockAnimate;
  });

  afterEach(() => {
    // Clear all pending timers to prevent "document is not defined" errors
    // from confetti setTimeout callbacks firing after test teardown
    vi.clearAllTimers();
    vi.useRealTimers();
  });

  it("renders nothing when closed", () => {
    const { container } = render(
      <LogoutModal isOpen={false} onClose={mockOnClose} />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("renders modal when open", () => {
    render(<LogoutModal isOpen={true} onClose={mockOnClose} />);

    expect(screen.getByTestId("modal")).toBeInTheDocument();
  });

  it("displays logout title", () => {
    render(<LogoutModal isOpen={true} onClose={mockOnClose} />);

    expect(
      screen.getByRole("heading", { name: "Abmelden" }),
    ).toBeInTheDocument();
  });

  it("displays confirmation message", () => {
    render(<LogoutModal isOpen={true} onClose={mockOnClose} />);

    expect(
      screen.getByText(/Möchten Sie sich wirklich von Ihrem Konto abmelden/),
    ).toBeInTheDocument();
  });

  it("renders logout icon", () => {
    render(<LogoutModal isOpen={true} onClose={mockOnClose} />);

    const svg = document.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("renders logout button", () => {
    render(<LogoutModal isOpen={true} onClose={mockOnClose} />);

    const logoutButton = screen.getByRole("button", { name: /Abmelden/i });
    expect(logoutButton).toBeInTheDocument();
  });

  it("calls signOut when logout button is clicked", async () => {
    render(<LogoutModal isOpen={true} onClose={mockOnClose} />);

    const logoutButton = screen.getByRole("button", { name: /Abmelden/i });
    fireEvent.click(logoutButton);

    // Flush all timers and microtasks
    await act(async () => {
      await vi.runAllTimersAsync();
    });

    expect(mockSignOut).toHaveBeenCalledTimes(1);
  });

  it("shows loading state after logout is triggered", async () => {
    mockSignOut.mockImplementation(
      () => new Promise((resolve) => setTimeout(resolve, 1000)),
    );

    render(<LogoutModal isOpen={true} onClose={mockOnClose} />);

    const logoutButton = screen.getByRole("button", { name: /Abmelden/i });

    await act(async () => {
      fireEvent.click(logoutButton);
      // Advance a bit to trigger the loading state
      await vi.advanceTimersByTimeAsync(100);
    });

    expect(screen.getByText("Abmelden...")).toBeInTheDocument();
    expect(
      screen.getByText(/Sie werden zur Anmeldeseite weitergeleitet/),
    ).toBeInTheDocument();
  });

  it("disables close during logout", async () => {
    mockSignOut.mockImplementation(
      () => new Promise((resolve) => setTimeout(resolve, 1000)),
    );

    render(<LogoutModal isOpen={true} onClose={mockOnClose} />);

    const logoutButton = screen.getByRole("button", { name: /Abmelden/i });

    await act(async () => {
      fireEvent.click(logoutButton);
      await vi.advanceTimersByTimeAsync(100);
    });

    const closeButton = screen.getByTestId("modal-close");
    fireEvent.click(closeButton);
    expect(mockOnClose).not.toHaveBeenCalled();
  });

  it("launches confetti animation on logout", async () => {
    render(<LogoutModal isOpen={true} onClose={mockOnClose} />);

    const logoutButton = screen.getByRole("button", { name: /Abmelden/i });
    fireEvent.click(logoutButton);

    // Run all confetti timers
    await act(async () => {
      await vi.runAllTimersAsync();
    });

    expect(mockAnimate).toHaveBeenCalled();
  });

  it("creates confetti container in body", async () => {
    render(<LogoutModal isOpen={true} onClose={mockOnClose} />);

    const logoutButton = screen.getByRole("button", { name: /Abmelden/i });
    fireEvent.click(logoutButton);

    // Run confetti creation timers
    await act(async () => {
      await vi.advanceTimersByTimeAsync(500);
    });

    const confettiContainer = document.querySelector(
      "div[style*='position: fixed']",
    );
    expect(confettiContainer).toBeTruthy();
  });

  it("handles signOut errors gracefully", async () => {
    const consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation((_msg: unknown, ..._args: unknown[]) => {
        // suppress console.error in tests
      });
    mockSignOut.mockRejectedValue(new Error("Sign out failed"));

    render(<LogoutModal isOpen={true} onClose={mockOnClose} />);

    const logoutButton = screen.getByRole("button", { name: /Abmelden/i });
    fireEvent.click(logoutButton);

    // Flush all timers and handle the rejection
    await act(async () => {
      await vi.runAllTimersAsync();
    });

    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "Failed to sign out:",
      expect.any(Error),
    );

    consoleErrorSpy.mockRestore();
  });
});
