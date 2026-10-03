/**
 * Tests for InvitationForm Component
 * Tests the rendering and submission of invitation form
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ApiError } from "~/lib/api-error";
import { InvitationForm } from "./invitation-form";

// Mock dependencies. The form reports errors through the real
// useApiFormError; only the success toast is stubbed.
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: vi.fn(() => ({
    success: vi.fn(),
    error: vi.fn(),
  })),
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
          id: role.id,
          name: role.name ? getRoleDisplayName(role.name) : `Rolle ${role.id}`,
        }))
        .filter((role) => role.id.length > 0),
  };
});

const mockRoles = [
  { id: "9007199254740993", name: "user" },
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

  it("marks every blank required field the server names and focuses the first", async () => {
    mockCreateInvitation.mockRejectedValue(
      new ApiError("validation failed", 400, {
        code: "general.input",
        errors: [
          { field: "email", reason: "cannot be blank" },
          { field: "role_id", reason: "cannot be blank" },
        ],
      }),
    );
    render(<InvitationForm />);
    await waitFor(() => {
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
    });

    fireEvent.click(screen.getByText("Einladung senden"));

    const email = screen.getByLabelText("E-Mail-Adresse");
    await waitFor(() => expect(email).toHaveFocus());
    expect(mockCreateInvitation).toHaveBeenCalledWith(
      expect.objectContaining({ email: "" }),
    );
    expect(
      screen.getByText(
        "Die Einladung konnte nicht übernommen werden. Bitte prüfen Sie Ihre Angaben.",
      ),
    ).toBeInTheDocument();
    expect(email).toHaveAttribute("aria-invalid", "true");
    const role = screen.getByLabelText("Rolle");
    expect(role).toHaveAttribute("aria-invalid", "true");
    expect(role).toHaveAttribute("aria-describedby", "invitation-role-error");
    expect(screen.getAllByText("Bitte prüfen Sie dieses Feld.")).toHaveLength(
      2,
    );
  });

  it("focuses the role when it is the only failed field", async () => {
    mockCreateInvitation.mockRejectedValue(
      new ApiError("validation failed", 400, {
        code: "general.input",
        errors: [{ field: "role_id", reason: "cannot be blank" }],
      }),
    );
    render(<InvitationForm />);
    const emailInput = await screen.findByLabelText("E-Mail-Adresse");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    fireEvent.click(screen.getByText("Einladung senden"));

    await waitFor(() => expect(screen.getByLabelText("Rolle")).toHaveFocus());
  });

  it("calls createInvitation with form data", async () => {
    render(<InvitationForm />);

    // Wait for roles to load first
    await waitFor(() => {
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
    });

    const emailInput = screen.getByLabelText("E-Mail-Adresse");
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
          roleId: "9007199254740993",
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

    const emailInput = screen.getByLabelText("E-Mail-Adresse");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.click(roleSelect);
    fireEvent.click(screen.getByRole("option", { name: "Betreuer" }));

    const firstNameInput = screen.getByLabelText("Vorname (optional)");
    fireEvent.change(firstNameInput, { target: { value: "John" } });

    const lastNameInput = screen.getByLabelText("Nachname (optional)");
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

    const emailInput = screen.getByLabelText("E-Mail-Adresse");
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

    const emailInput = screen.getByLabelText("E-Mail-Adresse");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.click(roleSelect);
    fireEvent.click(screen.getByRole("option", { name: "Betreuer" }));

    const submitButton = screen.getByText("Einladung senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      const emailInput =
        screen.getByLabelText<HTMLInputElement>("E-Mail-Adresse");
      expect(emailInput.value).toBe("");
    });
  });

  it("calls onCreated callback on success", async () => {
    render(<InvitationForm onCreated={mockOnCreated} />);

    // Wait for roles to load first
    await waitFor(() => {
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
    });

    const emailInput = screen.getByLabelText("E-Mail-Adresse");
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
    mockCreateInvitation.mockRejectedValue(
      new ApiError("account already has access to tenant", 409, {
        code: "identity.account_already_has_tenant_access",
        errors: [{ field: "email", reason: "account already has access" }],
      }),
    );

    render(<InvitationForm />);

    await waitFor(() => {
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
    });

    const emailInput = screen.getByLabelText("E-Mail-Adresse");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.click(roleSelect);
    fireEvent.click(screen.getByRole("option", { name: "Betreuer" }));

    const submitButton = screen.getByText("Einladung senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Diese Person hat schon Zugang zu dieser Schule. Sie finden sie in der Personalliste.",
        ),
      ).toBeInTheDocument();
    });
    expect(screen.getByLabelText("E-Mail-Adresse")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
  });

  it("shows error for an email that already has an account", async () => {
    mockCreateInvitation.mockRejectedValue(
      new ApiError("email already exists", 409, {
        code: "identity.email_already_exists",
        errors: [{ field: "email", reason: "email already exists" }],
      }),
    );

    render(<InvitationForm />);

    // Wait for roles to load first
    await waitFor(() => {
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
    });

    const emailInput = screen.getByLabelText("E-Mail-Adresse");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.click(roleSelect);
    fireEvent.click(screen.getByRole("option", { name: "Betreuer" }));

    const submitButton = screen.getByText("Einladung senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Für diese E-Mail-Adresse gibt es schon ein Konto. Bitte verwenden Sie eine andere Adresse.",
        ),
      ).toBeInTheDocument();
    });
  });

  it("shows the crash text, never the raw message, for a failure without a code", async () => {
    mockCreateInvitation.mockRejectedValue(new Error("Network error"));

    render(<InvitationForm />);

    // Wait for roles to load first
    await waitFor(() => {
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
    });

    const emailInput = screen.getByLabelText("E-Mail-Adresse");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.click(roleSelect);
    fireEvent.click(screen.getByRole("option", { name: "Betreuer" }));

    const submitButton = screen.getByText("Einladung senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Die Einladung konnte nicht bearbeitet werden. Bitte versuchen Sie es später erneut.",
        ),
      ).toBeInTheDocument();
    });
    expect(screen.queryByText(/Network error/)).not.toBeInTheDocument();
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

    const emailInput = screen.getByLabelText("E-Mail-Adresse");
    fireEvent.change(emailInput, { target: { value: "test@example.com" } });

    const roleSelect = screen.getByLabelText("Rolle");
    fireEvent.click(roleSelect);
    fireEvent.click(screen.getByRole("option", { name: "Betreuer" }));

    const submitButton = screen.getByText("Einladung senden");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(screen.getByLabelText("E-Mail-Adresse")).toBeDisabled();
      expect(screen.getByLabelText("Rolle")).toBeDisabled();
    });
  });

  it("shows a copyable request ID and a retry for a server error", async () => {
    mockCreateInvitation.mockRejectedValueOnce(
      new ApiError("boom", 500, {
        code: "general.server",
        instance: "req-invite-1",
      }),
    );
    render(<InvitationForm />);
    await waitFor(() => {
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
    });
    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "test@example.com" },
    });

    fireEvent.click(screen.getByText("Einladung senden"));

    expect(
      await screen.findByRole("button", { name: "Vorgangskennung kopieren" }),
    ).toHaveTextContent("Vorgangskennung: req-invite-1");
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(mockCreateInvitation).toHaveBeenCalledTimes(2));
  });

  it("offers a retry when the roles cannot be loaded", async () => {
    mockGetRoles
      .mockRejectedValueOnce(new ApiError("boom", 503, {}))
      .mockResolvedValueOnce(mockRoles);
    render(<InvitationForm />);

    fireEvent.click(await screen.findByRole("button", { name: "Wiederholen" }));

    await waitFor(() => expect(mockGetRoles).toHaveBeenCalledTimes(2));
    await waitFor(() => {
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
    });
  });

  describe("Scroll to error and field highlighting", () => {
    it("scrolls the form error into view", async () => {
      const scrollIntoViewMock = vi.fn();
      Element.prototype.scrollIntoView = scrollIntoViewMock;
      mockCreateInvitation.mockRejectedValue(
        new ApiError("validation failed", 400, {
          code: "general.input",
          errors: [{ field: "email", reason: "cannot be blank" }],
        }),
      );

      render(<InvitationForm />);
      await waitFor(() => {
        expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
      });

      fireEvent.click(screen.getByText("Einladung senden"));

      await waitFor(() => {
        expect(scrollIntoViewMock).toHaveBeenCalledWith({
          behavior: "smooth",
          block: "nearest",
        });
      });
    });

    it("highlights the role label when the server names the role", async () => {
      Element.prototype.scrollIntoView = vi.fn();
      mockCreateInvitation.mockRejectedValue(
        new ApiError("validation failed", 400, {
          code: "general.input",
          errors: [{ field: "role_id", reason: "cannot be blank" }],
        }),
      );

      render(<InvitationForm />);
      await waitFor(() => {
        expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
      });
      fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
        target: { value: "test@example.com" },
      });

      fireEvent.click(screen.getByText("Einladung senden"));

      await waitFor(() => {
        expect(screen.getByText("Rolle").className).toContain(
          "text-moto-red-strong",
        );
      });
    });
  });
});
