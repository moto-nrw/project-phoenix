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

vi.mock("~/components/ui/input", () => ({
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

vi.mock("~/lib/auth-helpers", () => {
  const getRoleDisplayName = (role: string) =>
    role === "teacher" ? "Lehrkraft" : role === "user" ? "Betreuer" : role;
  return {
    getRoleDisplayName,
    toAssignableRoleOptions: (roles: { id: string; name: string }[]) =>
      roles
        .filter(
          (role) => !["guardian", "teacher"].includes(role.name.toLowerCase()),
        )
        .map((role) => ({
          id: Number(role.id),
          name: role.name ? getRoleDisplayName(role.name) : `Rolle ${role.id}`,
        }))
        .filter((role) => !Number.isNaN(role.id)),
  };
});

const mockRoles = [
  { id: "1", name: "user" },
  { id: "2", name: "admin" },
  { id: "3", name: "teacher" },
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
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
    });

    fireEvent.click(screen.getByLabelText("Rolle"));

    expect(
      screen.getByRole("option", { name: "Betreuer" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: "Lehrkraft" }),
    ).not.toBeInTheDocument();
  });

  it("renders position input with placeholder", async () => {
    render(<InvitationForm />);

    await waitFor(() => {
      const input = screen.getByPlaceholderText(
        "z.B. Pädagogische Fachkraft, OGS-Büro",
      );
      expect(input).toBeInTheDocument();
    });
  });

  it("renders position suggestions when existing positions are provided", async () => {
    const { container } = render(
      <InvitationForm
        existingPositions={["Pädagogische Fachkraft", "OGS-Büro"]}
      />,
    );

    await waitFor(() => {
      const input = screen.getByLabelText("Position (optional)");
      expect(input).toHaveAttribute("list", "invitation-position-suggestions");
      expect(
        container.querySelector(
          'datalist#invitation-position-suggestions option[value="Pädagogische Fachkraft"]',
        ),
      ).toBeTruthy();
      expect(
        container.querySelector(
          'datalist#invitation-position-suggestions option[value="OGS-Büro"]',
        ),
      ).toBeTruthy();
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
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
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

    const emailInput = await screen.findByTestId("invitation-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

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
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
    });

    const emailInput = screen.getByTestId("invitation-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.click(roleSelect);
    fireEvent.click(screen.getByRole("option", { name: "Betreuer" }));

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
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
    });

    const emailInput = screen.getByTestId("invitation-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.click(roleSelect);
    fireEvent.click(screen.getByRole("option", { name: "Betreuer" }));

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
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
    });

    const emailInput = screen.getByTestId("invitation-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.click(roleSelect);
    fireEvent.click(screen.getByRole("option", { name: "Betreuer" }));

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
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
    });

    const emailInput = screen.getByTestId("invitation-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.click(roleSelect);
    fireEvent.click(screen.getByRole("option", { name: "Betreuer" }));

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
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
    });

    const emailInput = screen.getByTestId("invitation-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.click(roleSelect);
    fireEvent.click(screen.getByRole("option", { name: "Betreuer" }));

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

  it("shows error for account already has tenant access (409 with code)", async () => {
    mockCreateInvitation.mockRejectedValue({
      status: 409,
      code: "ACCOUNT_ALREADY_HAS_TENANT_ACCESS",
      message: "account already has access to tenant",
    });

    render(<InvitationForm />);

    await waitFor(() => {
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
    });

    const emailInput = screen.getByTestId("invitation-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.click(roleSelect);
    fireEvent.click(screen.getByRole("option", { name: "Betreuer" }));

    const submitButton = screen.getByText("Einladung senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText(
          /Dieser Account hat bereits Zugang zu dieser Einrichtung/,
        ),
      ).toBeInTheDocument();
    });
  });

  it("shows error for duplicate email (409 without code)", async () => {
    mockCreateInvitation.mockRejectedValue({
      status: 409,
      message: "Conflict",
    });

    render(<InvitationForm />);

    // Wait for roles to load first
    await waitFor(() => {
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
    });

    const emailInput = screen.getByTestId("invitation-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.click(roleSelect);
    fireEvent.click(screen.getByRole("option", { name: "Betreuer" }));

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
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
    });

    const emailInput = screen.getByTestId("invitation-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.click(roleSelect);
    fireEvent.click(screen.getByRole("option", { name: "Betreuer" }));

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
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
    });

    const emailInput = screen.getByTestId("invitation-email");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.click(roleSelect);
    fireEvent.click(screen.getByRole("option", { name: "Betreuer" }));

    const submitButton = screen.getByText("Einladung senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByTestId("invitation-email")).toBeDisabled();
      expect(screen.getByLabelText("Rolle")).toBeDisabled();
    });
  });

  describe("Scroll to error and field highlighting", () => {
    it("scrolls to error when submitting with empty email", async () => {
      const scrollIntoViewMock = vi.fn();
      Element.prototype.scrollIntoView = scrollIntoViewMock;

      render(<InvitationForm />);

      // Wait for roles to load
      await waitFor(() => {
        expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
      });

      const submitButton = screen.getByText("Einladung senden");
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(/Bitte gib eine gültige E-Mail-Adresse ein/),
        ).toBeInTheDocument();
        expect(scrollIntoViewMock).toHaveBeenCalledWith({
          behavior: "smooth",
          block: "start",
        });
      });
    });

    it("highlights the role label when role is not selected", async () => {
      Element.prototype.scrollIntoView = vi.fn();

      render(<InvitationForm />);

      // Wait for roles to load
      await waitFor(() => {
        expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
      });

      // Fill email but leave role empty
      const emailInput = screen.getByTestId("invitation-email");
      fireEvent.change(emailInput, { target: { value: "test@example.com" } });

      const submitButton = screen.getByText("Einladung senden");
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText(/Bitte wähle eine Rolle aus/),
        ).toBeInTheDocument();
      });

      const roleLabel = screen.getByText("Rolle");
      expect(roleLabel.className).toContain("text-moto-red-strong");
    });
  });
});
