import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  act,
} from "@testing-library/react";

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

// Mock operator-url to avoid NEXT_PUBLIC_OPERATOR_HOSTNAME requirement
vi.mock("~/lib/operator-url", () => ({
  operatorPath: (path: string) => path,
  isOperatorSubdomain: () => false,
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
    let resolveSignIn: (value: { error: null }) => void = () => undefined;
    mockSignIn.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveSignIn = resolve;
        }),
    );

    render(<OperatorLoginPage />);

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
      expect(mockSignIn).toHaveBeenCalledWith("operator-credentials", {
        redirect: false,
        email: "test@example.com",
        password: "password123",
      });
    });
  });

  it("shows account_inactive error message", async () => {
    mockSignIn.mockResolvedValue({
      error: "CredentialsSignin",
      code: "account_inactive",
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

  it("shows rate_limited error message", async () => {
    mockSignIn.mockResolvedValue({
      error: "CredentialsSignin",
      code: "rate_limited",
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

  it("shows generic error when signIn throws an exception", async () => {
    mockSignIn.mockRejectedValue(new Error("Network error"));

    render(<OperatorLoginPage />);

    const submitButton = screen.getByRole("button", { name: /Anmelden/i });
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent("Network error");
    });
  });

  it("shows fallback error when signIn throws a non-Error", async () => {
    mockSignIn.mockRejectedValue("unknown failure");

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
