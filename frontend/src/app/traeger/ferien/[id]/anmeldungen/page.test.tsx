import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { HolidayRegistrationFilter } from "~/lib/traeger-holiday-registrations-api";
import {
  HOLIDAY_PERIOD,
  HOLIDAY_REGISTRATION_LIST,
  HOLIDAY_SCHOOLS,
} from "~/test/fixtures/holiday-registrations";

const { mockPush, mockFetchList, mockDownload, mockDownloadBlob } = vi.hoisted(
  () => ({
    mockPush: vi.fn(),
    mockFetchList: vi.fn(),
    mockDownload: vi.fn(),
    mockDownloadBlob: vi.fn(),
  }),
);

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
  useParams: () => ({ id: "5" }),
}));
vi.mock("~/lib/traeger-api", () => ({
  fetchHolidayPeriod: () => Promise.resolve(HOLIDAY_PERIOD),
  fetchTraegerSchools: () => Promise.resolve(HOLIDAY_SCHOOLS),
}));
vi.mock("~/lib/traeger-holiday-registrations-api", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("~/lib/traeger-holiday-registrations-api")
  >()),
  fetchHolidayRegistrations: (
    periodId: string,
    filter: HolidayRegistrationFilter,
  ) => mockFetchList(periodId, filter),
  downloadHolidayRegistrations: (
    periodId: string,
    filter: HolidayRegistrationFilter,
    format: string,
  ) => mockDownload(periodId, filter, format),
}));
vi.mock("~/lib/file-download", () => ({
  downloadBlob: (blob: Blob, name: string) => mockDownloadBlob(blob, name),
}));

import TraegerHolidayRegistrationsPage from "./page";

function renderPage() {
  return render(
    <ToastProvider>
      <TraegerHolidayRegistrationsPage />
    </ToastProvider>,
  );
}

describe("TraegerHolidayRegistrationsPage", () => {
  beforeEach(() => {
    mockPush.mockReset();
    mockFetchList.mockReset();
    mockDownload.mockReset();
    mockDownloadBlob.mockReset();
    mockFetchList.mockResolvedValue(HOLIDAY_REGISTRATION_LIST);
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", "traeger.localhost:3000");
  });
  afterEach(() => vi.unstubAllEnvs());

  it("lists the registrations with status, schools, weeks and the duplicate hint", async () => {
    renderPage();

    expect(
      await screen.findByText(
        "2 Anmeldungen · 1 eingegangen · 0 bestätigt · 1 storniert",
      ),
    ).toBeInTheDocument();
    const table = within(screen.getByRole("table"));
    expect(table.getByText("Eva Muster")).toBeInTheDocument();
    expect(table.getByText("Lia Muster")).toBeInTheDocument();
    expect(table.getByText("OGS Nord")).toBeInTheDocument();
    expect(
      table.getByText("Feriengast (Grundschule Melle)"),
    ).toBeInTheDocument();
    expect(
      table.getByText("04.01. bis 08.01.2027: bis 16 Uhr"),
    ).toBeInTheDocument();
    expect(table.getByText("Mögliche Doppelanmeldung")).toBeInTheDocument();
    expect(
      screen.getByText(
        "In einer Anmeldung ist ein Kind vielleicht doppelt angemeldet. Prüfen Sie die markierte Anmeldung.",
      ),
    ).toBeInTheDocument();
    expect(table.getByText("Eingegangen")).toBeInTheDocument();
    expect(mockFetchList).toHaveBeenCalledWith("5", {
      status: "",
      school: "",
      weekId: "",
      timeModel: "",
    });
  });

  it("filters by status, school, week and time model", async () => {
    renderPage();
    await within(await screen.findByRole("table")).findByText("Eva Muster");

    fireEvent.click(screen.getByRole("combobox", { name: "Status" }));
    fireEvent.click(await screen.findByRole("option", { name: "Bestätigt" }));
    await waitFor(() =>
      expect(mockFetchList).toHaveBeenLastCalledWith(
        "5",
        expect.objectContaining({ status: "confirmed" }),
      ),
    );

    fireEvent.click(screen.getByRole("combobox", { name: "Schule" }));
    fireEvent.click(await screen.findByRole("option", { name: "Feriengäste" }));
    await waitFor(() =>
      expect(mockFetchList).toHaveBeenLastCalledWith(
        "5",
        expect.objectContaining({ school: "guest" }),
      ),
    );

    fireEvent.click(screen.getByRole("combobox", { name: "Woche" }));
    fireEvent.click(
      await screen.findByRole("option", {
        name: "11.01. bis 15.01.2027, OGS Süd",
      }),
    );
    fireEvent.click(screen.getByRole("combobox", { name: "Zeitmodell" }));
    fireEvent.click(await screen.findByRole("option", { name: "bis 13 Uhr" }));
    await waitFor(() =>
      expect(mockFetchList).toHaveBeenLastCalledWith("5", {
        status: "confirmed",
        school: "guest",
        weekId: "52",
        timeModel: "bis 13 Uhr",
      }),
    );
  });

  it("opens a registration from the list", async () => {
    renderPage();
    fireEvent.click(
      await within(await screen.findByRole("table")).findByText("Eva Muster"),
    );

    expect(mockPush).toHaveBeenCalledWith("/traeger/ferien/5/anmeldungen/7");
  });

  it("exports the filtered list as CSV and as Excel", async () => {
    const blob = new Blob(["Status"]);
    mockDownload.mockResolvedValue({
      blob,
      filename: "ferienanmeldungen.xlsx",
    });
    renderPage();
    await within(await screen.findByRole("table")).findByText("Eva Muster");

    fireEvent.click(
      screen.getByRole("button", { name: "Als Excel herunterladen" }),
    );
    await waitFor(() =>
      expect(mockDownloadBlob).toHaveBeenCalledWith(
        blob,
        "ferienanmeldungen.xlsx",
      ),
    );
    expect(mockDownload).toHaveBeenCalledWith(
      "5",
      expect.objectContaining({ status: "" }),
      "xlsx",
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Als CSV herunterladen" }),
    );
    await waitFor(() =>
      expect(mockDownload).toHaveBeenLastCalledWith(
        "5",
        expect.anything(),
        "csv",
      ),
    );
  });

  it("explains an empty period", async () => {
    mockFetchList.mockResolvedValue({
      registrations: [],
      total: 0,
      statusCounts: { received: 0, confirmed: 0, cancelled: 0 },
    });
    renderPage();

    expect(
      (
        await screen.findAllByText("Es ist noch keine Anmeldung eingegangen.")
      )[0],
    ).toBeInTheDocument();
    expect(
      screen.getAllByText(
        "Eltern melden ihre Kinder über den Link für Eltern an.",
      )[0],
    ).toBeInTheDocument();
  });
});
