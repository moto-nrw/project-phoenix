// #6533: Schließen während des Meldens ist gesperrt, eine späte Antwort
// zeigt ihren Erfolg nicht in einem neu geöffneten Dialog. Echter FormModal.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import {
  deferred,
  expectDialogUnlocked,
  expectSaveLocksDialog,
} from "~/test/helpers/form-modal-pending";

const mocks = vi.hoisted(() => ({ createAbsence: vi.fn() }));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn() }),
}));
vi.mock("~/lib/staff-api", () => ({
  staffAbsenceService: { createAbsence: mocks.createAbsence },
}));

import { SickReportModal } from "./sick-report-modal";

const staff = { id: "7", firstName: "Ada", lastName: "Lovelace" };
const TITLE = "Krank melden: Ada Lovelace";

function renderAndReport() {
  const save = deferred<unknown>();
  mocks.createAbsence.mockReturnValue(save.promise);
  const onClose = vi.fn();
  const onCreated = vi.fn();
  const modal = (isOpen: boolean) => (
    <SickReportModal
      isOpen={isOpen}
      staff={staff}
      onClose={onClose}
      onCreated={onCreated}
    />
  );
  const view = render(modal(true));
  fireEvent.click(screen.getByRole("button", { name: "Krank melden" }));
  return { ...view, modal, save, onClose, onCreated };
}

describe("SickReportModal pending lock (#6533)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("keeps the dialog shut while reporting and unlocks it after a failure", async () => {
    const { save, onClose } = renderAndReport();

    expectSaveLocksDialog(TITLE);

    await act(async () => {
      save.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expectDialogUnlocked(TITLE);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("shows a late success not in the dialog opened again meanwhile", async () => {
    const { save, modal, rerender, onCreated } = renderAndReport();
    expect(mocks.createAbsence).toHaveBeenCalledOnce();

    rerender(modal(false));
    rerender(modal(true));
    await act(async () => {
      save.resolve({ id: 42 });
    });

    // Die Daten frischt die Antwort trotzdem auf.
    expect(onCreated).toHaveBeenCalledOnce();
    expect(
      screen.getByRole("button", { name: "Krank melden" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Zur Vertretung" }),
    ).not.toBeInTheDocument();
  });
});
