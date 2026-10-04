import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { StaffBirthdayExportModal } from "./staff-birthday-export-modal";

const exportStaffBirthdays = vi.hoisted(() => vi.fn());

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

  it("schließt nach einem Export und meldet den Erfolg", async () => {
    exportStaffBirthdays.mockResolvedValue(undefined);
    const onClose = renderModal();

    fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));

    expect(
      await screen.findByText("Die Geburtstagsliste ist erstellt."),
    ).toBeInTheDocument();
    expect(onClose).toHaveBeenCalled();
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

    fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));

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
