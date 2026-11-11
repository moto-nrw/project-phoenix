// #6533: Schließen während des Anlegens ist gesperrt, eine späte Antwort
// schließt keinen anderen Dialog. Echter FormModal.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  deferred,
  expectDialogUnlocked,
  expectSaveLocksDialog,
} from "~/test/helpers/form-modal-pending";

// Ein stabiler Rückgabewert wie beim echten Hook (useState).
const { activityForm } = vi.hoisted(() => ({
  activityForm: {
    form: { name: "Fußball", category_id: "1", max_participants: "15" },
    setForm: () => undefined,
    categories: [{ id: "1", name: "Sport" }],
    loading: false,
    loadError: null,
    handleInputChange: () => undefined,
    validateForm: () => null,
  },
}));
vi.mock("~/hooks/useActivityForm", () => ({
  parseParticipantLimit: (value: string) =>
    value ? Number.parseInt(value, 10) : null,
  useActivityForm: () => activityForm,
}));
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: vi.fn(() => ({ success: vi.fn(), error: vi.fn() })),
}));

import { QuickCreateActivityModal } from "./quick-create-modal";

const TITLE = "Aktivität erstellen";

function renderAndCreate() {
  const answer = deferred<Response>();
  vi.stubGlobal(
    "fetch",
    vi.fn(() => answer.promise),
  );
  const onClose = vi.fn();
  const onSuccess = vi.fn();
  const view = render(
    <QuickCreateActivityModal isOpen onClose={onClose} onSuccess={onSuccess} />,
  );
  fireEvent.click(screen.getByRole("button", { name: "Aktivität erstellen" }));
  return { ...view, answer, onClose, onSuccess };
}

describe("QuickCreateActivityModal pending lock (#6533)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("keeps the dialog shut while creating and unlocks it after a failure", async () => {
    const { answer, onClose } = renderAndCreate();

    expectSaveLocksDialog(TITLE);

    await act(async () => {
      answer.resolve(
        Response.json(
          { status: 500, code: "general.server", detail: "kaputt" },
          { status: 500 },
        ),
      );
    });
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expectDialogUnlocked(TITLE);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("refreshes the data but closes nothing when the answer arrives after unmount", async () => {
    const { answer, onClose, onSuccess, unmount } = renderAndCreate();
    expect(fetch).toHaveBeenCalledOnce();

    unmount();
    await act(async () => {
      answer.resolve(Response.json({ data: { id: 1 } }));
    });

    expect(onSuccess).toHaveBeenCalledOnce();
    expect(onClose).not.toHaveBeenCalled();
  });
});
