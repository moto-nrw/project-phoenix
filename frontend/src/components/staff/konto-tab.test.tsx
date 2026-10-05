import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Teacher } from "~/lib/teacher-api";
import { ApiError } from "~/lib/api-error";
import {
  KontoRoleSaveError,
  KontoTab,
  type KontoDraft,
  type KontoEditing,
} from "./konto-tab";

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
    ariaDescribedBy,
    name,
    invalid,
  }: {
    id?: string;
    value: string;
    options: ReadonlyArray<{ value: string; label: string }>;
    onChange: (next: string) => void;
    disabled?: boolean;
    ariaLabelledBy?: string;
    ariaDescribedBy?: string;
    name?: string;
    invalid?: boolean;
  }) => (
    <select
      id={id}
      name={name}
      aria-labelledby={ariaLabelledBy}
      aria-describedby={ariaDescribedBy}
      aria-invalid={invalid || undefined}
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
  account_id: "7",
  is_teacher: true,
};

function editingProps(overrides: Partial<KontoEditing> = {}): KontoEditing {
  return {
    canEditPersonFields: true,
    canEditStaffFields: true,
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

  it("disables saving until both required name fields have content", () => {
    const editing = editingProps();
    render(<KontoTab teacher={teacher} editing={editing} />);

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.change(screen.getByLabelText("Nachname"), {
      target: { value: "  " },
    });
    expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();
    expect(editing.onSave).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Nachname")).toBeInTheDocument();
  });

  it("disables saving while the editable system role is still loading", () => {
    const editing = editingProps({ canEditRole: true });
    render(<KontoTab teacher={teacher} editing={editing} />);

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));

    expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();
    expect(editing.onSave).not.toHaveBeenCalled();
  });

  it("reports a failed save in the form alert, never the raw message, and stays in the edit state", async () => {
    const editing = editingProps({
      onSave: vi.fn(() => Promise.reject(new Error("socket hang up"))),
    });
    render(<KontoTab teacher={teacher} editing={editing} />);

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Das Konto konnte nicht bearbeitet werden. Bitte versuchen Sie es später erneut.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/socket hang up/)).not.toBeInTheDocument();
    expect(screen.getByLabelText("Notizen der Leitung")).toBeInTheDocument();
  });

  it("marks the fields the server rejects and focuses the first", async () => {
    const editing = editingProps({
      onSave: vi.fn(() =>
        Promise.reject(
          new ApiError("first name is required", 400, {
            code: "general.input",
            errors: [
              { field: "first_name", reason: "is required" },
              { field: "last_name", reason: "is required" },
            ],
          }),
        ),
      ),
    });
    render(<KontoTab teacher={teacher} editing={editing} />);

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    const first = screen.getByLabelText("Vorname");
    await waitFor(() => expect(first).toHaveFocus());
    expect(first).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByLabelText("Nachname")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(
      screen.getByText(
        "Das Konto konnte nicht übernommen werden. Bitte prüfen Sie Ihre Angaben.",
      ),
    ).toBeInTheDocument();
  });

  it("retries with the draft as it is now, not as it was when the save failed", async () => {
    const onSave = vi
      .fn<(draft: unknown) => Promise<void>>()
      .mockRejectedValueOnce(
        new ApiError("unavailable", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce(undefined);
    const editing = editingProps({ onSave });
    render(<KontoTab teacher={teacher} editing={editing} />);

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    const retry = await screen.findByRole("button", { name: "Wiederholen" });
    fireEvent.change(screen.getByLabelText("Notizen der Leitung"), {
      target: { value: "Neu nach dem Fehler" },
    });
    fireEvent.click(retry);

    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(2));
    expect(onSave).toHaveBeenLastCalledWith(
      expect.objectContaining({ staff_notes: "Neu nach dem Fehler" }),
    );
  });

  it("names the system role when only the role change failed", async () => {
    const editing = editingProps({
      onSave: vi.fn(() =>
        Promise.reject(
          new KontoRoleSaveError(
            new ApiError("forbidden", 403, { code: "general.permission" }),
          ),
        ),
      ),
    });
    render(<KontoTab teacher={teacher} editing={editing} />);

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Für die Systemrolle fehlt Ihnen die Berechtigung. Bitte fragen Sie die Schule.",
      ),
    ).toBeInTheDocument();
  });

  it("marks and focuses the system role when the server rejects it", async () => {
    const editing = editingProps({
      canEditRole: true,
      roleAssignment: {
        options: [
          { id: "1", name: "Administration", systemName: "admin" },
          { id: "2", name: "Betreuung", systemName: "user" },
        ],
        currentRoleIds: ["2"],
        currentIsLehrkraft: false,
      },
      onSave: vi.fn(() =>
        Promise.reject(
          new KontoRoleSaveError(
            new ApiError("role is invalid", 400, {
              code: "general.input",
              errors: [{ field: "role_id", reason: "is invalid" }],
            }),
          ),
        ),
      ),
    });
    render(<KontoTab teacher={teacher} editing={editing} />);

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    const role = screen.getByLabelText("Systemrolle");
    fireEvent.change(role, { target: { value: "1" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(role).toHaveFocus());
    expect(role).toHaveAttribute("name", "role_id");
    expect(role).toHaveAttribute("aria-invalid", "true");
    expect(role).toHaveAttribute("aria-describedby", "konto-role-error");
    expect(document.getElementById("konto-role-error")).toHaveTextContent(
      "Bitte prüfen Sie dieses Feld.",
    );
  });

  it("retries only the system role after the account fields were saved", async () => {
    const onSave = vi
      .fn<(draft: KontoDraft) => Promise<void>>()
      .mockRejectedValueOnce(
        new KontoRoleSaveError(
          new ApiError("unavailable", 503, { code: "general.unavailable" }),
        ),
      )
      .mockResolvedValueOnce(undefined);
    const editing = editingProps({
      canEditRole: true,
      roleAssignment: {
        options: [
          { id: "1", name: "Administration", systemName: "admin" },
          { id: "2", name: "Betreuung", systemName: "user" },
        ],
        currentRoleIds: ["2"],
        currentIsLehrkraft: false,
      },
      onSave,
    });
    render(<KontoTab teacher={teacher} editing={editing} />);

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.change(screen.getByLabelText("Vorname"), {
      target: { value: "Milena" },
    });
    fireEvent.change(screen.getByLabelText("Systemrolle"), {
      target: { value: "1" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    const retry = await screen.findByRole("button", { name: "Wiederholen" });
    fireEvent.change(screen.getByLabelText("Vorname"), {
      target: { value: "Mia" },
    });
    fireEvent.click(retry);

    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(2));
    expect(onSave).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        first_name: "Milena",
        last_name: "Muster",
        role_id: "1",
      }),
    );
    expect(onSave).toHaveBeenLastCalledWith({ role_id: "1" });
  });

  it("sends the new system role only when it differs from the current one", async () => {
    const editing = editingProps({
      canEditRole: true,
      roleAssignment: {
        options: [
          { id: "1", name: "Administration", systemName: "admin" },
          { id: "2", name: "Betreuung", systemName: "user" },
        ],
        currentRoleIds: ["2"],
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
        expect.objectContaining({ role_id: "1" }),
      );
    });
  });

  it("treats a further role of the account as a change to the shown one", async () => {
    // Zwei Systemrollen sind der Rest eines halb fertigen Wechsels. Das Feld
    // zeigt die älteste; sie erneut zu wählen ändert nichts, die andere zu
    // wählen macht sie zur einzigen.
    const editing = editingProps({
      canEditPersonFields: false,
      canEditStaffFields: false,
      canEditRole: true,
      roleAssignment: {
        options: [
          { id: "1", name: "Administration", systemName: "admin" },
          { id: "2", name: "Betreuung", systemName: "user" },
        ],
        currentRoleIds: ["2", "1"],
        currentIsLehrkraft: false,
      },
    });
    render(<KontoTab teacher={teacher} editing={editing} />);

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    expect(screen.getByLabelText<HTMLSelectElement>("Systemrolle").value).toBe(
      "2",
    );
    fireEvent.change(screen.getByLabelText("Systemrolle"), {
      target: { value: "2" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(editing.onSave).toHaveBeenCalledWith({});
    });

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.change(screen.getByLabelText("Systemrolle"), {
      target: { value: "1" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() => {
      expect(editing.onSave).toHaveBeenLastCalledWith({ role_id: "1" });
    });
  });

  it("shows the staff role of a parent who became staff and keeps it unchanged", async () => {
    // Die guardian-Rolle ist die älteste des Kontos und nicht wählbar. Das
    // Feld zeigt trotzdem die Systemrolle; sie zu bestätigen sendet keinen
    // Rollenwechsel, der den Elternzugang gefährden könnte.
    const editing = editingProps({
      canEditPersonFields: false,
      canEditStaffFields: false,
      canEditRole: true,
      roleAssignment: {
        options: [
          { id: "1", name: "Administration", systemName: "admin" },
          { id: "2", name: "Betreuung", systemName: "user" },
        ],
        currentRoleIds: ["9", "2"],
        currentIsLehrkraft: false,
      },
    });
    render(<KontoTab teacher={teacher} editing={editing} />);

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    expect(screen.getByLabelText<HTMLSelectElement>("Systemrolle").value).toBe(
      "2",
    );
    fireEvent.change(screen.getByLabelText("Systemrolle"), {
      target: { value: "2" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(editing.onSave).toHaveBeenCalledWith({});
    });

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.change(screen.getByLabelText("Systemrolle"), {
      target: { value: "1" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() => {
      expect(editing.onSave).toHaveBeenLastCalledWith({ role_id: "1" });
    });
  });

  it("changes only the system role without staff management", async () => {
    const editing = editingProps({
      canEditPersonFields: false,
      canEditStaffFields: false,
      canEditRole: true,
      roleAssignment: {
        options: [
          { id: "1", name: "Administration", systemName: "admin" },
          { id: "2", name: "Betreuung", systemName: "user" },
        ],
        currentRoleIds: ["2"],
        currentIsLehrkraft: false,
      },
    });
    render(<KontoTab teacher={teacher} editing={editing} />);

    expect(screen.queryByText("Notizen")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));

    expect(screen.queryByLabelText("Position")).not.toBeInTheDocument();
    expect(
      screen.queryByLabelText("Notizen der Leitung"),
    ).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Systemrolle"), {
      target: { value: "1" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(editing.onSave).toHaveBeenCalledWith({ role_id: "1" });
    });
  });

  it("keeps the role of a Lehrkraft read-only and explains why", () => {
    render(
      <KontoTab
        teacher={{ ...teacher, account_role: "lehrkraft", is_teacher: false }}
        editing={editingProps({
          canEditRole: true,
          roleAssignment: {
            options: [{ id: "1", name: "Administration", systemName: "admin" }],
            currentRoleIds: ["3"],
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
