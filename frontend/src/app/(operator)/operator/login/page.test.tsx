import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";

// Mock dependencies
const { mockPush, mockSignIn, mockUseSession } = vi.hoisted(() => ({
  mockPush: vi.fn(),
  mockSignIn: vi.fn(),
  mockUseSession: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
}));

vi.mock("next/image", () => ({
  // eslint-disable-next-line @next/next/no-img-element, jsx-a11y/alt-text
  default: (props: Record<string, unknown>) => <img {...props} />,
}));

vi.mock("next-auth/react", () => ({
  signIn: mockSignIn,
  useSession: mockUseSession,
}));

vi.mock("~/lib/confetti", () => ({
  launchConfetti: vi.fn(),
  clearConfetti: vi.fn(),
}));

vi.mock("~/components/ui", () => ({
  Input: ({
    value,
    onChange,
    ...props
  }: React.InputHTMLAttributes<HTMLInputElement> & {
    value?: string;
    onChange?: (e: React.ChangeEvent<HTMLInputElement>) => void;
  }) => <input value={value} onChange={onChange} {...props} />,
  Alert: ({ message }: { message: string }) => (
    <div role="alert">{message}</div>
  ),
  HelpButton: () => <button>Help</button>,
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

vi.mock("~/components/shared/login-help-content", () => ({
  LoginHelpContent: () => <div>Help Content</div>,
}));

import OperatorLoginPage from "./page";

describe("OperatorLoginPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseSession.mockReturnValue({
      status: "unauthenticated",
      data: null,
    });
  });

  it("renders login form", () => {
    render(<OperatorLoginPage />);

    expect(screen.getByText("Willkommen bei moto")).toBeInTheDocument();
    expect(screen.getByText("Operator Dashboard")).toBeInTheDocument();
    expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
    expect(screen.getByLabelText("Passwort")).toBeInTheDocument();
  });

  it("renders logo", () => {
    render(<OperatorLoginPage />);

    const logo = screen.getByAltText("MOTO Logo");
    expect(logo).toBeInTheDocument();
  });

  it("submits form with email and password", async () => {
    mockSignIn.mockResolvedValue({ error: null });

    render(<OperatorLoginPage />);

    const emailInput = screen.getByLabelText("E-Mail-Adresse");
    const passwordInput = screen.getByLabelText("Passwort");
    const submitButton = screen.getByRole("button", { name: /Anmelden/i });

    fireEvent.change(emailInput, { target: { value: "test@example.com" } });
    fireEvent.change(passwordInput, { target: { value: "password123" } });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockSignIn).toHaveBeenCalledWith("operator-credentials", {
        redirect: false,
        email: "test@example.com",
        password: "password123",
      });
    });
  });

  it("shows error message on login failure", async () => {
    mockSignIn.mockResolvedValue({ error: "CredentialsSignin" });

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
    mockSignIn.mockImplementation(
      () => new Promise((resolve) => setTimeout(resolve, 100)),
    );

    render(<OperatorLoginPage />);

    const submitButton = screen.getByRole("button", { name: /Anmelden/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText("Anmeldung läuft...")).toBeInTheDocument();
    });
  });

  it("redirects when already authenticated as operator", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: { user: { scope: "platform" } },
    });

    render(<OperatorLoginPage />);

    expect(mockPush).toHaveBeenCalledWith("/operator/suggestions");
  });

  it("shows loading screen while checking auth", () => {
    mockUseSession.mockReturnValue({
      status: "loading",
      data: null,
    });

    render(<OperatorLoginPage />);

    expect(screen.getByText("Loading...")).toBeInTheDocument();
  });
});
