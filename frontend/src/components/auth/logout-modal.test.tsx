/**
 * Tests for LogoutModal Component
 * Tests the rendering and logout functionality
 */
import { render, screen, fireEvent, act } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { LogoutModal } from "./logout-modal";

// Mock logout function
const mockLogout = vi.fn();

// Mock shell auth context
vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: () => ({
    user: { name: "Test User", email: "test@example.com", roles: [] },
    profile: { firstName: "Test", lastName: "User" },
    status: "authenticated",
    isSessionExpired: false,
    logout: mockLogout,
    mode: "teacher",
    homeUrl: "/dashboard",

    profileUrl: "/profile",
  }),
}));

// The shared error path; the toast itself is tested with ToastContext.
const mockShowError = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
  useApiErrorDisplay: () => ({ show: mockShowError }),
}));

describe("LogoutModal", () => {
  const mockOnClose = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mockLogout.mockResolvedValue(undefined);
  });

  afterEach(() => {
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

    expect(
      screen.getByRole("dialog", { name: "Abmelden" }),
    ).toBeInTheDocument();
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

    expect(mockLogout).toHaveBeenCalledTimes(1);
  });

  it("keeps modal content stable and shows pending button state after logout is triggered", async () => {
    mockLogout.mockImplementation(
      () => new Promise((resolve) => setTimeout(resolve, 1000)),
    );

    render(<LogoutModal isOpen={true} onClose={mockOnClose} />);

    const logoutButton = screen.getByRole("button", { name: /Abmelden/i });

    await act(async () => {
      fireEvent.click(logoutButton);
      // Advance a bit to trigger the loading state
      await vi.advanceTimersByTimeAsync(100);
    });

    expect(
      screen.getByRole("heading", { name: "Abmelden" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Abmeldung läuft..." }),
    ).toBeDisabled();
    expect(screen.queryByText("Abmelden...")).not.toBeInTheDocument();
  });

  it("disables close during logout", async () => {
    mockLogout.mockImplementation(
      () => new Promise((resolve) => setTimeout(resolve, 1000)),
    );

    render(<LogoutModal isOpen={true} onClose={mockOnClose} />);

    const logoutButton = screen.getByRole("button", { name: /Abmelden/i });

    await act(async () => {
      fireEvent.click(logoutButton);
      await vi.advanceTimersByTimeAsync(100);
    });

    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeDisabled();
    const closeButton = screen.getByRole("button", { name: "Modal schließen" });
    fireEvent.click(closeButton);
    expect(mockOnClose).not.toHaveBeenCalled();
  });

  it("clears deliberateLogout flag on signOut failure", async () => {
    const consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation((_msg: unknown, ..._args: unknown[]) => {
        // suppress console.error in tests
      });
    sessionStorage.setItem("deliberateLogout", "1");
    mockLogout.mockRejectedValue(new Error("Sign out failed"));

    render(<LogoutModal isOpen={true} onClose={mockOnClose} />);

    const logoutButton = screen.getByRole("button", { name: /Abmelden/i });
    fireEvent.click(logoutButton);

    await act(async () => {
      await vi.runAllTimersAsync();
    });

    expect(sessionStorage.getItem("deliberateLogout")).toBeNull();

    consoleErrorSpy.mockRestore();
  });

  it("handles signOut errors gracefully", async () => {
    const consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation((_msg: unknown, ..._args: unknown[]) => {
        // suppress console.error in tests
      });
    mockLogout.mockRejectedValue(new Error("Sign out failed"));

    render(<LogoutModal isOpen={true} onClose={mockOnClose} />);

    const logoutButton = screen.getByRole("button", { name: /Abmelden/i });
    fireEvent.click(logoutButton);

    await act(async () => {
      await vi.runAllTimersAsync();
    });

    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "failed to sign out",
      expect.objectContaining({
        error: "Sign out failed",
      }),
    );

    consoleErrorSpy.mockRestore();
  });

  it("closes on a failed logout and shows the error with a retry (#5194)", async () => {
    vi.spyOn(console, "error").mockImplementation(() => undefined);
    const failure = new Error("logout could not revoke the session");
    mockLogout.mockRejectedValueOnce(failure).mockResolvedValueOnce(undefined);

    render(<LogoutModal isOpen={true} onClose={mockOnClose} />);
    fireEvent.click(screen.getByRole("button", { name: /Abmelden/i }));
    await act(async () => {
      await vi.runAllTimersAsync();
    });

    expect(mockShowError).toHaveBeenCalledWith(failure, {
      object: "die Abmeldung",
      retry: expect.any(Function),
    });
    expect(mockOnClose).toHaveBeenCalledTimes(1);

    const [[, options]] = mockShowError.mock.calls as [
      [unknown, { retry: () => void }],
    ];
    await act(async () => {
      options.retry();
      await vi.runAllTimersAsync();
    });
    expect(mockLogout).toHaveBeenCalledTimes(2);
  });
});
