// #6533: Schließen während des Speicherns ist gesperrt, eine späte Antwort
// schließt keinen anderen Dialog. Mit dem echten FormModal.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { unavailableApiError } from "~/lib/api-error";
import {
  deferred,
  expectDialogUnlocked,
  expectSaveLocksDialog,
} from "~/test/helpers/form-modal-pending";

vi.mock("~/components/ui/date-picker", async () =>
  (await import("~/test/mocks/date-picker")).datePickerModuleMock(),
);

const { mockCreate } = vi.hoisted(() => ({ mockCreate: vi.fn() }));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: { bulkCancel: vi.fn() },
}));
vi.mock("~/lib/closing-day-api", () => ({
  closingDayService: { create: mockCreate, update: vi.fn() },
}));
vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), info: vi.fn(), warn: vi.fn() }),
}));

import { ClosingDayModal } from "./closing-day-modal";

function renderModal() {
  const onClose = vi.fn();
  const onSaved = vi.fn();
  const view = render(
    <ClosingDayModal
      isOpen
      onClose={onClose}
      onSaved={onSaved}
      initial={null}
    />,
  );
  fireEvent.change(screen.getByLabelText("Grund"), {
    target: { value: "Pädagogischer Tag" },
  });
  fireEvent.change(screen.getByLabelText("Von"), {
    target: { value: "2026-09-14" },
  });
  fireEvent.change(screen.getByLabelText("Bis"), {
    target: { value: "2026-09-14" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
  return { ...view, onClose, onSaved };
}

describe("ClosingDayModal pending lock (#6533)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("keeps the dialog shut against X, Escape, backdrop and Abbrechen while saving", async () => {
    const save = deferred();
    mockCreate.mockReturnValue(save.promise);
    const { onClose } = renderModal();

    expectSaveLocksDialog("Schließtag anlegen");

    await act(async () => {
      save.reject(unavailableApiError());
    });
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expectDialogUnlocked("Schließtag anlegen");
    expect(screen.getByLabelText("Grund")).toHaveValue("Pädagogischer Tag");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("refreshes the data but closes nothing when the answer arrives after unmount", async () => {
    const save = deferred();
    mockCreate.mockReturnValue(save.promise);
    const { onClose, onSaved, unmount } = renderModal();
    expect(mockCreate).toHaveBeenCalledOnce();

    unmount();
    await act(async () => {
      save.resolve();
    });

    expect(onSaved).toHaveBeenCalledOnce();
    expect(onClose).not.toHaveBeenCalled();
  });
});
