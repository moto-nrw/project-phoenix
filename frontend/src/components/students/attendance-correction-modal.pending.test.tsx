// #6533: Schließen während der Korrektur ist gesperrt, eine späte Antwort
// schließt keinen anderen Dialog. Echter FormModal.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  deferred,
  expectDialogUnlocked,
  expectSaveLocksDialog,
} from "~/test/helpers/form-modal-pending";

import { AttendanceCorrectionModal } from "./attendance-correction-modal";

vi.mock("~/lib/session-cache", () => ({
  sessionFetch: (url: string, init?: RequestInit) => fetch(url, init),
}));

const slot = {
  instanceId: "instance-1",
  title: "Bastelstunde",
  date: "2026-09-07",
  startTime: "14:00",
  endTime: "15:00",
  status: "present",
  substatus: null,
  note: null,
};

function renderAndSave() {
  const answer = deferred<Response>();
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_url: string, init?: RequestInit) =>
      init?.method === "POST"
        ? answer.promise
        : Response.json({ data: { corrections: [] } }),
    ),
  );
  const onClose = vi.fn();
  const onCorrected = vi.fn();
  const view = render(
    <AttendanceCorrectionModal
      isOpen
      onClose={onClose}
      studentId="student-1"
      slot={slot}
      onCorrected={onCorrected}
    />,
  );
  fireEvent.change(screen.getByLabelText(/Bemerkung/), {
    target: { value: "kam später" },
  });
  fireEvent.change(screen.getByLabelText(/Grund der Korrektur/), {
    target: { value: "Nachtrag" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
  return { ...view, answer, onClose, onCorrected };
}

describe("AttendanceCorrectionModal pending lock (#6533)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("keeps the dialog shut while saving and unlocks it after a failure", async () => {
    const { answer, onClose } = renderAndSave();

    expectSaveLocksDialog("Eintrag korrigieren");

    await act(async () => {
      answer.resolve(
        Response.json(
          { status: 500, code: "general.server", detail: "kaputt" },
          { status: 500 },
        ),
      );
    });
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expectDialogUnlocked("Eintrag korrigieren");
    expect(screen.getByLabelText(/Grund der Korrektur/)).toHaveValue(
      "Nachtrag",
    );
    expect(onClose).not.toHaveBeenCalled();
  });

  it("refreshes the data but closes nothing when the answer arrives after unmount", async () => {
    const { answer, onClose, onCorrected, unmount } = renderAndSave();
    expect(fetch).toHaveBeenCalledWith(
      expect.stringContaining("/correction"),
      expect.objectContaining({ method: "POST" }),
    );

    unmount();
    await act(async () => {
      answer.resolve(Response.json({ data: {} }));
    });

    expect(onCorrected).toHaveBeenCalledOnce();
    expect(onClose).not.toHaveBeenCalled();
  });
});
