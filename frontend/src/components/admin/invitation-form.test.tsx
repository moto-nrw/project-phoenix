/**
 * Tests for InvitationForm Component
 * Tests the rendering and submission of invitation form
 */
import {
  act,
  render,
  screen,
  waitFor,
  fireEvent,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ApiError } from "~/lib/api-error";
import { releaseFakeTimers } from "~/test/clock";
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
const mockFindFormerEmployment = vi.fn();

vi.mock("~/lib/auth-service", () => ({
  authService: {
    getRoles: (): unknown => mockGetRoles(),
  },
}));

vi.mock("~/lib/invitation-api", () => ({
  createInvitation: (data: unknown): unknown => mockCreateInvitation(data),
  findFormerEmployment: (email: unknown, signal?: AbortSignal): unknown =>
    mockFindFormerEmployment(email, signal),
}));

vi.mock("~/lib/auth-helpers", () => {
  const getRoleDisplayName = (role: string) =>
    role === "teacher" ? "Lehrkraft" : role === "user" ? "Betreuer" : role;
  return {
    getRoleDisplayName,
    isExternalCaregiverRole: (name?: string) =>
      name?.trim().toLowerCase() === "extern",
    toAssignableRoleOptions: (roles: { id: string; name: string }[]) =>
      roles
        .filter(
          (role) => !["guardian", "teacher"].includes(role.name.toLowerCase()),
        )
        .map((role) => ({
          id: role.id,
          name: role.name ? getRoleDisplayName(role.name) : `Rolle ${role.id}`,
          systemName: role.name.toLowerCase(),
        }))
        .filter((role) => role.id.length > 0),
  };
});

// The address lookup starts FORMER_EMPLOYMENT_DEBOUNCE_MS (invitation-form.tsx)
// after the last change. Each describe runs on fake timers that keep advancing
// with real time, so waitFor and findBy behave as before; tests skip the
// debounce explicitly where they wait for the lookup.
const LOOKUP_DEBOUNCE_MS = 400;

function skipLookupDebounce() {
  act(() => {
    vi.advanceTimersByTime(LOOKUP_DEBOUNCE_MS);
  });
}

const mockRoles = [
  { id: "9007199254740993", name: "user" },
  { id: "2", name: "admin" },
  { id: "3", name: "teacher" },
];

describe("InvitationForm", () => {
  const mockOnCreated = vi.fn();

  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.clearAllMocks();
    mockGetRoles.mockResolvedValue(mockRoles);
    mockCreateInvitation.mockResolvedValue({
      id: 1,
      email: "test@example.com",
      token: "abc123",
    });
    mockFindFormerEmployment.mockResolvedValue(null);
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

    fireEvent.click(await screen.findByText("Einladung senden"));

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
    skipLookupDebounce();

    fireEvent.click(await screen.findByText("Einladung senden"));

    await waitFor(() => expect(screen.getByLabelText("Rolle")).toHaveFocus());
  });

  // #4573: an invitation with the role „Extern“ creates no external
  // profile. The form says so where the role is picked and offers the right
  // way.
  describe("role Extern", () => {
    const hint =
      "Mit dieser Einladung zählt die Person als normales Personal, zum Beispiel in der Zeiterfassung.";

    async function pickRole(name: string) {
      await waitFor(() => {
        expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
      });
      fireEvent.click(screen.getByLabelText("Rolle"));
      fireEvent.click(screen.getByRole("option", { name }));
    }

    beforeEach(() => {
      mockGetRoles.mockResolvedValue([
        ...mockRoles,
        { id: "4", name: "extern" },
      ]);
    });

    it("points to „Externe Person anlegen“ when Extern is chosen", async () => {
      const onAddExternal = vi.fn();
      render(<InvitationForm onAddExternal={onAddExternal} />);

      await pickRole("extern");

      expect(screen.getByText(new RegExp(hint))).toHaveTextContent(
        "Externe Personen legen Sie mit „Externe Person anlegen“ an.",
      );
      fireEvent.click(
        screen.getByRole("button", { name: "Externe Person anlegen" }),
      );
      expect(onAddExternal).toHaveBeenCalledTimes(1);
    });

    it("only warns when the caller cannot add externals", async () => {
      render(<InvitationForm />);

      await pickRole("extern");

      expect(screen.getByText(hint)).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: "Externe Person anlegen" }),
      ).not.toBeInTheDocument();
    });

    it("shows no hint for other roles", async () => {
      render(<InvitationForm onAddExternal={vi.fn()} />);

      await pickRole("Betreuer");

      expect(screen.queryByText(new RegExp(hint))).not.toBeInTheDocument();
    });
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

    skipLookupDebounce();
    const submitButton = await screen.findByText("Einladung senden");
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

    skipLookupDebounce();
    const submitButton = await screen.findByText("Einladung senden");
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

    skipLookupDebounce();
    const submitButton = await screen.findByText("Einladung senden");
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

    skipLookupDebounce();
    const submitButton = await screen.findByText("Einladung senden");
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

    skipLookupDebounce();
    const submitButton = await screen.findByText("Einladung senden");
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

    skipLookupDebounce();
    const submitButton = await screen.findByText("Einladung senden");
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

    skipLookupDebounce();
    const submitButton = await screen.findByText("Einladung senden");
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

    skipLookupDebounce();
    const submitButton = await screen.findByText("Einladung senden");
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

    skipLookupDebounce();
    const submitButton = await screen.findByText("Einladung senden");
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
    skipLookupDebounce();

    fireEvent.click(await screen.findByText("Einladung senden"));

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
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Wiederholen" }),
    ).not.toBeInTheDocument();
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

      fireEvent.click(await screen.findByText("Einladung senden"));

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
      skipLookupDebounce();

      fireEvent.click(await screen.findByText("Einladung senden"));

      await waitFor(() => {
        expect(screen.getByText("Rolle").className).toContain(
          "text-moto-red-strong",
        );
      });
    });
  });
});

// Before an invitation goes out, the dialog says whether the address belongs
// to a former employment at this school and what a Wiederaufnahme brings
// back (ADR 0021, #3402).
describe("InvitationForm former employment hint", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.clearAllMocks();
    mockGetRoles.mockResolvedValue(mockRoles);
    mockFindFormerEmployment.mockResolvedValue(null);
  });

  const typeEmail = async (value: string) => {
    render(<InvitationForm />);
    await waitFor(() => {
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled();
    });
    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value },
    });
    skipLookupDebounce();
  };

  it("names the period, what comes back and what does not", async () => {
    mockFindFormerEmployment.mockResolvedValue({
      startedOn: "2024-08-01",
      endedOn: "2026-07-31",
      resumes: [
        "master_data",
        "qualifications",
        "work_time",
        "past_shifts_and_absences",
      ],
      notReturning: [
        "staff_documents",
        "future_shifts_and_absences",
        "groups_classes_substitutions",
      ],
    });

    await typeEmail(" Former@Example.com ");

    expect(
      await screen.findByText("Diese Person war schon bei Ihnen beschäftigt"),
    ).toBeInTheDocument();
    expect(mockFindFormerEmployment).toHaveBeenCalledWith(
      "former@example.com",
      expect.any(AbortSignal),
    );
    expect(
      screen.getByText("In moto eingetragen vom 01.08.2024 bis 31.07.2026."),
    ).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: /Stammdaten/ })).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: /Qualifikationen/ }),
    ).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /Arbeitszeit/ })).toBeChecked();
    expect(
      screen.getByText(
        "Vergangene Schichten und Abwesenheiten bleiben erhalten.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Dokumente, künftige Schichten und Abwesenheiten, Gruppen, Klassen und Vertretungen",
      ),
    ).toBeInTheDocument();
  });

  it("looks nothing like a hint when there is no former employment", async () => {
    await typeEmail("new@example.com");

    await waitFor(() => {
      expect(mockFindFormerEmployment).toHaveBeenCalledWith(
        "new@example.com",
        expect.any(AbortSignal),
      );
    });
    expect(
      screen.queryByText("Diese Person war schon bei Ihnen beschäftigt"),
    ).not.toBeInTheDocument();
  });

  it("does not ask for an incomplete address", async () => {
    await typeEmail("new@exa");

    // Longer than the debounce: a lookup would have started by now.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(600);
    });
    expect(mockFindFormerEmployment).not.toHaveBeenCalled();
  });

  it("drops the hint when the address changes to one without", async () => {
    mockFindFormerEmployment.mockImplementation((email: string) =>
      Promise.resolve(
        email === "former@example.com"
          ? {
              startedOn: "2024-08-01",
              endedOn: "2026-07-31",
              resumes: ["master_data"],
              notReturning: ["staff_documents"],
            }
          : null,
      ),
    );

    await typeEmail("former@example.com");
    expect(
      await screen.findByText("Diese Person war schon bei Ihnen beschäftigt"),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "other@example.com" },
    });
    skipLookupDebounce();

    await waitFor(() => {
      expect(
        screen.queryByText("Diese Person war schon bei Ihnen beschäftigt"),
      ).not.toBeInTheDocument();
    });
  });

  it("waits for a fresh lookup when returning to an address with a former employment", async () => {
    mockFindFormerEmployment.mockImplementation((email: string) => {
      if (email === "former@example.com") {
        return Promise.resolve({
          startedOn: "2024-08-01",
          endedOn: "2026-07-31",
          resumes: ["master_data"],
          notReturning: ["staff_documents"],
        });
      }
      return new Promise(() => undefined);
    });

    await typeEmail("former@example.com");
    expect(
      await screen.findByText("Diese Person war schon bei Ihnen beschäftigt"),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "other@example.com" },
    });
    skipLookupDebounce();
    await waitFor(() =>
      expect(mockFindFormerEmployment).toHaveBeenCalledTimes(2),
    );
    await waitFor(() =>
      expect(screen.getByText("Adresse wird geprüft…")).toBeDisabled(),
    );

    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "former@example.com" },
    });

    expect(
      screen.queryByText("Diese Person war schon bei Ihnen beschäftigt"),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Adresse wird geprüft…")).toBeDisabled();
  });

  it("keeps the dialog usable when the lookup fails", async () => {
    mockFindFormerEmployment.mockRejectedValue(new Error("network"));
    mockCreateInvitation.mockResolvedValue({
      id: 1,
      email: "former@example.com",
      token: "abc123",
    });

    await typeEmail("former@example.com");
    await waitFor(() => {
      expect(mockFindFormerEmployment).toHaveBeenCalled();
    });

    expect(
      screen.queryByText("Diese Person war schon bei Ihnen beschäftigt"),
    ).not.toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("Einladung senden")).not.toBeDisabled();
    });
  });

  it("falls back to the existing invitation behavior when the lookup times out", async () => {
    let lookupSignal: AbortSignal | undefined;
    mockFindFormerEmployment.mockImplementation(
      (_email: string, signal?: AbortSignal) => {
        lookupSignal = signal;
        return new Promise((_, reject) => {
          signal?.addEventListener("abort", () => reject(new Error("aborted")));
        });
      },
    );

    let usesFakeTimers = false;
    try {
      render(<InvitationForm />);
      await waitFor(() =>
        expect(screen.getByLabelText("Rolle")).not.toBeDisabled(),
      );
      vi.useFakeTimers();
      usesFakeTimers = true;
      fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
        target: { value: "former@example.com" },
      });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(400);
      });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5_000);
      });

      expect(lookupSignal?.aborted).toBe(true);
      expect(screen.getByText("Einladung senden")).not.toBeDisabled();
      fireEvent.click(screen.getByText("Einladung senden"));
      await act(async () => {
        await vi.advanceTimersByTimeAsync(0);
      });
      expect(mockCreateInvitation).toHaveBeenCalledWith(
        expect.not.objectContaining({ employmentAction: expect.anything() }),
      );
    } finally {
      if (usesFakeTimers) releaseFakeTimers();
    }
  });
});

describe("InvitationForm employment choice", () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.clearAllMocks();
    mockGetRoles.mockResolvedValue(mockRoles);
    mockCreateInvitation.mockResolvedValue({
      id: 1,
      email: "former@example.com",
      token: "abc123",
    });
    mockFindFormerEmployment.mockResolvedValue({
      startedOn: "2024-08-01",
      endedOn: "2026-07-31",
      resumes: ["master_data", "work_time"],
      notReturning: ["staff_documents"],
    });
  });

  const openFormer = async () => {
    render(<InvitationForm />);
    await waitFor(() =>
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled(),
    );
    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "former@example.com" },
    });
    skipLookupDebounce();
    await screen.findByRole("button", { name: "Neuaufnahme" });
  };

  it("waits for the address lookup before sending", async () => {
    let resolveLookup!: (value: null) => void;
    mockFindFormerEmployment.mockImplementation(
      () =>
        new Promise<null>((resolve) => {
          resolveLookup = resolve;
        }),
    );
    render(<InvitationForm />);
    await waitFor(() =>
      expect(screen.getByLabelText("Rolle")).not.toBeDisabled(),
    );
    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "former@example.com" },
    });
    expect(screen.getByText("Adresse wird geprüft…")).toBeDisabled();
    skipLookupDebounce();
    await waitFor(() => expect(mockFindFormerEmployment).toHaveBeenCalled());
    expect(mockCreateInvitation).not.toHaveBeenCalled();
    resolveLookup(null);
    expect(await screen.findByText("Einladung senden")).not.toBeDisabled();
  });

  it("sends Wiederaufnahme by default for a recognized employment", async () => {
    await openFormer();
    expect(
      screen.getByRole("button", { name: "Wiederaufnahme" }),
    ).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(await screen.findByText("Einladung senden"));
    await waitFor(() =>
      expect(mockCreateInvitation).toHaveBeenCalledWith(
        expect.objectContaining({ employmentAction: "resume" }),
      ),
    );
  });

  it("explains and sends Neuaufnahme", async () => {
    await openFormer();
    fireEvent.click(screen.getByRole("button", { name: "Neuaufnahme" }));
    expect(
      screen.getByText(
        "Eine neue Beschäftigung beginnt. Die frühere bleibt beendet.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Kommt bei Wiederaufnahme zurück:"),
    ).not.toBeInTheDocument();
    fireEvent.click(await screen.findByText("Einladung senden"));
    await waitFor(() =>
      expect(mockCreateInvitation).toHaveBeenCalledWith(
        expect.objectContaining({ employmentAction: "new" }),
      ),
    );
  });

  it("clears the choice when the address changes", async () => {
    await openFormer();
    fireEvent.click(screen.getByRole("button", { name: "Neuaufnahme" }));
    mockFindFormerEmployment.mockResolvedValue(null);
    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "new@example.com" },
    });
    expect(
      screen.queryByRole("button", { name: "Neuaufnahme" }),
    ).not.toBeInTheDocument();
    skipLookupDebounce();
    fireEvent.click(await screen.findByText("Einladung senden"));
    await waitFor(() =>
      expect(mockCreateInvitation).toHaveBeenCalledWith(
        expect.objectContaining({ email: "new@example.com" }),
      ),
    );
    expect(mockCreateInvitation.mock.calls[0]?.[0]).not.toHaveProperty(
      "employmentAction",
    );
  });
  it("stores the three carryover choices and keeps work time together", async () => {
    await openFormer();
    expect(screen.getAllByRole("checkbox")).toHaveLength(3);
    fireEvent.click(screen.getByRole("checkbox", { name: /Stammdaten/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Arbeitszeit/ }));
    fireEvent.click(screen.getByText("Einladung senden"));
    await waitFor(() =>
      expect(mockCreateInvitation).toHaveBeenCalledWith(
        expect.objectContaining({
          employmentAction: "resume",
          resetMasterData: true,
          resetQualifications: false,
          resetWorkTime: true,
        }),
      ),
    );
  });

  it("explains when unchecked qualifications will be reset", async () => {
    await openFormer();
    expect(
      screen.getByText("Bisherige Qualifikationen bleiben erhalten."),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("checkbox", { name: /Qualifikationen/ }));

    expect(
      screen.getByText("Bisherige Qualifikationen werden zurückgesetzt."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Bisherige Qualifikationen bleiben erhalten."),
    ).not.toBeInTheDocument();
  });

  it("hides and omits reset choices for a new employment", async () => {
    await openFormer();
    fireEvent.click(screen.getByRole("checkbox", { name: /Qualifikationen/ }));
    fireEvent.click(screen.getByRole("button", { name: "Neuaufnahme" }));
    expect(screen.queryAllByRole("checkbox")).toHaveLength(0);
    fireEvent.click(screen.getByText("Einladung senden"));
    await waitFor(() => expect(mockCreateInvitation).toHaveBeenCalled());
    const request = mockCreateInvitation.mock.calls[0]?.[0];
    expect(request).toMatchObject({ employmentAction: "new" });
    expect(request).not.toHaveProperty("resetMasterData");
    expect(request).not.toHaveProperty("resetQualifications");
    expect(request).not.toHaveProperty("resetWorkTime");
  });

  it("does not reuse another address's reset choices", async () => {
    await openFormer();
    fireEvent.click(screen.getByRole("checkbox", { name: /Qualifikationen/ }));
    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "another@example.com" },
    });
    skipLookupDebounce();
    await screen.findByRole("checkbox", { name: /Qualifikationen/ });
    expect(
      screen.getByRole("checkbox", { name: /Qualifikationen/ }),
    ).toBeChecked();
  });
});
