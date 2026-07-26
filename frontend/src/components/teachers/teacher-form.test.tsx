import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { TeacherForm } from "./teacher-form";

const { mockGetRoles, mockLoggerError } = vi.hoisted(() => ({
  mockGetRoles: vi.fn(() =>
    Promise.resolve([
      { id: "1", name: "admin" },
      { id: "2", name: "betreuer" },
    ]),
  ),
  mockLoggerError: vi.fn(),
}));

vi.mock("@/lib/auth-service", () => ({
  authService: {
    getRoles: mockGetRoles,
  },
}));

vi.mock("@/lib/auth-helpers", () => {
  const getRoleDisplayName = (name: string) => {
    const names: Record<string, string> = {
      admin: "Administrator",
      betreuer: "Betreuer",
    };
    return names[name] ?? name;
  };
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

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    error: mockLoggerError,
    info: vi.fn(),
    warn: vi.fn(),
    debug: vi.fn(),
    child: vi.fn(),
  }),
}));

describe("TeacherForm", () => {
  const defaultProps = {
    initialData: {},
    onSubmitAction: vi.fn().mockResolvedValue(undefined),
    onCancelAction: vi.fn(),
    isLoading: false,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockGetRoles.mockResolvedValue([
      { id: "1", name: "admin" },
      { id: "2", name: "betreuer" },
    ]);
  });

  it("renders form with default title", () => {
    render(<TeacherForm {...defaultProps} />);

    expect(screen.getByText("Details des Personals")).toBeInTheDocument();
  });

  it("renders form with custom title", () => {
    render(<TeacherForm {...defaultProps} formTitle="Custom Title" />);

    expect(screen.getByText("Custom Title")).toBeInTheDocument();
  });

  it("hides title when wrapInCard is false", () => {
    render(
      <TeacherForm {...defaultProps} wrapInCard={false} formTitle="Hidden" />,
    );

    expect(screen.queryByText("Hidden")).not.toBeInTheDocument();
  });

  it("renders firstName and lastName fields", () => {
    render(<TeacherForm {...defaultProps} />);

    expect(screen.getByLabelText(/Vorname/)).toBeInTheDocument();
    expect(screen.getByLabelText(/Nachname/)).toBeInTheDocument();
  });

  it("populates form with initial data", () => {
    render(
      <TeacherForm
        {...defaultProps}
        initialData={{
          id: "1",
          first_name: "John",
          last_name: "Doe",
          role: "Pädagogische Fachkraft",
        }}
      />,
    );

    expect(screen.getByDisplayValue("John")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Doe")).toBeInTheDocument();
  });

  it("shows email and password fields for new teacher", () => {
    render(<TeacherForm {...defaultProps} initialData={{}} />);

    expect(screen.getByLabelText(/E-Mail/)).toBeInTheDocument();
    expect(screen.getByLabelText(/^Passwort \*/)).toBeInTheDocument();
    expect(screen.getByLabelText(/Passwort bestätigen/)).toBeInTheDocument();
  });

  it("hides email and password fields for existing teacher", () => {
    render(
      <TeacherForm
        {...defaultProps}
        initialData={{ id: "1", first_name: "John", last_name: "Doe" }}
      />,
    );

    expect(screen.queryByLabelText(/E-Mail/)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/^Passwort \*/)).not.toBeInTheDocument();
  });

  it("shows role selection for new teacher", async () => {
    render(<TeacherForm {...defaultProps} initialData={{}} />);

    await waitFor(() => {
      expect(screen.getByLabelText(/System-Rolle/)).toBeInTheDocument();
    });
  });

  it("filters unsupported legacy roles from the selectable roles", async () => {
    mockGetRoles.mockResolvedValue([
      { id: "1", name: "guardian" },
      { id: "2", name: "teacher" },
      { id: "3", name: "admin" },
    ]);

    render(<TeacherForm {...defaultProps} initialData={{}} />);

    await waitFor(() => {
      expect(screen.getByLabelText(/System-Rolle/)).toBeInTheDocument();
    });

    fireEvent.click(screen.getByLabelText(/System-Rolle/));

    expect(
      screen.queryByRole("option", { name: "guardian" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: "teacher" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: "Administrator" }),
    ).toBeInTheDocument();
  });

  it("shows position input with placeholder", () => {
    render(<TeacherForm {...defaultProps} />);

    expect(screen.getByLabelText(/Position/)).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText("z.B. Pädagogische Fachkraft, OGS-Büro"),
    ).toBeInTheDocument();
  });

  it("renders position suggestions when existing positions are provided", () => {
    const { container } = render(
      <TeacherForm
        {...defaultProps}
        existingPositions={["Pädagogische Fachkraft", "OGS-Büro"]}
      />,
    );

    const input = screen.getByLabelText(/Position/);
    expect(input).toHaveAttribute("list", "teacher-position-suggestions");
    expect(
      container.querySelector(
        'datalist#teacher-position-suggestions option[value="Pädagogische Fachkraft"]',
      ),
    ).toBeTruthy();
    expect(
      container.querySelector(
        'datalist#teacher-position-suggestions option[value="OGS-Büro"]',
      ),
    ).toBeTruthy();
  });

  it("disables inputs when loading", () => {
    render(<TeacherForm {...defaultProps} isLoading={true} />);

    expect(screen.getByLabelText(/Vorname/)).toBeDisabled();
    expect(screen.getByLabelText(/Nachname/)).toBeDisabled();
  });

  it("shows custom submit label", () => {
    render(<TeacherForm {...defaultProps} submitLabel="Create Teacher" />);

    expect(screen.getByText("Create Teacher")).toBeInTheDocument();
  });

  it("shows validation error for empty firstName", async () => {
    render(<TeacherForm {...defaultProps} initialData={{ id: "1" }} />);

    const submitBtn = screen.getByRole("button", { name: /Speichern/ });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(screen.getByText("Vorname ist erforderlich")).toBeInTheDocument();
    });
  });

  it("shows validation error for empty lastName", async () => {
    render(<TeacherForm {...defaultProps} initialData={{ id: "1" }} />);

    fireEvent.change(screen.getByLabelText(/Vorname/), {
      target: { value: "John" },
    });

    const submitBtn = screen.getByRole("button", { name: /Speichern/ });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(screen.getByText("Nachname ist erforderlich")).toBeInTheDocument();
    });
  });

  it("validates email format for new teacher", async () => {
    render(<TeacherForm {...defaultProps} initialData={{}} />);

    fireEvent.change(screen.getByLabelText(/Vorname/), {
      target: { value: "John" },
    });
    fireEvent.change(screen.getByLabelText(/Nachname/), {
      target: { value: "Doe" },
    });
    fireEvent.change(screen.getByLabelText(/E-Mail/), {
      target: { value: "invalid-email" },
    });

    const submitBtn = screen.getByRole("button", { name: /Speichern/ });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(screen.getByText("Ungültige E-Mail-Adresse")).toBeInTheDocument();
    });
  });

  it("requires an email for new teachers", async () => {
    render(<TeacherForm {...defaultProps} initialData={{}} />);

    await waitFor(() => {
      expect(screen.getByLabelText(/System-Rolle/)).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/Vorname/), {
      target: { value: "John" },
    });
    fireEvent.change(screen.getByLabelText(/Nachname/), {
      target: { value: "Doe" },
    });
    fireEvent.click(screen.getByLabelText(/System-Rolle/));
    fireEvent.click(screen.getByRole("option", { name: "Administrator" }));
    fireEvent.change(screen.getByLabelText(/^Passwort \*/), {
      target: { value: "ValidPass1!" },
    });
    fireEvent.change(screen.getByLabelText(/Passwort bestätigen/), {
      target: { value: "ValidPass1!" },
    });

    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => {
      expect(screen.getByText("E-Mail ist erforderlich")).toBeInTheDocument();
    });
  });

  it("validates password minimum length", async () => {
    render(<TeacherForm {...defaultProps} initialData={{}} />);

    fireEvent.change(screen.getByLabelText(/Vorname/), {
      target: { value: "John" },
    });
    fireEvent.change(screen.getByLabelText(/Nachname/), {
      target: { value: "Doe" },
    });
    fireEvent.change(screen.getByLabelText(/E-Mail/), {
      target: { value: "john@example.com" },
    });
    fireEvent.change(screen.getByLabelText(/^Passwort \*/), {
      target: { value: "short" },
    });

    const submitBtn = screen.getByRole("button", { name: /Speichern/ });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(
        screen.getByText("Passwort muss mindestens 8 Zeichen lang sein"),
      ).toBeInTheDocument();
    });
  });

  it("validates password requires uppercase", async () => {
    render(<TeacherForm {...defaultProps} initialData={{}} />);

    fireEvent.change(screen.getByLabelText(/Vorname/), {
      target: { value: "John" },
    });
    fireEvent.change(screen.getByLabelText(/Nachname/), {
      target: { value: "Doe" },
    });
    fireEvent.change(screen.getByLabelText(/E-Mail/), {
      target: { value: "john@example.com" },
    });
    fireEvent.change(screen.getByLabelText(/^Passwort \*/), {
      target: { value: "alllowercase1!" },
    });

    const submitBtn = screen.getByRole("button", { name: /Speichern/ });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(
        screen.getByText(
          "Passwort muss mindestens einen Großbuchstaben enthalten",
        ),
      ).toBeInTheDocument();
    });
  });

  it("validates password confirmation matches", async () => {
    render(<TeacherForm {...defaultProps} initialData={{}} />);

    fireEvent.change(screen.getByLabelText(/Vorname/), {
      target: { value: "John" },
    });
    fireEvent.change(screen.getByLabelText(/Nachname/), {
      target: { value: "Doe" },
    });
    fireEvent.change(screen.getByLabelText(/E-Mail/), {
      target: { value: "john@example.com" },
    });
    fireEvent.change(screen.getByLabelText(/^Passwort \*/), {
      target: { value: "ValidPass1!" },
    });
    fireEvent.change(screen.getByLabelText(/Passwort bestätigen/), {
      target: { value: "DifferentPass1!" },
    });

    // Select a role
    await waitFor(() => {
      expect(screen.getByLabelText(/System-Rolle/)).toBeInTheDocument();
    });
    fireEvent.change(screen.getByLabelText(/System-Rolle/), {
      target: { value: "1" },
    });

    const submitBtn = screen.getByRole("button", { name: /Speichern/ });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(
        screen.getByText("Passwörter stimmen nicht überein"),
      ).toBeInTheDocument();
    });
  });

  it("calls onCancelAction when cancel button clicked", () => {
    const onCancelAction = vi.fn();
    render(<TeacherForm {...defaultProps} onCancelAction={onCancelAction} />);

    fireEvent.click(screen.getByText("Abbrechen"));

    expect(onCancelAction).toHaveBeenCalledTimes(1);
  });

  it("calls onSubmitAction with valid data for existing teacher", async () => {
    const onSubmitAction = vi.fn().mockResolvedValue(undefined);
    render(
      <TeacherForm
        {...defaultProps}
        initialData={{ id: "1", person_id: 10 }}
        onSubmitAction={onSubmitAction}
      />,
    );

    fireEvent.change(screen.getByLabelText(/Vorname/), {
      target: { value: "John" },
    });
    fireEvent.change(screen.getByLabelText(/Nachname/), {
      target: { value: "Doe" },
    });

    const submitBtn = screen.getByRole("button", { name: /Speichern/ });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(onSubmitAction).toHaveBeenCalledWith(
        expect.objectContaining({
          first_name: "John",
          last_name: "Doe",
          id: "1",
          person_id: 10, // person_id is passed through as number
          is_teacher: true,
        }),
      );
    });
  });

  it("shows loading state on submit button", () => {
    render(<TeacherForm {...defaultProps} isLoading={true} />);

    expect(screen.getByText("Wird gespeichert...")).toBeInTheDocument();
  });

  it("shows RFID section when showRFID is true", () => {
    render(<TeacherForm {...defaultProps} showRFID={true} />);

    expect(
      screen.getByText("RFID-Karte (Funktion nicht verfügbar)"),
    ).toBeInTheDocument();
    expect(screen.getByText("RFID-Funktion deaktiviert")).toBeInTheDocument();
  });

  it("keeps the RFID selector disabled when shown", () => {
    render(<TeacherForm {...defaultProps} showRFID={true} />);

    expect(screen.getByLabelText(/RFID-Karte/)).toBeDisabled();
  });

  it("hides RFID section by default", () => {
    render(<TeacherForm {...defaultProps} />);

    expect(
      screen.queryByText("RFID-Karte (Funktion nicht verfügbar)"),
    ).not.toBeInTheDocument();
  });

  it("displays personal information section header", () => {
    render(<TeacherForm {...defaultProps} />);

    expect(screen.getByText("Persönliche Informationen")).toBeInTheDocument();
  });

  it("displays professional information section header", () => {
    render(<TeacherForm {...defaultProps} />);

    expect(screen.getByText("Berufliche Informationen")).toBeInTheDocument();
  });

  it("shows role selection helper text", async () => {
    render(<TeacherForm {...defaultProps} initialData={{}} />);

    await waitFor(() => {
      expect(
        screen.getByText("Bitte wähle eine Rolle aus"),
      ).toBeInTheDocument();
    });
  });

  it("updates helper text when a role is selected", async () => {
    render(<TeacherForm {...defaultProps} initialData={{}} />);

    await waitFor(() => {
      expect(screen.getByLabelText(/System-Rolle/)).toBeInTheDocument();
    });

    fireEvent.click(screen.getByLabelText(/System-Rolle/));
    fireEvent.click(screen.getByRole("option", { name: "Administrator" }));

    expect(
      screen.getByText("Rolle kann später geändert werden"),
    ).toBeInTheDocument();
  });

  it("submits new teacher with selected role id", async () => {
    const onSubmitAction = vi.fn().mockResolvedValue(undefined);

    render(
      <TeacherForm
        {...defaultProps}
        initialData={{}}
        onSubmitAction={onSubmitAction}
      />,
    );

    await waitFor(() => {
      expect(screen.getByLabelText(/System-Rolle/)).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText(/Vorname/), {
      target: { value: " Anna " },
    });
    fireEvent.change(screen.getByLabelText(/Nachname/), {
      target: { value: " Schmidt " },
    });
    fireEvent.change(screen.getByLabelText(/E-Mail/), {
      target: { value: "anna@example.com" },
    });
    fireEvent.change(screen.getByLabelText(/Position/), {
      target: { value: " Betreuung " },
    });
    fireEvent.click(screen.getByLabelText(/System-Rolle/));
    fireEvent.click(screen.getByRole("option", { name: "Betreuer" }));
    fireEvent.change(screen.getByLabelText(/^Passwort \*/), {
      target: { value: "ValidPass1!" },
    });
    fireEvent.change(screen.getByLabelText(/Passwort bestätigen/), {
      target: { value: "ValidPass1!" },
    });

    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => {
      expect(onSubmitAction).toHaveBeenCalledWith(
        expect.objectContaining({
          first_name: "Anna",
          last_name: "Schmidt",
          email: "anna@example.com",
          role: "Betreuung",
          role_id: 2,
          is_teacher: true,
        }),
      );
    });
  });

  it("logs role-loading errors without crashing the form", async () => {
    mockGetRoles.mockRejectedValueOnce(new Error("roles unavailable"));

    render(<TeacherForm {...defaultProps} initialData={{}} />);

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith(
        "failed to load roles",
        expect.objectContaining({
          error: "roles unavailable",
        }),
      );
    });

    expect(screen.getByLabelText(/System-Rolle/)).toBeInTheDocument();
  });

  describe("Scroll to error", () => {
    it("scrolls to error when submission fails with API error", async () => {
      const scrollIntoViewMock = vi.fn();
      Element.prototype.scrollIntoView = scrollIntoViewMock;

      const onSubmitAction = vi.fn().mockRejectedValue(new Error("API Error"));

      render(
        <TeacherForm
          {...defaultProps}
          initialData={{ id: "1", person_id: 10 }}
          onSubmitAction={onSubmitAction}
        />,
      );

      fireEvent.change(screen.getByLabelText(/Vorname/), {
        target: { value: "John" },
      });
      fireEvent.change(screen.getByLabelText(/Nachname/), {
        target: { value: "Doe" },
      });

      const submitBtn = screen.getByRole("button", { name: /Speichern/ });
      fireEvent.click(submitBtn);

      await waitFor(() => {
        expect(
          screen.getByText(/Es ist ein Fehler aufgetreten/),
        ).toBeInTheDocument();
      });

      await waitFor(() => {
        expect(scrollIntoViewMock).toHaveBeenCalledWith({
          behavior: "smooth",
          block: "start",
        });
      });
    });
  });

  it("resets the form when editing switches to another teacher", async () => {
    const { rerender } = render(
      <TeacherForm
        {...defaultProps}
        initialData={{ id: "1", first_name: "Anna", last_name: "Alt" }}
      />,
    );

    fireEvent.change(screen.getByLabelText(/Vorname/), {
      target: { value: "Changed" },
    });
    expect(screen.getByDisplayValue("Changed")).toBeInTheDocument();

    rerender(
      <TeacherForm
        {...defaultProps}
        initialData={{
          id: "2",
          first_name: "Bea",
          last_name: "Neu",
          email: "bea@example.com",
          role: "OGS-Büro",
          tag_id: "RFID-42",
        }}
      />,
    );

    await waitFor(() => {
      expect(screen.getByDisplayValue("Bea")).toBeInTheDocument();
      expect(screen.getByDisplayValue("Neu")).toBeInTheDocument();
      expect(screen.getByDisplayValue("OGS-Büro")).toBeInTheDocument();
    });
  });
});
