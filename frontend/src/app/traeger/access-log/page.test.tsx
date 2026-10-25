import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type {
  TraegerAccessLogEntry,
  TraegerAccessLogFilter,
} from "~/lib/traeger-api";

const {
  mockFetchSchools,
  mockFetchLog,
  mockFetchPeople,
  mockDownload,
  mockDownloadBlob,
} = vi.hoisted(() => ({
  mockFetchSchools: vi.fn(),
  mockFetchLog: vi.fn(),
  mockFetchPeople: vi.fn(),
  mockDownload: vi.fn(),
  mockDownloadBlob: vi.fn(),
}));

vi.mock("~/lib/traeger-api", () => ({
  fetchTraegerSchools: () => mockFetchSchools(),
  fetchTraegerAccessLog: (filter: TraegerAccessLogFilter, page: number) =>
    mockFetchLog(filter, page),
  fetchTraegerAccessLogPeople: (from: string, to: string) =>
    mockFetchPeople(from, to),
  downloadTraegerAccessLog: (filter: TraegerAccessLogFilter) =>
    mockDownload(filter),
}));
vi.mock("~/lib/file-download", () => ({
  downloadBlob: (blob: Blob, name: string) => mockDownloadBlob(blob, name),
}));

import TraegerAccessLogPage from "./page";

const READ: TraegerAccessLogEntry = {
  id: "read-attendance_history-7",
  occurredAt: "2026-09-08T07:30:00Z",
  schoolId: "2",
  schoolName: "OGS Am Bach",
  accountId: "11",
  person: "Frieda Fachkraft",
  kind: "read",
  dataLabel: "Anwesenheit eines Kindes",
};

const CHANGE: TraegerAccessLogEntry = {
  id: "change-staff_record-3",
  occurredAt: "2026-09-07T12:00:00Z",
  schoolId: "3",
  schoolName: "OGS Waldweg",
  accountId: "12",
  person: "Lena Leitung",
  kind: "change",
  dataLabel: "Angaben zu einer Fachkraft",
};

function renderPage() {
  return render(
    <ToastProvider>
      <TraegerAccessLogPage />
    </ToastProvider>,
  );
}

describe("TraegerAccessLogPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetchSchools.mockResolvedValue([
      { id: "2", name: "OGS Am Bach", slug: "a", subdomain: "a", city: "" },
      { id: "3", name: "OGS Waldweg", slug: "w", subdomain: "w", city: "" },
    ]);
    mockFetchPeople.mockResolvedValue([
      { accountId: "11", name: "Frieda Fachkraft" },
      { accountId: "12", name: "Lena Leitung" },
    ]);
    mockFetchLog.mockResolvedValue({ entries: [READ, CHANGE], hasMore: false });
  });

  it("shows the last 30 days as a read-only table in Berlin time", async () => {
    renderPage();

    const table = await screen.findByRole("table");
    expect(within(table).getByText("Frieda Fachkraft")).toBeInTheDocument();
    expect(
      within(table).getByText("08.09.2026, 09:30 Uhr"),
    ).toBeInTheDocument();
    expect(
      within(table).getByText("Anwesenheit eines Kindes"),
    ).toBeInTheDocument();
    expect(within(table).getByText("Geändert")).toBeInTheDocument();
    // The test clock stands on 2026-09-09.
    expect(mockFetchLog).toHaveBeenCalledWith(
      {
        from: "2026-08-11",
        to: "2026-09-09",
        schoolId: "",
        accountId: "",
        kind: "",
      },
      1,
    );
    expect(mockFetchPeople).toHaveBeenCalledWith("2026-08-11", "2026-09-09");
    // Anker: die Zeilen sind geladen; sie sind reine Anzeige.
    expect(within(table).getByText("Lena Leitung")).toBeInTheDocument();
    expect(within(table).queryAllByRole("button", { name: /Leitung/ })).toEqual(
      [],
    );
  });

  it("filters by the kind of access", async () => {
    renderPage();
    await screen.findByRole("table");

    fireEvent.click(screen.getByRole("button", { name: "Geändert" }));

    await waitFor(() =>
      expect(mockFetchLog).toHaveBeenLastCalledWith(
        expect.objectContaining({ kind: "change" }),
        1,
      ),
    );
  });

  it("loads the next page below the list", async () => {
    mockFetchLog
      .mockResolvedValueOnce({ entries: [READ], hasMore: true })
      .mockResolvedValueOnce({ entries: [CHANGE], hasMore: false });
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", { name: "Weitere Einträge laden" }),
    );

    expect((await screen.findAllByText("Lena Leitung"))[0]).toBeInTheDocument();
    expect(mockFetchLog).toHaveBeenLastCalledWith(expect.anything(), 2);
    expect(screen.getAllByText("Frieda Fachkraft")[0]).toBeInTheDocument();
  });

  it("downloads the CSV of the filter", async () => {
    const blob = new Blob(["Datum"]);
    mockDownload.mockResolvedValue({ blob, filename: "zugriffsprotokoll.csv" });
    renderPage();
    await screen.findByRole("table");

    fireEvent.click(
      screen.getByRole("button", { name: "Als CSV herunterladen" }),
    );

    await waitFor(() =>
      expect(mockDownloadBlob).toHaveBeenCalledWith(
        blob,
        "zugriffsprotokoll.csv",
      ),
    );
    expect(mockDownload).toHaveBeenCalledWith(
      expect.objectContaining({ from: "2026-08-11", to: "2026-09-09" }),
    );
  });

  it("says how to narrow an export with too many entries", async () => {
    mockDownload.mockRejectedValue(
      new ApiError("too large", 400, {
        code: "traeger.access_log_export_too_large",
        details: { limit: 10000 },
      }),
    );
    renderPage();
    await screen.findByRole("table");

    fireEvent.click(
      screen.getByRole("button", { name: "Als CSV herunterladen" }),
    );

    expect(
      await screen.findByText(/Der Export hat mehr als 10000 Einträge/),
    ).toBeInTheDocument();
    expect(mockDownloadBlob).not.toHaveBeenCalled();
  });
});
