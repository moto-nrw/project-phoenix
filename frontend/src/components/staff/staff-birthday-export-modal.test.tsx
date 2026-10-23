import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { StaffBirthdayExportModal } from "./staff-birthday-export-modal";

const exportStaffBirthdays = vi.hoisted(() => vi.fn());

// Der Dialog ist ein SlideOver; Vaul rendert in jsdom nichts.
vi.mock("vaul", () => import("~/test/mocks/vaul"));

vi.mock("~/lib/birthdays-api", () => ({ exportStaffBirthdays }));

function renderModal(onClose = vi.fn()) {
  render(
    <ToastProvider>
      <StaffBirthdayExportModal isOpen onClose={onClose} />
    </ToastProvider>,
  );
  return onClose;
}

describe("StaffBirthdayExportModal", () => {
  beforeEach(() => {
    exportStaffBirthdays.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("schließt nach einem Export und meldet den Erfolg", async () => {
    exportStaffBirthdays.mockResolvedValue(undefined);
    const onClose = renderModal();

    fireEvent.click(screen.getByRole("button", { name: "Herunterladen" }));

    expect(
      await screen.findByText("Die Geburtstagsliste ist erstellt."),
    ).toBeInTheDocument();
    expect(onClose).toHaveBeenCalled();
  });

  it("druckt die PDF im vor dem Laden geöffneten Tab", async () => {
    const target = {} as Window;
    vi.spyOn(globalThis, "open").mockReturnValue(target);
    exportStaffBirthdays.mockResolvedValue(undefined);
    renderModal();

    fireEvent.click(screen.getByRole("button", { name: "Drucken" }));

    await waitFor(() =>
      expect(exportStaffBirthdays).toHaveBeenCalledWith(
        expect.objectContaining({ format: "pdf" }),
        target,
      ),
    );
  });

  it("zeigt einen Fehler im offenen Dialog und wiederholt mit der aktuellen Auswahl", async () => {
    exportStaffBirthdays.mockRejectedValueOnce(
      new ApiError("boom", 500, {
        code: "general.server",
        instance: "req-birthdays",
      }),
    );
    exportStaffBirthdays.mockResolvedValueOnce(undefined);
    const onClose = renderModal();

    fireEvent.click(screen.getByRole("button", { name: "Herunterladen" }));

    const message = catalogText("general.server", "die Geburtstagsliste");
    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toContainElement(
      screen.getByText(message),
    );
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Titel"), {
      target: { value: "Neue Liste" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    await waitFor(() => expect(exportStaffBirthdays).toHaveBeenCalledTimes(2));
    expect(exportStaffBirthdays).toHaveBeenLastCalledWith(
      expect.objectContaining({ title: "Neue Liste" }),
    );
  });
});
