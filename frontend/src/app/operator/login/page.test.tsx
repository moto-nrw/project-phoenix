import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  act,
} from "@testing-library/react";

// Mock dependencies
const { mockPush, mockRedirect, mockSignIn, mockSignOut, mockUseSession } =
  vi.hoisted(() => ({
    mockPush: vi.fn(),
    mockRedirect: vi.fn(),
    mockSignIn: vi.fn(),
    mockSignOut: vi.fn(),
    mockUseSession: vi.fn(),
  }));

const originalFetch = global.fetch;

function mockOperatorFetchResponse(
  status: number,
  body: unknown,
): typeof global.fetch {
  return vi.fn().mockResolvedValue(
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

function mockAuthenticatedResponse(): typeof global.fetch {
  return mockOperatorFetchResponse(200, {
    status: "success",
    data: {
      status: "authenticated",
      access_token: "access-token",
      refresh_token: "refresh-token",
    },
    message: "Login successful",
  });
}

vi.mock("next/navigation", () => ({
  redirect: mockRedirect,
  useRouter: () => ({ push: mockPush }),
}));

vi.mock("next/image", () => ({
  // eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text
  default: (props: Record<string, unknown>) => <img {...props} />,
}));

vi.mock("next-auth/react", () => ({
  signIn: mockSignIn,
  signOut: mockSignOut,
  useSession: mockUseSession,
}));

// Mock operator-url to avoid NEXT_PUBLIC_OPERATOR_HOSTNAME requirement
vi.mock("~/lib/operator-url", () => ({
  operatorPath: (path: string) => path,
  isOperatorSubdomain: () => false,
}));

vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message }: { message: string }) => (
    <div role="alert">{message}</div>
  ),
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div>Loading...</div>,
}));

vi.mock("~/components/shared/password-toggle-button", () => ({
  PasswordToggleButton: ({ onToggle }: { onToggle: () => void }) => (
    <button onClick={onToggle} type="button">
      Toggle
    </button>
  ),
}));

import OperatorLoginPage from "./page";

describe("OperatorLoginPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseSession.mockReturnValue({
      status: "unauthenticated",
      data: null,
    });
    global.fetch = mockAuthenticatedResponse();
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("renders login form", () => {
    render(<OperatorLoginPage />);

    expect(screen.getByText("Operator Console")).toBeInTheDocument();
    expect(screen.getByText("Interner Plattformzugang")).toBeInTheDocument();
    expect(screen.getByText("Operator Dashboard")).toBeInTheDocument();
    expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
    expect(screen.getByLabelText("Passwort")).toBeInTheDocument();
  });

  it("does not render the MOTO logo", () => {
    render(<OperatorLoginPage />);

    expect(screen.queryByAltText("MOTO Logo")).not.toBeInTheDocument();
  });

  it("submits form with email and password (2-step MFA flow)", async () => {
    mockSignIn.mockResolvedValue({ error: null });

    render(<OperatorLoginPage />);

    const emailInput = screen.getByLabelText("E-Mail-Adresse");
    const passwordInput = screen.getByLabelText("Passwort");
    const submitButton = screen.getByRole("button", { name: /Anmelden/i });

    fireEvent.change(emailInput, { target: { value: "test@example.com" } });
    fireEvent.change(passwordInput, { target: { value: "password123" } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/operator/auth/login",
        expect.objectContaining({
          method: "POST",
          credentials: "include",
          body: JSON.stringify({
            email: "test@example.com",
            password: "password123",
          }),
        }),
      );
    });
    await waitFor(() => {
      expect(mockSignIn).toHaveBeenCalledWith("operator-credentials", {
        redirect: false,
        internalRefresh: "true",
        token: "access-token",
        refreshToken: "refresh-token",
      });
    });
  });

  it("shows error message on login failure (401)", async () => {
    global.fetch = mockOperatorFetchResponse(401, {
      status: "error",
      error: "Invalid credentials",
    });

    render(<OperatorLoginPage />);

    const emailInput = screen.getByLabelText("E-Mail-Adresse");
    const passwordInput = screen.getByLabelText("Passwort");
    const submitButton = screen.getByRole("button", { name: /Anmelden/i });

    fireEvent.change(emailInput, { target: { value: "test@example.com" } });
    fireEvent.change(passwordInput, { target: { value: "wrong" } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Ungültige Anmeldedaten",
      );
    });
  });

  it("shows loading state during authentication", async () => {
    let resolveSignIn: (value: { error: null }) => void = () => undefined;
    mockSignIn.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveSignIn = resolve;
        }),
    );

    render(<OperatorLoginPage />);

    const emailInput = screen.getByLabelText("E-Mail-Adresse");
    fireEvent.change(emailInput, { target: { value: "op@example.com" } });
    const passwordInput = screen.getByLabelText("Passwort");
    fireEvent.change(passwordInput, { target: { value: "password123" } });

    const submitButton = screen.getByRole("button", { name: /Anmelden/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText("Anmeldung läuft...")).toBeInTheDocument();
    });

    await act(async () => {
      resolveSignIn({ error: null });
    });

    await waitFor(() => {
      expect(mockPush).toHaveBeenCalledWith("/operator/suggestions");
    });
  });

  it("redirects when already authenticated as operator", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: { user: { scope: "platform", token: "valid-token" } },
    });

    render(<OperatorLoginPage />);

    expect(mockRedirect).toHaveBeenCalledWith("/operator/suggestions");
  });

  it("clears an errored operator session before redirecting", async () => {
    mockSignOut.mockResolvedValue(undefined);
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: {
        error: "RefreshTokenExpired",
        user: { scope: "platform", token: "stale-token" },
      },
    });

    render(<OperatorLoginPage />);

    expect(mockRedirect).not.toHaveBeenCalled();
    await waitFor(() => {
      expect(mockSignOut).toHaveBeenCalledWith({ redirect: false });
    });
  });

  it("shows loading screen while checking auth", () => {
    mockUseSession.mockReturnValue({
      status: "loading",
      data: null,
    });

    render(<OperatorLoginPage />);

    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });

  it("submits form when submit event is triggered", async () => {
    mockSignIn.mockResolvedValue({ error: null });

    render(<OperatorLoginPage />);

    const emailInput = screen.getByLabelText("E-Mail-Adresse");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const passwordInput = screen.getByLabelText("Passwort");
    fireEvent.change(passwordInput, {
      target: { value: "password123" },
    });

    const form = screen
      .getByRole("button", { name: /Anmelden/i })
      .closest("form")!;
    await act(async () => {
      fireEvent.submit(form);
    });

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/operator/auth/login",
        expect.objectContaining({ method: "POST" }),
      );
    });
    await waitFor(() => {
      expect(mockSignIn).toHaveBeenCalledWith("operator-credentials", {
        redirect: false,
        internalRefresh: "true",
        token: "access-token",
        refreshToken: "refresh-token",
      });
    });
  });

  it("shows account_inactive error message (HTTP 403)", async () => {
    global.fetch = mockOperatorFetchResponse(403, {
      status: "error",
      error: "Account inactive",
    });

    render(<OperatorLoginPage />);

    const emailInput = screen.getByLabelText("E-Mail-Adresse");
    const passwordInput = screen.getByLabelText("Passwort");
    const submitButton = screen.getByRole("button", { name: /Anmelden/i });

    fireEvent.change(emailInput, { target: { value: "test@example.com" } });
    fireEvent.change(passwordInput, { target: { value: "password" } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Ihr Konto ist deaktiviert. Bitte kontaktieren Sie den Administrator.",
      );
    });
  });

  it("shows rate_limited error message (HTTP 429)", async () => {
    global.fetch = mockOperatorFetchResponse(429, {
      status: "error",
      error: "Too many requests",
    });

    render(<OperatorLoginPage />);

    const submitButton = screen.getByRole("button", { name: /Anmelden/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Zu viele Anmeldeversuche. Bitte versuchen Sie es später erneut.",
      );
    });
  });

  it("shows error message when fetch throws an exception", async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error("Network error"));

    render(<OperatorLoginPage />);

    const submitButton = screen.getByRole("button", { name: /Anmelden/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("Network error");
    });
  });

  it("shows fallback error when fetch throws a non-Error", async () => {
    global.fetch = vi.fn().mockRejectedValue("unknown failure");

    render(<OperatorLoginPage />);

    const submitButton = screen.getByRole("button", { name: /Anmelden/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Anmeldefehler. Bitte versuchen Sie es erneut.",
      );
    });
  });

  it("does not redirect when authenticated but not operator scope", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: { user: { scope: "tenant" } },
    });

    render(<OperatorLoginPage />);

    expect(mockPush).not.toHaveBeenCalled();
    expect(screen.getByText("Operator Console")).toBeInTheDocument();
  });

  it("toggles password visibility", () => {
    render(<OperatorLoginPage />);

    const passwordInput = screen.getByLabelText("Passwort");
    expect(passwordInput).toHaveAttribute("type", "password");

    const toggleButton = screen.getByRole("button", { name: "Toggle" });
    fireEvent.click(toggleButton);

    expect(passwordInput).toHaveAttribute("type", "text");
  });
});
