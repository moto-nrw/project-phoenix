import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import InvitePage from "./page";

// Mock next/navigation
const mockSearchParams = new Map<string, string>();

vi.mock("next/navigation", () => ({
  useSearchParams: vi.fn(() => ({
    get: (key: string) => mockSearchParams.get(key),
  })),
}));

// Mock next/image
vi.mock("next/image", () => ({
  default: vi.fn(
    (props: { src: string; alt: string; width: number; height: number }) => (
      // eslint-disable-next-line @next/next/no-img-element
      <img
        src={props.src}
        alt={props.alt}
        width={props.width}
        height={props.height}
      />
    ),
  ),
}));

// Mock the invitation API
const { mockValidateInvitation } = vi.hoisted(() => ({
  mockValidateInvitation: vi.fn(),
}));
vi.mock("~/lib/invitation-api", () => ({
  validateInvitation: mockValidateInvitation,
}));

// Mock the invitation accept form component
vi.mock("~/components/auth/invitation-accept-form", () => ({
  InvitationAcceptForm: vi.fn(
    (props: { token: string; invitation: { email: string } }) => (
      <div data-testid="accept-form">
        <div>Token: {props.token}</div>
        <div>Email: {props.invitation.email}</div>
      </div>
    ),
  ),
}));

// Mock the loading component
vi.mock("~/components/ui/loading", () => ({
  Loading: vi.fn((props: { fullPage?: boolean }) => (
    <div data-testid="loading">
      {props.fullPage ? "Full Page" : "Loading..."}
    </div>
  )),
}));

describe("InvitePage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockValidateInvitation.mockReset();
    mockSearchParams.clear();
  });

  it("should render with Suspense boundary", () => {
    mockSearchParams.set("token", "test-token");
    // eslint-disable-next-line @typescript-eslint/no-empty-function
    mockValidateInvitation.mockImplementation(() => new Promise(() => {})); // Never resolves

    render(<InvitePage />);

    // Should show loading state
    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("should show error when no token provided", async () => {
    // No token in search params
    mockValidateInvitation.mockResolvedValueOnce({
      email: "test@example.com",
    });

    render(<InvitePage />);

    await waitFor(() => {
      expect(
        screen.getByText("Kein Einladungstoken angegeben."),
      ).toBeInTheDocument();
    });

    expect(mockValidateInvitation).not.toHaveBeenCalled();
  });

  it("should validate invitation with token", async () => {
    mockSearchParams.set("token", "test-token-123");
    mockValidateInvitation.mockResolvedValueOnce({
      email: "test@example.com",
      firstName: "Test",
      lastName: "User",
    });

    render(<InvitePage />);

    await waitFor(() => {
      expect(mockValidateInvitation).toHaveBeenCalledWith("test-token-123");
    });
  });

  it("should display invitation accept form on success", async () => {
    mockSearchParams.set("token", "valid-token");
    mockValidateInvitation.mockResolvedValueOnce({
      email: "test@example.com",
      firstName: "Test",
      lastName: "User",
    });

    render(<InvitePage />);

    await waitFor(() => {
      expect(screen.getByTestId("accept-form")).toBeInTheDocument();
    });

    expect(screen.getByText("Token: valid-token")).toBeInTheDocument();
    expect(screen.getByText("Email: test@example.com")).toBeInTheDocument();
  });

  it("should show error for 410 expired invitation", async () => {
    mockSearchParams.set("token", "expired-token");
    mockValidateInvitation.mockRejectedValueOnce({
      status: 410,
      message: "Invitation expired",
    });

    render(<InvitePage />);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Diese Einladung ist abgelaufen oder wurde bereits verwendet.",
        ),
      ).toBeInTheDocument();
    });

    expect(mockValidateInvitation).toHaveBeenCalledWith("expired-token");
  });

  it("should show error for 404 not found invitation", async () => {
    mockSearchParams.set("token", "invalid-token");
    mockValidateInvitation.mockRejectedValueOnce({
      status: 404,
      message: "Not found",
    });

    render(<InvitePage />);

    await waitFor(() => {
      expect(
        screen.getByText("Wir konnten diese Einladung nicht finden."),
      ).toBeInTheDocument();
    });
  });

  it("should show generic error for other failures", async () => {
    mockSearchParams.set("token", "error-token");
    mockValidateInvitation.mockRejectedValueOnce({
      status: 500,
      message: "Server error",
    });

    render(<InvitePage />);

    await waitFor(() => {
      expect(screen.getByText("Server error")).toBeInTheDocument();
    });
  });

  it("should show generic error when no message available", async () => {
    mockSearchParams.set("token", "error-token");
    mockValidateInvitation.mockRejectedValueOnce({
      status: 500,
    });

    render(<InvitePage />);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Beim Laden der Einladung ist ein Fehler aufgetreten.",
        ),
      ).toBeInTheDocument();
    });
  });

  it("should display branding elements", async () => {
    mockSearchParams.set("token", "valid-token");
    mockValidateInvitation.mockResolvedValueOnce({
      email: "test@example.com",
    });

    render(<InvitePage />);

    await waitFor(() => {
      expect(screen.getByAltText("moto Logo")).toBeInTheDocument();
    });

    expect(screen.getByText("Willkommen bei moto")).toBeInTheDocument();
    expect(
      screen.getByText(/Bitte bestätige deine Einladung/),
    ).toBeInTheDocument();
  });

  it("should show link to login page on error", async () => {
    mockSearchParams.set("token", "expired-token");
    mockValidateInvitation.mockRejectedValueOnce({
      status: 410,
    });

    render(<InvitePage />);

    await waitFor(() => {
      const loginLink = screen.getByRole("link", { name: /die Startseite/i });
      expect(loginLink).toBeInTheDocument();
      expect(loginLink).toHaveAttribute("href", "/");
    });
  });

  it("should handle cleanup on unmount during loading", async () => {
    mockSearchParams.set("token", "test-token");

    let resolveValidation: (value: unknown) => void;
    const validationPromise = new Promise((resolve) => {
      resolveValidation = resolve;
    });

    mockValidateInvitation.mockReturnValueOnce(validationPromise);

    const { unmount } = render(<InvitePage />);

    // Unmount before validation resolves
    unmount();

    // Resolve validation after unmount
    resolveValidation!({
      email: "test@example.com",
    });

    // Should not throw or cause issues
    await waitFor(() => {
      expect(mockValidateInvitation).toHaveBeenCalled();
    });
  });

  it("should show loading spinner while validating", async () => {
    mockSearchParams.set("token", "test-token");

    let resolveValidation: (value: unknown) => void;
    const validationPromise = new Promise((resolve) => {
      resolveValidation = resolve;
    });

    mockValidateInvitation.mockReturnValueOnce(validationPromise);

    render(<InvitePage />);

    // Should show loading initially
    expect(screen.getByTestId("loading")).toBeInTheDocument();

    resolveValidation!({
      email: "test@example.com",
    });

    await waitFor(() => {
      expect(screen.queryByTestId("loading")).not.toBeInTheDocument();
    });
  });

  it("should display error icon with error message", async () => {
    mockSearchParams.set("token", "expired-token");
    mockValidateInvitation.mockRejectedValueOnce({
      status: 410,
    });

    render(<InvitePage />);

    await waitFor(() => {
      // Check for SVG error icon
      const svg = screen.getByRole("img", { hidden: true });
      expect(svg).toBeInTheDocument();
    });
  });

  it("should re-fetch invitation when token changes", async () => {
    mockSearchParams.set("token", "token-1");
    mockValidateInvitation.mockResolvedValueOnce({
      email: "first@example.com",
    });

    const { rerender } = render(<InvitePage />);

    await waitFor(() => {
      expect(mockValidateInvitation).toHaveBeenCalledWith("token-1");
    });

    // Change token
    mockSearchParams.set("token", "token-2");
    mockValidateInvitation.mockResolvedValueOnce({
      email: "second@example.com",
    });

    rerender(<InvitePage />);

    await waitFor(() => {
      expect(mockValidateInvitation).toHaveBeenCalledWith("token-2");
    });

    expect(mockValidateInvitation).toHaveBeenCalledTimes(2);
  });
});
