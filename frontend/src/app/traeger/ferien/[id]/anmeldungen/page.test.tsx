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
const {
  mockPush,
  mockFetchList,
  mockDownload,
  mockDownloadPdfs,
  mockDownloadBlob,
  sessionPermissions,
} = vi.hoisted(() => ({
  mockPush: vi.fn(),
  mockFetchList: vi.fn(),
  mockDownload: vi.fn(),
  mockDownloadPdfs: vi.fn(),
  mockDownloadBlob: vi.fn(),
  sessionPermissions: { current: [] as string[] },
}));

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
  downloadHolidayRegistrationPdfs: (
    periodId: string,
    filter: HolidayRegistrationFilter,
    withHealth: boolean,
  ) => mockDownloadPdfs(periodId, filter, withHealth),
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
const { mockConfirm } = vi.hoisted(() => ({ mockConfirm: vi.fn() }));
vi.mock("~/lib/traeger-contributions-api", () => ({
  confirmHolidayRegistration: (
    periodId: string,
    registrationId: string,
    benefitsExemption: boolean,
  ) => mockConfirm(periodId, registrationId, benefitsExemption),
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
      wish: "",
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
        wish: "",
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

  it("says when moto deletes the registrations and downloads them as PDFs in a ZIP (#6691)", async () => {
    sessionPermissions.current = [];
    mockFetchList.mockResolvedValue({
      ...HOLIDAY_REGISTRATION_LIST,
      registrations: [
        {
          ...HOLIDAY_REGISTRATION_LIST.registrations[0]!,
          deletesOn: "2028-01-09",
          healthDeletesOn: "2027-04-11",
        },
      ],
    });
    const blob = new Blob(["PK"]);
    mockDownloadPdfs.mockResolvedValue({
      blob,
      filename: "ferienanmeldungen-winterferien.zip",
    });
    renderPage();
    await within(await screen.findByRole("table")).findByText("Eva Muster");

    expect(
      screen.getByText(
        "moto löscht diese Anmeldungen am 09.01.2028. Laden Sie sie vorher herunter, wenn Sie sie länger brauchen. Gesundheitsangaben löscht moto schon am 11.04.2027.",
      ),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Anmeldungen herunterladen" }),
    );
    // Without the right to see health information there is no such file.
    expect(
      screen.queryByRole("menuitem", { name: /Gesundheitsangaben/ }),
    ).not.toBeInTheDocument();
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Alle als PDF (ZIP)" }),
    );
    await waitFor(() =>
      expect(mockDownloadBlob).toHaveBeenCalledWith(
        blob,
        "ferienanmeldungen-winterferien.zip",
      ),
    );
    expect(mockDownloadPdfs).toHaveBeenCalledWith(
      "5",
      expect.objectContaining({ status: "" }),
      false,
    );
  });

  it("offers the PDFs with health information only with the right to see it (#6691)", async () => {
    sessionPermissions.current = ["traeger_health:read"];
    mockDownloadPdfs.mockResolvedValue({
      blob: new Blob(["PK"]),
      filename: null,
    });
    renderPage();
    await within(await screen.findByRole("table")).findByText("Eva Muster");

    fireEvent.click(
      screen.getByRole("button", { name: "Anmeldungen herunterladen" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", {
        name: "Alle als PDF mit Gesundheitsangaben (ZIP)",
      }),
    );
    await waitFor(() =>
      expect(mockDownloadPdfs).toHaveBeenCalledWith(
        "5",
        expect.anything(),
        true,
      ),
    );
    sessionPermissions.current = [];
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

  it("selects two registrations and merges them as siblings", async () => {
    mockMerge.mockResolvedValue({ id: "3", registrationIds: ["7", "8"] });
    renderPage();
    const table = within(await screen.findByRole("table"));

    fireEvent.click(
      await table.findByRole("checkbox", {
        name: "Anmeldung Nr. 7 von Eva Muster auswählen",
      }),
    );
    // One registration can be confirmed, not merged (#6674).
    expect(screen.getByText("1 Anmeldung ausgewählt")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Ausgewählte bestätigen" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Als Geschwister zusammenführen" }),
    ).not.toBeInTheDocument();
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

// Several registrations confirmed at once (#6674).
const CLEAN = {
  ...HOLIDAY_REGISTRATION_LIST.registrations[0]!,
  children: HOLIDAY_REGISTRATION_LIST.registrations[0]!.children.map(
    (child) => ({ ...child, duplicateRegistrationIds: [] }),
  ),
};
const BULK_LIST = {
  ...HOLIDAY_REGISTRATION_LIST,
  registrations: [
    { ...CLEAN, id: "21", guardianFirstName: "Anna" },
    { ...CLEAN, id: "22", guardianFirstName: "Bodo" },
    { ...CLEAN, id: "23", guardianFirstName: "Cem", benefitsDeclared: true },
    {
      ...CLEAN,
      id: "24",
      guardianFirstName: "Dana",
      status: "confirmed" as const,
    },
  ],
};

async function selectRows(names: readonly string[]) {
  const table = within(await screen.findByRole("table"));
  for (const name of names) {
    fireEvent.click(
      await table.findByRole("checkbox", { name: `${name} auswählen` }),
    );
  }
}

describe("TraegerHolidayRegistrationsPage bulk confirmation (#6674)", () => {
  beforeEach(() => {
    mockFetchList.mockReset();
    mockFetchGroups.mockReset();
    mockConfirm.mockReset();
    mockFetchList.mockResolvedValue(BULK_LIST);
    mockFetchGroups.mockResolvedValue([]);
    mockConfirmationSettings.mockResolvedValue({ debtorNumberRequired: false });
    vi.stubEnv("NEXT_PUBLIC_TRAEGER_HOSTNAME", "traeger.localhost:3000");
  });
  afterEach(() => vi.unstubAllEnvs());

  it("confirms what needs no input, one by one, and names why the rest stays open", async () => {
    mockConfirm.mockImplementation((_period: string, id: string) =>
      id === "22"
        ? Promise.reject(
            new ApiError("debtor", 400, {
              code: "parent_billing.debtor_number_invalid",
            }),
          )
        : Promise.resolve({}),
    );
    renderPage();
    await selectRows([
      "Anmeldung Nr. 21 von Anna Muster",
      "Anmeldung Nr. 22 von Bodo Muster",
      "Anmeldung Nr. 23 von Cem Muster",
      "Anmeldung Nr. 24 von Dana Muster",
    ]);
    fireEvent.click(
      screen.getByRole("button", { name: "Ausgewählte bestätigen" }),
    );

    const review = within(
      await screen.findByRole("dialog", {
        name: "2 Anmeldungen bestätigen?",
      }),
    );
    expect(
      review.getByText(
        "Die Familie bezahlt den Ferienbeitrag. Die Eltern bekommen eine E-Mail mit den gebuchten Wochen.",
      ),
    ).toBeInTheDocument();
    expect(review.getByText("Wird bestätigt")).toBeInTheDocument();
    expect(review.getByText("Wer bezahlt, ist offen")).toBeInTheDocument();
    expect(
      review.getByText("Schon bestätigt oder storniert"),
    ).toBeInTheDocument();
    expect(mockConfirm).not.toHaveBeenCalled();

    fireEvent.click(review.getByRole("button", { name: "2 bestätigen" }));

    const result = within(
      await screen.findByRole("dialog", { name: "1 Anmeldung bestätigt" }),
    );
    expect(mockConfirm.mock.calls).toEqual([
      ["5", "21", false],
      ["5", "22", false],
    ]);
    expect(
      result.getByText(
        "3 Anmeldungen bleiben offen. Bestätigen Sie sie einzeln.",
      ),
    ).toBeInTheDocument();
    expect(result.getByText("Debitorennummer fehlt")).toBeInTheDocument();
    expect(result.getByRole("link", { name: /Bodo Muster/ })).toHaveAttribute(
      "href",
      "/traeger/ferien/5/anmeldungen/22",
    );

    const loads = mockFetchList.mock.calls.length;
    fireEvent.click(result.getByRole("button", { name: "Fertig" }));
    await waitFor(() =>
      expect(mockFetchList.mock.calls.length).toBeGreaterThan(loads),
    );
    expect(
      screen.queryByText(/Anmeldungen? ausgewählt/),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("table")).toBeInTheDocument();
  });

  it("celebrates when every selected registration is confirmed", async () => {
    mockConfirm.mockResolvedValue({});
    renderPage();
    await selectRows(["Anmeldung Nr. 21 von Anna Muster"]);
    fireEvent.click(
      screen.getByRole("button", { name: "Ausgewählte bestätigen" }),
    );
    fireEvent.click(
      within(
        await screen.findByRole("dialog", {
          name: "1 Anmeldung bestätigen?",
        }),
      ).getByRole("button", { name: "Bestätigen" }),
    );

    expect(
      await screen.findByText("Alle ausgewählten Anmeldungen sind bestätigt."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("dialog", { name: "1 Anmeldung bestätigt" }),
    ).toBeInTheDocument();
  });

  it("stops when no further confirmation can succeed", async () => {
    mockConfirm.mockRejectedValue(new ApiError("expired", 401));
    renderPage();
    await selectRows([
      "Anmeldung Nr. 21 von Anna Muster",
      "Anmeldung Nr. 22 von Bodo Muster",
    ]);
    fireEvent.click(
      screen.getByRole("button", { name: "Ausgewählte bestätigen" }),
    );
    fireEvent.click(
      within(
        await screen.findByRole("dialog", {
          name: "2 Anmeldungen bestätigen?",
        }),
      ).getByRole("button", { name: "2 bestätigen" }),
    );

    const result = within(
      await screen.findByRole("dialog", { name: "Keine Anmeldung bestätigt" }),
    );
    expect(mockConfirm).toHaveBeenCalledTimes(1);
    expect(result.getByText("Nicht mehr bestätigt")).toBeInTheDocument();
    const loads = mockFetchList.mock.calls.length;
    fireEvent.click(result.getByRole("button", { name: "Fertig" }));
    // Nothing changed: the selection stays for another try.
    expect(screen.getByText("2 Anmeldungen ausgewählt")).toBeInTheDocument();
    expect(mockFetchList.mock.calls.length).toBe(loads);
  });

  it("explains a selection without anything to confirm", async () => {
    renderPage();
    await selectRows(["Anmeldung Nr. 23 von Cem Muster"]);
    fireEvent.click(
      screen.getByRole("button", { name: "Ausgewählte bestätigen" }),
    );

    const dialog = within(
      await screen.findByRole("dialog", {
        name: "Keine Anmeldung lässt sich hier bestätigen",
      }),
    );
    expect(dialog.getByText("Wer bezahlt, ist offen")).toBeInTheDocument();
    fireEvent.click(dialog.getByRole("button", { name: "Schließen" }));
    expect(mockConfirm).not.toHaveBeenCalled();
  });
});

function render(ui: React.ReactElement) {
  return rtlRender(
    <SWRConfig value={{ provider: () => new Map() }}>{ui}</SWRConfig>,
  );
}

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: {
        id: "A",
        scope: "org",
        orgId: 7,
        permissions: sessionPermissions.current,
      },
    },
    status: "authenticated",
  }),
}));
