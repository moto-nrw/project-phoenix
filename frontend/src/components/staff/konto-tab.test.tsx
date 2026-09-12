import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Teacher } from "~/lib/teacher-api";
import { KontoTab, type KontoEditing } from "./konto-tab";

vi.mock("~/lib/use-clipboard-copy", () => ({
  useClipboardCopy: () => ({ copied: false, copy: vi.fn() }),
}));

// Das Kit-Auswahlfeld ist portaliert und tastaturgesteuert; hier reicht ein
// natives Select, das denselben Wert meldet.
vi.mock("~/components/ui/custom-select", () => ({
  CustomSelect: ({
    id,
    value,
    options,
    onChange,
    disabled,
    ariaLabelledBy,
  }: {
    id?: string;
    value: string;
    options: ReadonlyArray<{ value: string; label: string }>;
    onChange: (next: string) => void;
    disabled?: boolean;
    ariaLabelledBy?: string;
  }) => (
    <select
      id={id}
      aria-labelledby={ariaLabelledBy}
      value={value}
      disabled={disabled}
      onChange={(event) => onChange(event.target.value)}
    >
      <option value="">–</option>
      {options.map((option) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  ),
}));

const teacher: Teacher = {
  id: "42",
  name: "Mila Muster",
  first_name: "Mila",
  last_name: "Muster",
  email: "mila@example.test",
  role: "Betreuung",
  account_role: "teacher",
  tag_id: "ABC123",
  staff_notes: "Springt gerne ein.",
  qualifications: "Erzieherin",
  created_at: "2026-01-05T09:00:00Z",
  account_id: 7,
  is_teacher: true,
};

function editingProps(overrides: Partial<KontoEditing> = {}): KontoEditing {
  return {
    canEditPersonFields: true,
    existingPositions: ["Betreuung", "OGS-Büro"],
    canEditRole: false,
    onEditingChange: vi.fn(),
    onSave: vi.fn(() => Promise.resolve()),
    ...overrides,
  };
}

describe("KontoTab", () => {
  it("shows role, access and account fields of the person", () => {
    render(<KontoTab teacher={teacher} />);

    expect(screen.getByText("Systemrolle")).toBeInTheDocument();
    expect(screen.getByText("Betreuung")).toBeInTheDocument();
    expect(screen.getByText("ABC123")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
    expect(screen.getByText("mila@example.test")).toBeInTheDocument();
    expect(screen.getByText("Erzieherin")).toBeInTheDocument();
    // Notizen und Bearbeiten brauchen staff:manage: ohne den
    // Bearbeiten-Zustand fehlen beide ganz.
    expect(screen.queryByText("Notizen")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Bearbeiten" }),
    ).not.toBeInTheDocument();
  });

  it("shows the notes and the edit button to the staff managers", () => {
    render(<KontoTab teacher={teacher} editing={editingProps()} />);

    expect(screen.getByText("Notizen")).toBeInTheDocument();
    expect(screen.getByText("Springt gerne ein.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Bearbeiten" })).toBeVisible();
  });

  it("edits name, position and notes in one edit state with one save button", async () => {
    const editing = editingProps();
    render(<KontoTab teacher={teacher} editing={editing} />);

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    expect(editing.onEditingChange).toHaveBeenCalledWith(true);
    expect(
      screen.queryByRole("button", { name: "Bearbeiten" }),
    ).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Vorname"), {
      target: { value: "Milena" },
    });
    fireEvent.change(screen.getByLabelText("Position"), {
      target: { value: "OGS-Büro" },
    });
    fireEvent.change(screen.getByLabelText("Notizen der Leitung"), {
      target: { value: "Neue Notiz" },
    });
    expect(screen.getAllByRole("button", { name: "Speichern" })).toHaveLength(
      1,
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(editing.onSave).toHaveBeenCalledWith({
        first_name: "Milena",
        last_name: "Muster",
        role: "OGS-Büro",
        staff_notes: "Neue Notiz",
      });
    });
    await waitFor(() => {
      expect(
        screen.queryByLabelText("Notizen der Leitung"),
      ).not.toBeInTheDocument();
    });
    expect(editing.onEditingChange).toHaveBeenLastCalledWith(false);
  });

  it("shows the name read-only without users:update and leaves it out of the save", async () => {
    const editing = editingProps({ canEditPersonFields: false });
    render(<KontoTab teacher={teacher} editing={editing} />);

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));

    expect(screen.queryByLabelText("Vorname")).not.toBeInTheDocument();
    expect(
      screen.getByText("Name und RFID-Karte ändert die OGS-Leitung."),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(editing.onSave).toHaveBeenCalledWith({
        role: "Betreuung",
        staff_notes: "Springt gerne ein.",
      });
    });
  });

  it("marks an empty name and keeps the edit state", async () => {
    const editing = editingProps();
    render(<KontoTab teacher={teacher} editing={editing} />);

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.change(screen.getByLabelText("Nachname"), {
      target: { value: "  " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText("Nachname ist erforderlich."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Bitte prüfen Sie die markierten Felder."),
    ).toBeInTheDocument();
    expect(editing.onSave).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Nachname")).toBeInTheDocument();
  });

  it("reports a failed save in the alert and stays in the edit state", async () => {
    const editing = editingProps({
      onSave: vi.fn(() =>
        Promise.reject(
          new Error("Die Änderungen konnten nicht gespeichert werden."),
        ),
      ),
    });
    render(<KontoTab teacher={teacher} editing={editing} />);

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Die Änderungen konnten nicht gespeichert werden.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Notizen der Leitung")).toBeInTheDocument();
  });

  it("sends the new system role only when it differs from the current one", async () => {
    const editing = editingProps({
      canEditRole: true,
      roleAssignment: {
        options: [
          { id: 1, name: "Administration", systemName: "admin" },
          { id: 2, name: "Betreuung", systemName: "user" },
        ],
        currentRoleIds: [2],
        currentIsLehrkraft: false,
      },
    });
    render(<KontoTab teacher={teacher} editing={editing} />);

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    const roleField = screen.getByLabelText<HTMLSelectElement>("Systemrolle");
    expect(roleField.value).toBe("2");

    // Unverändert: keine role_id im Entwurf.
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() => {
      expect(editing.onSave).toHaveBeenCalledWith(
        expect.not.objectContaining({ role_id: expect.anything() }),
      );
    });

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.change(screen.getByLabelText("Systemrolle"), {
      target: { value: "1" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() => {
      expect(editing.onSave).toHaveBeenLastCalledWith(
        expect.objectContaining({ role_id: 1 }),
      );
    });
  });

  it("keeps the role of a Lehrkraft read-only and explains why", () => {
    render(
      <KontoTab
        teacher={{ ...teacher, account_role: "lehrkraft", is_teacher: false }}
        editing={editingProps({
          canEditRole: true,
          roleAssignment: {
            options: [{ id: 1, name: "Administration", systemName: "admin" }],
            currentRoleIds: [3],
            currentIsLehrkraft: true,
          },
        })}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));

    expect(screen.queryByLabelText("Systemrolle")).not.toBeInTheDocument();
    expect(
      screen.getByText(/Lehrkraft-Zugänge haben kein Betreuungsprofil/),
    ).toBeInTheDocument();
    // Ohne Betreuungsprofil gibt es keine Position zu speichern.
    expect(screen.queryByLabelText("Position")).not.toBeInTheDocument();
  });

  it("drops the draft on cancel", () => {
    const editing = editingProps();
    render(<KontoTab teacher={teacher} editing={editing} />);

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.change(screen.getByLabelText("Notizen der Leitung"), {
      target: { value: "Verworfen" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

    expect(editing.onSave).not.toHaveBeenCalled();
    expect(editing.onEditingChange).toHaveBeenLastCalledWith(false);
    expect(screen.getByText("Springt gerne ein.")).toBeInTheDocument();
    expect(screen.queryByText("Verworfen")).not.toBeInTheDocument();
  });
});
