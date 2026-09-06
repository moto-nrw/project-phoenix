import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { StaffTimeExportModal } from "./staff-time-export-modal";
import {
  fetchDatevExportReport,
  DatevConfigIncompleteError,
  type DatevExportReport,
} from "~/lib/datev-export-api";
import {
  fetchSFTPStatus,
  transferExportViaSFTP,
  type SFTPStatus,
} from "~/lib/sftp-export-api";

vi.mock("~/lib/datev-export-api", async (importOriginal) => {
  const original =
    await importOriginal<typeof import("~/lib/datev-export-api")>();
  return { ...original, fetchDatevExportReport: vi.fn() };
});

vi.mock("~/lib/sftp-export-api", async (importOriginal) => {
  const original =
    await importOriginal<typeof import("~/lib/sftp-export-api")>();
  return {
    ...original,
    fetchSFTPStatus: vi.fn(),
    transferExportViaSFTP: vi.fn(),
  };
});

const fetchReportMock = vi.mocked(fetchDatevExportReport);
const fetchSFTPStatusMock = vi.mocked(fetchSFTPStatus);
const transferMock = vi.mocked(transferExportViaSFTP);

const notConfiguredStatus: SFTPStatus = {
  enabled: false,
  ready: false,
  missingSettings: [],
};

const readyStatus: SFTPStatus = {
  enabled: true,
  ready: true,
  host: "dateien.beispiel.de",
  remoteDirectory: "/upload/lohn",
  missingSettings: [],
};

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
    // Standard: keine Gegenstelle eingerichtet — der ehrliche Ausgangszustand
    // jeder Schule, die die Übertragung nicht nutzt.
    fetchSFTPStatusMock.mockResolvedValue(notConfiguredStatus);
    transferMock.mockReset();
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

  it("blocks DATEV downloads when the report contains staff without personnel numbers", async () => {
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
        screen.getByText(/Export gesperrt: Personalnummer fehlt/),
      ).toBeInTheDocument(),
    );
    expect(screen.getByText("Sicht, Ueber")).toBeInTheDocument();
    expect(screen.getByText(/Fortbildung/)).toBeInTheDocument();
    expect(screen.getByText(/noch nicht abgeschlossen/)).toBeInTheDocument();
    const exportButton = screen.getByRole("button", { name: "Exportieren" });
    expect(exportButton).toBeDisabled();
    fireEvent.click(exportButton);
    expect(globalThis.location.href).toBe("");
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

  it("requires a successful report for the currently selected DATEV format", async () => {
    let resolveSecondReport: (report: DatevExportReport) => void = () => {
      throw new Error("second report resolver was not initialized");
    };
    fetchReportMock.mockResolvedValueOnce(emptyReport).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveSecondReport = (report) => resolve(report);
        }),
    );
    renderModal();
    fireEvent.click(screen.getByRole("button", { name: "DATEV LODAS" }));

    await waitFor(() =>
      expect(screen.getByText(/12 Buchungszeilen/)).toBeInTheDocument(),
    );
    expect(
      screen.getByRole("button", { name: "Exportieren" }),
    ).not.toBeDisabled();

    fireEvent.click(
      screen.getByRole("button", { name: "DATEV Lohn und Gehalt" }),
    );

    expect(screen.getByRole("button", { name: "Exportieren" })).toBeDisabled();
    expect(fetchReportMock).toHaveBeenLastCalledWith(2026, 6, "datev_lug");

    resolveSecondReport(emptyReport);
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "Exportieren" }),
      ).not.toBeDisabled(),
    );
  });

  it("keeps the DATEV download blocked after a report failure and allows retrying", async () => {
    fetchReportMock
      .mockRejectedValueOnce(new Error("report unavailable"))
      .mockResolvedValueOnce(emptyReport);
    renderModal();
    fireEvent.click(screen.getByRole("button", { name: "DATEV LODAS" }));

    await waitFor(() =>
      expect(
        screen.getByText(/Bericht konnte nicht geladen werden/),
      ).toBeInTheDocument(),
    );
    expect(screen.getByRole("button", { name: "Exportieren" })).toBeDisabled();
    expect(globalThis.location.href).toBe("");
    const callsBeforeRetry = fetchReportMock.mock.calls.length;

    fireEvent.click(
      screen.getByRole("button", { name: "Bericht erneut laden" }),
    );

    await waitFor(() =>
      expect(screen.getByText(/12 Buchungszeilen/)).toBeInTheDocument(),
    );
    expect(fetchReportMock).toHaveBeenCalledTimes(callsBeforeRetry + 1);
    expect(
      screen.getByRole("button", { name: "Exportieren" }),
    ).not.toBeDisabled();
  });

  // Übertragung an die Gegenstelle (#3050). Die Tests pinnen vor allem die
  // Verständlichkeit: eine Auswahl, die nichts tun kann, ist deaktiviert und
  // sagt warum — und ein Fehlschlag wird nie als Erfolg gezeigt.
  describe("Übertragung an die Gegenstelle", () => {
    it("zeigt die Auswahl gar nicht, solange die Schnittstelle ausgeschaltet ist", async () => {
      renderModal();

      // Erst warten, bis der Status da ist — sonst würde der Test auch
      // bestehen, weil noch gar nichts gerendert wurde.
      await waitFor(() => expect(fetchSFTPStatusMock).toHaveBeenCalled());

      expect(screen.queryByText("Wohin")).not.toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: "An die Gegenstelle übertragen" }),
      ).not.toBeInTheDocument();
      // Der Download bleibt der einzige Weg — und der einzige Knopf.
      expect(
        screen.getByRole("button", { name: "Exportieren" }),
      ).toBeInTheDocument();
    });

    // Eingeschaltet, aber unvollständig: Hier hat jemand die Übertragung
    // gewollt. Die Auswahl bleibt sichtbar und sagt, was fehlt — ein stiller
    // Rückfall auf den Download würde die halbfertige Einrichtung verbergen.
    it("zeigt die Auswahl gesperrt, wenn sie eingeschaltet aber unvollständig ist", async () => {
      fetchSFTPStatusMock.mockResolvedValue({
        enabled: true,
        ready: false,
        missingSettings: ["sftp.host", "sftp.password"],
      });
      renderModal();

      await waitFor(() =>
        expect(
          screen.getByText(/noch nicht vollständig eingerichtet/),
        ).toBeInTheDocument(),
      );
      expect(
        screen.getByRole("button", { name: "An die Gegenstelle übertragen" }),
      ).toBeDisabled();
      expect(
        screen.getByRole("button", { name: "Exportieren" }),
      ).toBeInTheDocument();
    });

    it("nennt das Ziel und überträgt dieselbe Auswahl wie der Download", async () => {
      fetchSFTPStatusMock.mockResolvedValue(readyStatus);
      transferMock.mockResolvedValue({
        transferred: true,
        filename: "zeitkonten-2026-06.csv",
        targetHost: "dateien.beispiel.de",
        targetDirectory: "/upload/lohn",
      });
      renderModal();

      const option = await screen.findByRole("button", {
        name: "An die Gegenstelle übertragen",
      });
      expect(option).not.toBeDisabled();
      fireEvent.click(option);

      expect(screen.getByText(/dateien\.beispiel\.de/)).toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Übertragen" }));

      await waitFor(() =>
        expect(transferMock).toHaveBeenCalledWith({
          year: 2026,
          month: 6,
          format: "csv",
          wholeYear: false,
          granularity: "month",
          timeFormat: "hhmm",
        }),
      );
      await waitFor(() =>
        expect(
          screen.getByText(/zeitkonten-2026-06\.csv wurde übertragen/),
        ).toBeInTheDocument(),
      );
      // Kein Download nebenher.
      expect(globalThis.location.href).toBe("");
    });

    it("zeigt einen Fehlschlag als Fehlschlag, obwohl die Antwort 200 ist", async () => {
      fetchSFTPStatusMock.mockResolvedValue(readyStatus);
      transferMock.mockResolvedValue({
        transferred: false,
        filename: "zeitkonten-2026-06.csv",
        reason: "host_key_mismatch",
      });
      renderModal();

      fireEvent.click(
        await screen.findByRole("button", {
          name: "An die Gegenstelle übertragen",
        }),
      );
      fireEvent.click(screen.getByRole("button", { name: "Übertragen" }));

      await waitFor(() =>
        expect(
          screen.getByText(/nicht sicher erkannt werden/),
        ).toBeInTheDocument(),
      );
      expect(screen.queryByText(/wurde übertragen/)).not.toBeInTheDocument();
    });
  });
});
