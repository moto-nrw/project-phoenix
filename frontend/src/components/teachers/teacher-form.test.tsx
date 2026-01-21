import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { TeacherForm } from "./teacher-form";

vi.mock("@/lib/auth-service", () => ({
  authService: {
    getRoles: vi.fn(() =>
      Promise.resolve([
        { id: "1", name: "admin" },
        { id: "2", name: "betreuer" },
      ]),
    ),
  },
}));

vi.mock("@/lib/auth-helpers", () => ({
  getRoleDisplayName: (name: string) => {
    const names: Record<string, string> = {
      admin: "Administrator",
      betreuer: "Betreuer",
    };
    return names[name] ?? name;
  },
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
  });

  it("renders form with default title", () => {
    render(<TeacherForm {...defaultProps} />);

    expect(
      screen.getByText("Details der pädagogischen Fachkraft"),
    ).toBeInTheDocument();
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

  it("shows position dropdown", () => {
    render(<TeacherForm {...defaultProps} />);

    expect(screen.getByLabelText(/Position/)).toBeInTheDocument();
    expect(screen.getByText("Pädagogische Fachkraft")).toBeInTheDocument();
    expect(screen.getByText("OGS-Büro")).toBeInTheDocument();
    expect(screen.getByText("Extern")).toBeInTheDocument();
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
      expect(
        screen.getByText("Ungültige E-Mail-Adresse"),
      ).toBeInTheDocument();
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
    expect(
      screen.getByText("RFID-Funktion deaktiviert"),
    ).toBeInTheDocument();
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
});
