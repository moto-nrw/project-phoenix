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
import { ApiError } from "~/lib/api-error";
import type { HolidayRegistrationFilter } from "~/lib/traeger-holiday-registrations-api";
import {
  HOLIDAY_PERIOD,
  HOLIDAY_REGISTRATION_LIST,
  HOLIDAY_SCHOOLS,
} from "~/test/fixtures/holiday-registrations";

const { mockConfirmationSettings } = vi.hoisted(() => ({
  mockConfirmationSettings: vi.fn(),
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
  fetchHolidayConfirmationSettings: () => mockConfirmationSettings(),
}));
const { mockFetchGroups, mockMerge } = vi.hoisted(() => ({
  mockFetchGroups: vi.fn(),
  mockMerge: vi.fn(),
}));
vi.mock("~/lib/traeger-sibling-groups-api", () => ({
  fetchHolidaySiblingGroups: (periodId: string) => mockFetchGroups(periodId),
  mergeHolidaySiblings: (periodId: string, ids: readonly string[]) =>
    mockMerge(periodId, ids),
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
    mockFetchGroups.mockReset();
    mockMerge.mockReset();
    mockFetchGroups.mockResolvedValue([]);
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", "traeger.localhost:3000");
  });
  afterEach(() => vi.unstubAllEnvs());

  it("lists the registrations with status, schools, weeks and the duplicate hint", async () => {
    renderPage();

    // Anker: die Zahlen stehen einzeln, ohne Trennpunkte dazwischen (#5684).
    expect(await screen.findByText("2 Anmeldungen")).toBeInTheDocument();
    expect(screen.getByText("1 eingegangen")).toBeInTheDocument();
    expect(screen.getByText("0 bestätigt")).toBeInTheDocument();
    expect(screen.getByText("1 storniert")).toBeInTheDocument();
    expect(screen.queryByText(/·/)).not.toBeInTheDocument();
    const table = within(screen.getByRole("table"));
    expect(table.getByText("Eva Muster")).toBeInTheDocument();
    expect(table.getByText("Lia Muster")).toBeInTheDocument();
    expect(table.getByText("OGS Nord")).toBeInTheDocument();
    expect(
      table.getByText("Feriengast (Grundschule Melle)"),
    ).toBeInTheDocument();
    expect(
      table.getByText("04.01. bis 08.01.2027, OGS Nord: bis 16 Uhr"),
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
    fireEvent.click(await screen.findByRole("option", { name: "Bestätigt" }));
    await waitFor(() =>
      expect(mockFetchList).toHaveBeenLastCalledWith(
        "5",
        expect.objectContaining({ status: "confirmed" }),
      ),
    );

    fireEvent.click(screen.getByRole("button", { name: "Alle Schulen" }));
    fireEvent.click(await screen.findByRole("option", { name: "Feriengäste" }));
    await waitFor(() =>
      expect(mockFetchList).toHaveBeenLastCalledWith(
        "5",
        expect.objectContaining({ school: "guest" }),
      ),
    );

    fireEvent.click(screen.getByRole("button", { name: "Alle Wochen" }));
    fireEvent.click(
      await screen.findByRole("option", {
        name: "11.01. bis 15.01.2027, OGS Süd",
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Alle Zeitmodelle" }));
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

// Siblings from separate registrations (#5628): three registrations, two of
// them by the same e-mail address, the third with the same last name.
const SIBLING_LIST = {
  ...HOLIDAY_REGISTRATION_LIST,
  registrations: [
    HOLIDAY_REGISTRATION_LIST.registrations[0]!,
    {
      ...HOLIDAY_REGISTRATION_LIST.registrations[0]!,
      id: "8",
      guardianFirstName: "Tom",
      guardianEmail: "EVA@example.org",
      children: [],
    },
    {
      ...HOLIDAY_REGISTRATION_LIST.registrations[0]!,
      id: "9",
      guardianFirstName: "Ute",
      guardianEmail: "ute@example.org",
      children: [],
    },
  ],
};

describe("TraegerHolidayRegistrationsPage siblings (#5628)", () => {
  beforeEach(() => {
    mockFetchList.mockReset();
    mockFetchGroups.mockReset();
    mockMerge.mockReset();
    mockFetchList.mockResolvedValue(SIBLING_LIST);
    mockFetchGroups.mockResolvedValue([]);
    mockConfirmationSettings.mockResolvedValue({ debtorNumberRequired: false });
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", "traeger.localhost:3000");
  });
  afterEach(() => vi.unstubAllEnvs());

  it("hints at siblings by e-mail and last name without merging", async () => {
    renderPage();
    const table = within(await screen.findByRole("table"));

    expect(
      await table.findAllByText("Vielleicht Geschwister: gleiche E-Mail"),
    ).toHaveLength(2);
    expect(table.getByText("Nr. 8")).toBeInTheDocument();
    expect(table.getByText("Nr. 7")).toBeInTheDocument();
    expect(
      table.getByText("Vielleicht Geschwister: gleicher Nachname"),
    ).toBeInTheDocument();
    expect(table.getByText("Nr. 7, 8")).toBeInTheDocument();
    expect(mockMerge).not.toHaveBeenCalled();
  });

  // A parent calls: the office finds the registration by name or number;
  // a registration the search hides drops out of the selection.
  it("finds a registration by parent or number and keeps only shown rows selected", async () => {
    renderPage();
    const table = within(await screen.findByRole("table"));
    fireEvent.click(
      await table.findByRole("checkbox", {
        name: "Anmeldung Nr. 7 von Eva Muster auswählen",
      }),
    );

    const [field] = screen.getAllByPlaceholderText(
      "Eltern, Kind, Nr. oder Debitorennummer",
    );
    fireEvent.change(field!, { target: { value: "ute" } });

    const row = (id: string, name: string) =>
      table.queryByRole("checkbox", {
        name: `Anmeldung Nr. ${id} von ${name} auswählen`,
      });
    await waitFor(() => expect(row("9", "Ute Muster")).toBeInTheDocument());
    expect(row("7", "Eva Muster")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("region", { name: "Ausgewählte Anmeldungen" }),
    ).not.toBeInTheDocument();

    fireEvent.change(field!, { target: { value: "nr. 8" } });
    await waitFor(() => expect(row("8", "Tom Muster")).toBeInTheDocument());
    expect(row("9", "Ute Muster")).not.toBeInTheDocument();
  });

  it("selects two registrations and merges them as siblings", async () => {
    mockMerge.mockResolvedValue({ id: "3", registrationIds: ["7", "8"] });
    renderPage();
    const table = within(await screen.findByRole("table"));

    fireEvent.click(
      await table.findByRole("checkbox", {
        name: "Anmeldung Nr. 7 von Eva Muster auswählen",
      }),
    );
    const bar = within(
      screen.getByRole("region", { name: "Ausgewählte Anmeldungen" }),
    );
    expect(bar.getByText("1 Anmeldung ausgewählt.")).toBeInTheDocument();
    expect(
      bar.getByText(
        "Wählen Sie mindestens eine weitere Anmeldung derselben Familie.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(
      table.getByRole("checkbox", {
        name: "Anmeldung Nr. 8 von Tom Muster auswählen",
      }),
    );
    mockFetchGroups.mockResolvedValue([
      { id: "3", registrationIds: ["7", "8"] },
    ]);
    fireEvent.click(
      screen.getByRole("button", { name: "Als Geschwister zusammenführen" }),
    );

    await waitFor(() =>
      expect(mockMerge).toHaveBeenCalledWith("5", ["7", "8"]),
    );
    expect(
      await screen.findByText(
        "Die Anmeldungen sind als Geschwister zusammengeführt.",
      ),
    ).toBeInTheDocument();
    expect(await table.findByText("mit Nr. 8")).toBeInTheDocument();
    expect(table.getByText("mit Nr. 7")).toBeInTheDocument();
    expect(
      table.getByText("Vielleicht Geschwister: gleicher Nachname"),
    ).toBeInTheDocument();
    expect(table.getByText("Nr. 7, 8")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Als Geschwister zusammenführen" }),
    ).not.toBeInTheDocument();
  });

  it("locks the selection while new results load and keeps only shown rows", async () => {
    renderPage();
    const table = within(await screen.findByRole("table"));
    fireEvent.click(
      await table.findByRole("checkbox", {
        name: "Anmeldung Nr. 7 von Eva Muster auswählen",
      }),
    );
    fireEvent.click(
      table.getByRole("checkbox", {
        name: "Anmeldung Nr. 8 von Tom Muster auswählen",
      }),
    );
    expect(screen.getByText(/2 Anmeldungen ausgewählt/)).toBeInTheDocument();

    let resolveList: (list: typeof SIBLING_LIST) => void = () => {};
    mockFetchList.mockReturnValue(
      new Promise((resolve) => {
        resolveList = resolve;
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Alle Status" }));
    fireEvent.click(await screen.findByRole("option", { name: "Eingegangen" }));

    await waitFor(() =>
      expect(
        table.getByRole("checkbox", {
          name: "Anmeldung Nr. 9 von Ute Muster auswählen",
        }),
      ).toBeDisabled(),
    );
    expect(
      screen.queryByText(/Anmeldungen? ausgewählt/),
    ).not.toBeInTheDocument();

    resolveList({
      ...SIBLING_LIST,
      registrations: [SIBLING_LIST.registrations[2]!],
    });
    const enabled = await table.findByRole("checkbox", {
      name: "Anmeldung Nr. 9 von Ute Muster auswählen",
    });
    await waitFor(() => expect(enabled).toBeEnabled());
    expect(mockMerge).not.toHaveBeenCalled();
  });

  it("names the issued invoice when the merge is refused", async () => {
    mockMerge.mockRejectedValue(
      new ApiError("invoiced", 409, {
        code: "parent_billing.sibling_group_invoiced",
      }),
    );
    renderPage();
    const table = within(await screen.findByRole("table"));
    fireEvent.click(
      await table.findByRole("checkbox", {
        name: "Anmeldung Nr. 7 von Eva Muster auswählen",
      }),
    );
    fireEvent.click(
      table.getByRole("checkbox", {
        name: "Anmeldung Nr. 9 von Ute Muster auswählen",
      }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Als Geschwister zusammenführen" }),
    );

    expect(
      await screen.findByText(
        "Für diese Anmeldung gibt es schon eine Rechnung. Geschwister können Sie jetzt nicht mehr ändern.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText(/2 Anmeldungen ausgewählt/)).toBeInTheDocument();
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
