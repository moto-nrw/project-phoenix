// #6533: Schließen während des Umbenennens ist gesperrt, eine späte Antwort
// meldet sich nicht im Dialog einer anderen Vorlage. Echter FormModal.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeAll, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { FormSchema } from "~/lib/enrollment-form-schema-api";
import {
  deferred,
  expectDialogUnlocked,
  expectSaveLocksDialog,
} from "~/test/helpers/form-modal-pending";

import { RenameSchemaDialog } from "./enrollment-form-overview";

const TITLE = "Formular umbenennen";

function schema(id: string, name: string): FormSchema {
  return {
    id,
    name,
    version: 1,
    is_active: true,
    fields: [],
    created_by: "1",
    created_at: "2026-09-01T10:00:00Z",
  };
}

function renderAndRename() {
  const answer = deferred();
  const onConfirm = vi.fn(() => answer.promise);
  const onClose = vi.fn();
  const dialog = (shown: FormSchema) => (
    <RenameSchemaDialog
      schema={shown}
      onClose={onClose}
      onConfirm={onConfirm}
    />
  );
  const view = render(dialog(schema("1", "Sommer")));
  fireEvent.change(screen.getByLabelText("Name"), {
    target: { value: "Sommerferien" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
  return { ...view, dialog, answer, onConfirm, onClose };
}

describe("RenameSchemaDialog pending lock (#6533)", () => {
  // Der Fehlerweg lädt seine Texte nach; vorab geladen erscheint ein Fehler
  // im selben act, sonst bewiese ein fehlender Fehler nichts.
  beforeAll(async () => {
    await import("~/lib/error-presentation");
  });

  it("keeps the dialog shut while renaming and unlocks it after a failure", async () => {
    const { answer, onClose } = renderAndRename();

    expectSaveLocksDialog(TITLE);

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expectDialogUnlocked(TITLE);
    expect(screen.getByLabelText("Name")).toHaveValue("Sommerferien");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("shows a late failure not in the dialog of another template", async () => {
    const { answer, dialog, rerender, onConfirm } = renderAndRename();
    expect(onConfirm).toHaveBeenCalledWith("Sommerferien");

    rerender(dialog(schema("2", "Herbst")));
    // Anker: der Dialog zeigt die andere Vorlage, frei bedienbar.
    expect(screen.getByLabelText("Name")).toHaveValue("Herbst");
    expectDialogUnlocked(TITLE);

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
      await answer.promise.catch(() => undefined);
    });

    expect(screen.getByLabelText("Name")).toHaveValue("Herbst");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
