// #6593: Während die Gruppe übergeben wird, ist das Panel gesperrt, auch
// „Zurücknehmen“, dessen Rückfrage das Panel ersetzen würde. Eine späte
// Antwort zeigt keinen Fehler im wieder geöffneten Panel. Echter SlideOver
// über dem gemeinsamen Vaul-Mock.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

import { GroupTransferModal } from "./group-transfer-modal";

const TITLE = 'Gruppe "Gruppe A" übergeben';
const FOOTER = ["Abbrechen", /Wird übergeben/];
const TRANSFER_ERROR = catalogText("general.server", "die Übergabe");

const group = { id: "1", name: "Gruppe A", studentCount: 15 };
const users = [
  { id: "1", fullName: "John Doe" },
  { id: "2", fullName: "Jane Smith" },
];
const transfers = [
  { targetName: "Mike Johnson", substitutionId: "s1", targetStaffId: "9" },
];

function modal(
  isOpen: boolean,
  onTransfer: (id: string, name: string) => Promise<void>,
  onClose: () => void,
) {
  return (
    <GroupTransferModal
      isOpen={isOpen}
      onClose={onClose}
      group={group}
      availableUsers={users}
      onTransfer={onTransfer}
      existingTransfers={transfers}
      onCancelTransfer={vi.fn().mockResolvedValue(undefined)}
    />
  );
}

async function renderAndTransfer() {
  const answer = deferred();
  const onTransfer = vi.fn(() => answer.promise);
  const onClose = vi.fn();
  const view = render(modal(true, onTransfer, onClose));
  fireEvent.click(await screen.findByRole("combobox"));
  fireEvent.click(screen.getByRole("option", { name: "John Doe" }));
  const transfer = screen.getByRole("button", { name: "Übergeben" });
  expect(transfer).toBeEnabled();
  fireEvent.click(transfer);
  return { ...view, answer, onTransfer, onClose };
}

describe("GroupTransferModal pending lock (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("locks the panel while transferring and unlocks it after a failure", async () => {
    const { answer, onClose, onTransfer } = await renderAndTransfer();

    expectSlideOverSaveLocked(TITLE, FOOTER);
    // Die Rückfrage zum Zurücknehmen ersetzt das Panel und darf es so nicht
    // entsperren.
    expect(screen.getByRole("button", { name: "Zurücknehmen" })).toBeDisabled();
    expect(onClose).not.toHaveBeenCalled();

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });

    expect(await screen.findByText(TRANSFER_ERROR)).toBeInTheDocument();
    expectSlideOverUnlocked(TITLE);
    // Die Auswahl bleibt für „Wiederholen“ stehen.
    expect(screen.getByRole("combobox")).toHaveTextContent("John Doe");
    expect(screen.getByRole("button", { name: "Zurücknehmen" })).toBeEnabled();
    expect(onTransfer).toHaveBeenCalledWith("1", "John Doe");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("clears the choice after a successful transfer", async () => {
    const { answer } = await renderAndTransfer();

    await act(async () => {
      answer.resolve();
    });

    expectSlideOverUnlocked(TITLE);
    expect(screen.getByRole("combobox")).toHaveTextContent(
      "Fachkraft auswählen...",
    );
  });

  it("shows no late failure in the panel opened again", async () => {
    const { answer, onTransfer, onClose, rerender } = await renderAndTransfer();

    rerender(modal(false, onTransfer, onClose));
    rerender(modal(true, onTransfer, onClose));
    expectSlideOverUnlocked(TITLE);

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });

    // Anker: das neue Panel steht und ist bedienbar.
    expect(screen.getByRole("heading", { name: TITLE })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeEnabled();
    expect(screen.queryByText(TRANSFER_ERROR)).not.toBeInTheDocument();
  });
});
