import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { setTestClock } from "~/test/clock";
import { StaffExportButton } from "./staff-export-button";

const exportStaffTimeTracking = vi.hoisted(() => vi.fn());

vi.mock("~/lib/time-tracking-export-api", () => ({ exportStaffTimeTracking }));
// Der Export-Dialog ist ein SlideOver; Vaul rendert in jsdom nichts.
vi.mock("vaul", () => import("~/test/mocks/vaul"));

function renderButton(yearStart = new Date(2026, 0, 1)) {
  render(
    <ToastProvider>
      <StaffExportButton staffId="7" yearStart={yearStart} />
    </ToastProvider>,
  );
  fireEvent.click(screen.getByRole("button", { name: "Menü öffnen" }));
  fireEvent.click(screen.getByRole("menuitem", { name: "Exportieren" }));
}

describe("StaffExportButton", () => {
  beforeEach(() => {
    exportStaffTimeTracking.mockReset();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("exports the time tracking of the given staff member", async () => {
    exportStaffTimeTracking.mockResolvedValue(undefined);
    renderButton();

    expect(
      screen.getByRole("heading", { name: "Zeitnachweis exportieren" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Excel" }));
    fireEvent.click(screen.getByRole("button", { name: "Herunterladen" }));

    await waitFor(() => expect(exportStaffTimeTracking).toHaveBeenCalled());
    // Default: die letzten 30 Tage bis heute (Testuhr 09.09.2026).
    expect(exportStaffTimeTracking).toHaveBeenCalledWith(
      "7",
      "2026-08-11",
      "2026-09-09",
      "xlsx",
    );
    // Der Dialog schließt, sobald die Datei da ist.
    await waitFor(() =>
      expect(
        screen.queryByRole("heading", { name: "Zeitnachweis exportieren" }),
      ).not.toBeInTheDocument(),
    );
  });

  it("keeps the selected range when the parent recreates the same year start", async () => {
    exportStaffTimeTracking.mockResolvedValue(undefined);
    const view = () => (
      <ToastProvider>
        <StaffExportButton staffId="7" yearStart={new Date(2026, 0, 1)} />
      </ToastProvider>
    );
    const { rerender } = render(view());
    fireEvent.click(screen.getByRole("button", { name: "Menü öffnen" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Exportieren" }));
    fireEvent.click(screen.getByRole("button", { name: "Letzte 7 Tage" }));

    expect(
      screen.getByText("Arbeitszeiten vom 03.09.2026 bis 09.09.2026."),
    ).toBeInTheDocument();

    rerender(view());

    expect(
      screen.getByRole("heading", { name: "Zeitnachweis exportieren" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Herunterladen" }));
    await waitFor(() =>
      expect(exportStaffTimeTracking).toHaveBeenCalledWith(
        "7",
        "2026-09-03",
        "2026-09-09",
        "pdf",
      ),
    );
  });

  it("starts at the beginning of the year when fewer than 30 days have passed", async () => {
    setTestClock("2026-01-12T12:00:00+01:00");
    exportStaffTimeTracking.mockResolvedValue(undefined);
    renderButton(new Date(2026, 0, 1));

    fireEvent.click(screen.getByRole("button", { name: "Herunterladen" }));

    await waitFor(() =>
      expect(exportStaffTimeTracking).toHaveBeenCalledWith(
        "7",
        "2026-01-01",
        "2026-01-12",
        "pdf",
      ),
    );
  });

  it("prints the PDF for the Berlin range in the tab opened from the click", async () => {
    const target = {} as Window;
    vi.spyOn(globalThis, "open").mockReturnValue(target);
    exportStaffTimeTracking.mockResolvedValue(undefined);
    renderButton();

    fireEvent.click(screen.getByRole("button", { name: "Drucken" }));

    await waitFor(() =>
      expect(exportStaffTimeTracking).toHaveBeenCalledWith(
        "7",
        "2026-08-11",
        "2026-09-09",
        "pdf",
        target,
      ),
    );
  });

  it("keeps a failed export in the dialog with retry", async () => {
    exportStaffTimeTracking
      .mockRejectedValueOnce(
        new ApiError("boom", 500, { code: "general.server", instance: "r1" }),
      )
      .mockResolvedValueOnce(undefined);
    renderButton();

    fireEvent.click(screen.getByRole("button", { name: "Herunterladen" }));

    const message = catalogText("general.server", "den Zeitnachweis");
    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toContainElement(
      screen.getByText(message),
    );

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() =>
      expect(exportStaffTimeTracking).toHaveBeenCalledTimes(2),
    );
  });
});
