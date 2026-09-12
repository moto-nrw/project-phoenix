import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Teacher } from "~/lib/teacher-api";
import { KontoTab } from "./konto-tab";

vi.mock("~/lib/use-clipboard-copy", () => ({
  useClipboardCopy: () => ({ copied: false, copy: vi.fn() }),
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
};

describe("KontoTab", () => {
  it("shows role, access and account fields of the person", () => {
    render(<KontoTab teacher={teacher} />);

    expect(screen.getByText("Systemrolle")).toBeInTheDocument();
    expect(screen.getByText("Betreuung")).toBeInTheDocument();
    expect(screen.getByText("ABC123")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
    expect(screen.getByText("mila@example.test")).toBeInTheDocument();
    expect(screen.getByText("Erzieherin")).toBeInTheDocument();
    // Notizen brauchen staff:manage: ohne Handler fehlt der Abschnitt ganz.
    expect(screen.queryByText("Notizen")).not.toBeInTheDocument();
    expect(screen.queryByText("Konto und Zugriff")).not.toBeInTheDocument();
  });

  it("edits the notes in an edit state with one save button", async () => {
    const onUpdateNotes = vi.fn(() => Promise.resolve());
    render(<KontoTab teacher={teacher} onUpdateNotes={onUpdateNotes} />);

    expect(screen.getByText("Springt gerne ein.")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Notizen bearbeiten" }));
    const field = screen.getByLabelText<HTMLTextAreaElement>(
      "Notizen der Leitung",
    );
    fireEvent.change(field, { target: { value: "Neue Notiz" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(onUpdateNotes).toHaveBeenCalledWith("Neue Notiz");
    });
    await waitFor(() => {
      expect(
        screen.queryByLabelText("Notizen der Leitung"),
      ).not.toBeInTheDocument();
    });
  });

  it("keeps the edit state and reports when the notes could not be saved", async () => {
    const onUpdateNotes = vi.fn(() => Promise.reject(new Error("offline")));
    render(<KontoTab teacher={teacher} onUpdateNotes={onUpdateNotes} />);

    fireEvent.click(screen.getByRole("button", { name: "Notizen bearbeiten" }));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(
        screen.getByText("Die Notizen konnten nicht gespeichert werden."),
      ).toBeInTheDocument();
    });
    expect(screen.getByLabelText("Notizen der Leitung")).toBeInTheDocument();
  });

  it("puts the first account action next to the section and the rest in the menu", () => {
    const onManageRole = vi.fn();
    const onManageMFA = vi.fn();
    render(
      <KontoTab
        teacher={teacher}
        onManageRole={onManageRole}
        onManageMFA={onManageMFA}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Rolle verwalten" }));
    expect(onManageRole).toHaveBeenCalledOnce();

    fireEvent.click(
      screen.getByRole("button", { name: "Weitere Kontoaktionen" }),
    );
    fireEvent.click(
      screen.getByRole("menuitem", {
        name: "Zwei-Faktor-Authentifizierung verwalten",
      }),
    );
    expect(onManageMFA).toHaveBeenCalledOnce();
  });
});
