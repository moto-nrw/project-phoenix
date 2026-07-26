import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { StaffTimeExportModal } from "./staff-time-export-modal";
import {
  fetchDatevExportReport,
  DatevConfigIncompleteError,
  type DatevExportReport,
} from "~/lib/datev-export-api";

vi.mock("~/lib/datev-export-api", async (importOriginal) => {
  const original =
    await importOriginal<typeof import("~/lib/datev-export-api")>();
  return { ...original, fetchDatevExportReport: vi.fn() };
});

const fetchReportMock = vi.mocked(fetchDatevExportReport);

const emptyReport: DatevExportReport = {
  lineCount: 12,
  staffExported: 3,
  staffSkipped: [],
  unconfiguredCategories: [],
  openMonth: false,
};

// Der Dialog baut nur die Query — die Tests pinnen genau das: welche Parameter
// bei welcher Auswahl an die Streaming-Proxy-Route gehen (#1417 2b).
describe("StaffTimeExportModal", () => {
  const originalLocation = globalThis.location;

  beforeEach(() => {
    Object.defineProperty(globalThis, "location", {
      value: { href: "" },
      writable: true,
    });
  });

  afterEach(() => {
    globalThis.location = originalLocation;
  });

  function renderModal(onClose = () => undefined) {
    return render(
      <StaffTimeExportModal
        isOpen={true}
        onClose={onClose}
        year={2026}
        month={6}
      />,
    );
  }

  it("exports the displayed month as CSV with hh:mm by default", () => {
    renderModal();
    fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));

    expect(globalThis.location.href).toBe(
      "/api/staff/time-tracking/export?year=2026&format=csv&month=6&granularity=month&time_format=hhmm",
    );
  });

  it("exports the whole year in decimal Excel when selected", () => {
    renderModal();
    fireEvent.click(screen.getByRole("button", { name: "Gesamtes Jahr 2026" }));
    fireEvent.click(screen.getByRole("button", { name: "Excel" }));
    fireEvent.click(screen.getByRole("button", { name: "Dezimalstunden" }));
    fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));

    expect(globalThis.location.href).toBe(
      "/api/staff/time-tracking/export?year=2026&format=xlsx&granularity=month&time_format=decimal",
    );
  });

  it("drops time_format and disables the toggle for the day granularity", () => {
    renderModal();
    fireEvent.click(screen.getByRole("button", { name: "Einzelne Tage" }));

    expect(
      screen.getByRole("button", { name: "Dezimalstunden" }),
    ).toBeDisabled();
    expect(
      screen.getByText(/Zeitformat gilt nur für Monatssummen/),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));
    expect(globalThis.location.href).toBe(
      "/api/staff/time-tracking/export?year=2026&format=csv&month=6&granularity=day",
    );
  });

  it("closes after triggering the download", () => {
    let closed = false;
    renderModal(() => {
      closed = true;
    });
    fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));
    expect(closed).toBe(true);
  });

  it("exports a single DATEV month without granularity or time_format", async () => {
    fetchReportMock.mockResolvedValue(emptyReport);
    renderModal();
    fireEvent.click(screen.getByRole("button", { name: "DATEV LODAS" }));

    await waitFor(() =>
      expect(screen.getByText(/12 Buchungszeilen/)).toBeInTheDocument(),
    );
    // Zeitraum und Detailgrad sind bei DATEV fixiert.
    expect(
      screen.getByRole("button", { name: "Gesamtes Jahr 2026" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Einzelne Tage" }),
    ).toBeDisabled();
    expect(fetchReportMock).toHaveBeenCalledWith(2026, 6, "datev_lodas");

    fireEvent.click(screen.getByRole("button", { name: "Exportieren" }));
    expect(globalThis.location.href).toBe(
      "/api/staff/time-tracking/export?year=2026&format=datev_lodas&month=6",
    );
  });

  it("shows the skip report and the open-month warning before a DATEV download", async () => {
    fetchReportMock.mockResolvedValue({
      lineCount: 4,
      staffExported: 1,
      staffSkipped: [
        {
          lastName: "Sicht",
          firstName: "Ueber",
          reason: "keine Personalnummer",
        },
      ],
      unconfiguredCategories: ["Fortbildung"],
      openMonth: true,
    });
    renderModal();
    fireEvent.click(
      screen.getByRole("button", { name: "DATEV Lohn und Gehalt" }),
    );

    await waitFor(() =>
      expect(
        screen.getByText(/Ohne Personalnummer übersprungen/),
      ).toBeInTheDocument(),
    );
    expect(screen.getByText("Sicht, Ueber")).toBeInTheDocument();
    expect(screen.getByText(/Fortbildung/)).toBeInTheDocument();
    expect(screen.getByText(/noch nicht abgeschlossen/)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Exportieren" }),
    ).not.toBeDisabled();
  });

  it("blocks the DATEV download while the payroll configuration is incomplete", async () => {
    fetchReportMock.mockRejectedValue(new DatevConfigIncompleteError());
    renderModal();
    fireEvent.click(screen.getByRole("button", { name: "DATEV LODAS" }));

    await waitFor(() =>
      expect(
        screen.getByText(/Konfiguration ist unvollständig/),
      ).toBeInTheDocument(),
    );
    expect(screen.getByRole("button", { name: "Exportieren" })).toBeDisabled();
  });
});
