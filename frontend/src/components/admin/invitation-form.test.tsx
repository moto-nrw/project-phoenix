/**
 * Tests for InvitationForm Component
 * Tests the rendering and submission of invitation form
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { InvitationForm } from "./invitation-form";

// Mock dependencies
vi.mock("~/contexts/ToastContext", () => ({
  useToast: vi.fn(() => ({
    success: vi.fn(),
    error: vi.fn(),
  })),
}));

vi.mock("~/components/ui", () => ({
  Input: ({
    id,
    label,
    value,
    onChange,
    disabled,
    required,
  }: {
    id: string;
    label: string;
    value?: string;
    onChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
    disabled?: boolean;
    required?: boolean;
  }) => (
    <div>
      <label htmlFor={id}>{label}</label>
      <input
        id={id}
        value={value ?? ""}
        onChange={onChange}
        disabled={disabled}
        required={required}
        data-testid={id}
      />
    </div>
  ),
}));

const mockGetRoles = vi.fn();
const mockCreateInvitation = vi.fn();

vi.mock("~/lib/auth-service", () => ({
  authService: {
    getRoles: (): unknown => mockGetRoles(),
  },
}));

vi.mock("~/lib/invitation-api", () => ({
  createInvitation: (data: unknown): unknown => mockCreateInvitation(data),
}));

vi.mock("~/lib/auth-helpers", () => ({
  getRoleDisplayName: (role: string) =>
    role === "teacher" ? "Lehrkraft" : role,
}));

const mockRoles = [
  { id: "1", name: "teacher" },
  { id: "2", name: "admin" },
];

describe("InvitationForm", () => {
  const mockOnCreated = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockGetRoles.mockResolvedValue(mockRoles);
    mockCreateInvitation.mockResolvedValue({
      id: 1,
      email: "test@example.com",
      token: "abc123",
    });
  });

  it("shows loading state while fetching roles", async () => {
    render(<InvitationForm />);

    // Component shows form even while loading roles, just disables the role select
    await waitFor(() => {
      const roleSelect = screen.getByLabelText("Rolle");
      expect(roleSelect).toBeDisabled();
    });
  });

  it("renders form after loading roles", async () => {
    render(<InvitationForm />);

    await waitFor(() => {
      expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
    });
  });

  it("renders all form fields", async () => {
    render(<InvitationForm />);

    await waitFor(() => {
      expect(screen.getByLabelText("E-Mail-Adresse")).toBeInTheDocument();
      expect(screen.getByLabelText("Rolle")).toBeInTheDocument();
      expect(screen.getByLabelText("Vorname (optional)")).toBeInTheDocument();
      expect(screen.getByLabelText("Nachname (optional)")).toBeInTheDocument();
      expect(screen.getByLabelText("Position (optional)")).toBeInTheDocument();
    });
  });

  it("renders role options", async () => {
    render(<InvitationForm />);

    await waitFor(() => {
      expect(screen.getByText("Lehrkraft")).toBeInTheDocument();
    });
  });

  it("renders position options", async () => {
    render(<InvitationForm />);

    await waitFor(() => {
      expect(screen.getByText("Pädagogische Fachkraft")).toBeInTheDocument();
      expect(screen.getByText("OGS-Büro")).toBeInTheDocument();
      expect(screen.getByText("Extern")).toBeInTheDocument();
    });
  });

  it("renders submit button", async () => {
    render(<InvitationForm />);

    await waitFor(() => {
      expect(screen.getByText("Einladung senden")).toBeInTheDocument();
    });
  });

  it("validates email field", async () => {
    render(<InvitationForm />);

    // Wait for roles to load
    await waitFor(() => {
      expect(screen.getByText("Lehrkraft")).toBeInTheDocument();
    });

    const submitButton = screen.getByText("Einladung senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText(/Bitte gib eine gültige E-Mail-Adresse ein/),
      ).toBeInTheDocument();
    });
  });

  it("validates role field", async () => {
    render(<InvitationForm />);

    await waitFor(() => {
      const emailInput = screen.getByTestId("invitation-email");
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });
    });

    const submitButton = screen.getByText("Einladung senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText(/Bitte wähle eine Rolle aus/),
      ).toBeInTheDocument();
    });
  });

  it("calls createInvitation with form data", async () => {
    render(<InvitationForm />);

    // Wait for roles to load first
    await waitFor(() => {
      expect(screen.getByText("Lehrkraft")).toBeInTheDocument();
    });

    const emailInput = screen.getByTestId("invitation-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.change(roleSelect, { target: { value: "1" } });

    const submitButton = screen.getByText("Einladung senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockCreateInvitation).toHaveBeenCalledWith(
        expect.objectContaining({
          email: "test@example.com",
          roleId: 1,
        }),
      );
    });
  });

  it("includes optional fields in submission", async () => {
    render(<InvitationForm />);

    // Wait for roles to load first
    await waitFor(() => {
      expect(screen.getByText("Lehrkraft")).toBeInTheDocument();
    });

    const emailInput = screen.getByTestId("invitation-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.change(roleSelect, { target: { value: "1" } });

    const firstNameInput = screen.getByTestId("invitation-first-name");
    fireEvent.change(firstNameInput, { target: { value: "John" } });

    const lastNameInput = screen.getByTestId("invitation-last-name");
    fireEvent.change(lastNameInput, { target: { value: "Doe" } });

    const positionSelect = screen.getByLabelText("Position (optional)");
    fireEvent.change(positionSelect, {
      target: { value: "Pädagogische Fachkraft" },
    });

    const submitButton = screen.getByText("Einladung senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockCreateInvitation).toHaveBeenCalledWith(
        expect.objectContaining({
          firstName: "John",
          lastName: "Doe",
          position: "Pädagogische Fachkraft",
        }),
      );
    });
  });

  it("displays success message with invitation link", async () => {
    render(<InvitationForm />);

    // Wait for roles to load first
    await waitFor(() => {
      expect(screen.getByText("Lehrkraft")).toBeInTheDocument();
    });

    const emailInput = screen.getByTestId("invitation-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.change(roleSelect, { target: { value: "1" } });

    const submitButton = screen.getByText("Einladung senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByText(/abc123/)).toBeInTheDocument();
    });
  });

  it("resets form after successful submission", async () => {
    render(<InvitationForm />);

    // Wait for roles to load first
    await waitFor(() => {
      expect(screen.getByText("Lehrkraft")).toBeInTheDocument();
    });

    const emailInput = screen.getByTestId("invitation-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.change(roleSelect, { target: { value: "1" } });

    const submitButton = screen.getByText("Einladung senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      const emailInput =
        screen.getByTestId<HTMLInputElement>("invitation-email");
      expect(emailInput.value).toBe("");
    });
  });

  it("calls onCreated callback on success", async () => {
    render(<InvitationForm onCreated={mockOnCreated} />);

    // Wait for roles to load first
    await waitFor(() => {
      expect(screen.getByText("Lehrkraft")).toBeInTheDocument();
    });

    const emailInput = screen.getByTestId("invitation-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.change(roleSelect, { target: { value: "1" } });

    const submitButton = screen.getByText("Einladung senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockOnCreated).toHaveBeenCalledWith(
        expect.objectContaining({
          email: "test@example.com",
        }),
      );
    });
  });

  it("shows error for duplicate email (409)", async () => {
    mockCreateInvitation.mockRejectedValue({
      status: 409,
      message: "Conflict",
    });

    render(<InvitationForm />);

    // Wait for roles to load first
    await waitFor(() => {
      expect(screen.getByText("Lehrkraft")).toBeInTheDocument();
    });

    const emailInput = screen.getByTestId("invitation-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.change(roleSelect, { target: { value: "1" } });

    const submitButton = screen.getByText("Einladung senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText(
          /Für diese E-Mail-Adresse existiert bereits ein Account/,
        ),
      ).toBeInTheDocument();
    });
  });

  it("shows generic error for other failures", async () => {
    mockCreateInvitation.mockRejectedValue(new Error("Network error"));

    render(<InvitationForm />);

    // Wait for roles to load first
    await waitFor(() => {
      expect(screen.getByText("Lehrkraft")).toBeInTheDocument();
    });

    const emailInput = screen.getByTestId("invitation-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.change(roleSelect, { target: { value: "1" } });

    const submitButton = screen.getByText("Einladung senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      // Error object's message is shown directly
      expect(screen.getByText(/Network error/)).toBeInTheDocument();
    });
  });

  it("disables form during submission", async () => {
    mockCreateInvitation.mockImplementation(
      () => new Promise((resolve) => setTimeout(resolve, 1000)),
    );

    render(<InvitationForm />);

    // Wait for roles to load first
    await waitFor(() => {
      expect(screen.getByText("Lehrkraft")).toBeInTheDocument();
    });

    const emailInput = screen.getByTestId("invitation-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.change(roleSelect, { target: { value: "1" } });

    const submitButton = screen.getByText("Einladung senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByTestId("invitation-email")).toBeDisabled();
      expect(screen.getByLabelText("Rolle")).toBeDisabled();
    });
  });
});
