import { SWRConfig } from "~/lib/swr/testing";
vi.unmock("swr");
import {
  fireEvent,
  render as rtlRender,
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

const { mockConfirmationSettings } = vi.hoisted(() => ({
  mockConfirmationSettings: vi.fn(),
}));
vi.mock("~/lib/traeger-contributions-api", () => ({
  fetchHolidayConfirmationSettings: () => mockConfirmationSettings(),
}));
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
    mockConfirmationSettings.mockResolvedValue({ debtorNumberRequired: false });
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
    // Planung and Anmeldungen are the two page tabs of the period (#5146).
    expect(screen.getByRole("tab", { name: /Anmeldungen/ })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(screen.getByRole("tab", { name: "Planung" })).toHaveAttribute(
      "href",
      "/traeger/ferien/5",
    );
    expect(mockFetchList).toHaveBeenCalledWith("5", {
      status: "",
      school: "",
      weekId: "",
      timeModel: "",
    });
  });

  it("shows the debtor number column only while the Träger asks for it (#5621)", async () => {
    mockFetchList.mockResolvedValue({
      ...HOLIDAY_REGISTRATION_LIST,
      registrations: [
        {
          ...HOLIDAY_REGISTRATION_LIST.registrations[0]!,
          debtorNumber: "10 4711",
        },
      ],
    });
    mockConfirmationSettings.mockResolvedValue({ debtorNumberRequired: true });
    renderPage();

    const table = within(await screen.findByRole("table"));
    expect(
      await table.findByRole("columnheader", { name: /Debitorennummer/ }),
    ).toBeInTheDocument();
    expect(table.getByText("10 4711")).toBeInTheDocument();
  });

  it("leaves the debtor number column out while the switch is off (#5621)", async () => {
    renderPage();

    const table = within(await screen.findByRole("table"));
    expect(
      table.getByRole("columnheader", { name: /Status/ }),
    ).toBeInTheDocument();
    await waitFor(() => expect(mockConfirmationSettings).toHaveBeenCalled());
    expect(
      table.queryByRole("columnheader", { name: /Debitorennummer/ }),
    ).not.toBeInTheDocument();
  });

  it("filters by status, school, week and time model", async () => {
    renderPage();
    await within(await screen.findByRole("table")).findByText("Eva Muster");

    // Die Filter stehen in der Filterzeile des Seitenkopfs (#5146); jeder
    // Auslöser nennt seinen aktuellen Wert.
    fireEvent.click(screen.getByRole("button", { name: "Alle Status" }));
    fireEvent.click(await screen.findByRole("button", { name: "Bestätigt" }));
    await waitFor(() =>
      expect(mockFetchList).toHaveBeenLastCalledWith(
        "5",
        expect.objectContaining({ status: "confirmed" }),
      ),
    );

    fireEvent.click(screen.getByRole("button", { name: "Alle Schulen" }));
    fireEvent.click(await screen.findByRole("button", { name: "Feriengäste" }));
    await waitFor(() =>
      expect(mockFetchList).toHaveBeenLastCalledWith(
        "5",
        expect.objectContaining({ school: "guest" }),
      ),
    );

    fireEvent.click(screen.getByRole("button", { name: "Alle Wochen" }));
    fireEvent.click(
      await screen.findByRole("button", {
        name: "11.01. bis 15.01.2027, OGS Süd",
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Alle Zeitmodelle" }));
    fireEvent.click(await screen.findByRole("button", { name: "bis 13 Uhr" }));
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

    // Export als Menü im Seitenkopf (#5146).
    fireEvent.click(
      screen.getByRole("button", { name: "Anmeldungen herunterladen" }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "Excel" }));
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
      screen.getByRole("button", { name: "Anmeldungen herunterladen" }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "CSV" }));
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

function render(ui: React.ReactElement) {
  return rtlRender(
    <SWRConfig value={{ provider: () => new Map() }}>{ui}</SWRConfig>,
  );
}

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "A", scope: "org", orgId: 7 } },
    status: "authenticated",
  }),
}));
