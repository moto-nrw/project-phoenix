import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type {
  ContractImportResult,
  ContractImportRow,
} from "~/lib/traeger-care-contract-import-api";
import TraegerContractImportPage from "./page";

const api = vi.hoisted(() => ({
  previewContractImport: vi.fn(),
  applyContractImport: vi.fn(),
  fetchContractImportLog: vi.fn(),
}));
vi.mock("~/lib/traeger-care-contract-import-api", async (original) => ({
  ...(await original<object>()),
  ...api,
}));
vi.mock("next-auth/react", () => ({ useSession: () => ({ data: null }) }));
vi.mock("~/lib/auth-utils", () => ({ hasPermission: () => true }));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));
vi.mock("~/lib/traeger-api", () => ({
  fetchTraegerBuero: () => Promise.resolve({ members: [] }),
}));
vi.mock("~/components/ui/tenant-page", () => ({
  TenantPage: ({ children }: { children: ReactNode }) => children,
}));

function row(
  line: number,
  overrides: Partial<ContractImportRow> = {},
): ContractImportRow {
  return {
    line,
    status: "ready",
    childFirstName: "Mia",
    childLastName: "Beispiel",
    childBirthDate: "2018-05-03",
    schoolName: "Grundschule Nord",
    moduleNames: ["OGS bis 16 Uhr"],
    weekdays: ["mon", "tue", "wed", "thu"],
    effectiveFrom: "2026-08-01",
    endsOn: "2027-07-31",
    partyName: "Eva Beispiel",
    partyEmail: "eva.beispiel@example.org",
    problems: [],
    contractId: null,
    repeatsLine: null,
    debtorNumber: "70001",
    debtorNumberStatus: "new",
    ...overrides,
  };
}

function preview(
  overrides: Partial<ContractImportResult> = {},
): ContractImportResult {
  return {
    encoding: "utf-8",
    delimiter: ";",
    headers: ["Vorname", "Nachname", "Beginn", "Zahler"],
    mapping: {
      columns: {
        child_first_name: "Vorname",
        child_last_name: "Nachname",
        child_birth_date: "",
        school: "",
        module: "",
        weekdays: "",
        effective_from: "Beginn",
        ends_on: "",
        concluded_on: "",
        party_first_name: "",
        party_last_name: "Zahler",
        party_email: "",
        debtor_number: "",
      },
      schoolId: "1",
      moduleName: "OGS bis 16 Uhr",
    },
    unmapped: [],
    schools: [
      {
        id: "1",
        name: "Grundschule Nord",
        modules: [
          { name: "OGS bis 16 Uhr", phaseName: "2026/27", hasConditions: true },
        ],
      },
      { id: "2", name: "Grundschule Süd", modules: [] },
    ],
    debtorNumberScope: "payer",
    fingerprint: "preview-1",
    applied: false,
    counts: {
      ready: 1,
      duplicate: 1,
      conflict: 0,
      invalid: 0,
      imported: 0,
      debtorNumbers: 1,
      debtorConflicts: 0,
    },
    rows: [
      row(2),
      row(3, {
        status: "duplicate",
        childFirstName: "Ben",
        contractId: "7",
        debtorNumber: "",
        debtorNumberStatus: "",
      }),
    ],
    ...overrides,
  };
}

async function choose(result = preview()) {
  api.previewContractImport.mockResolvedValueOnce(result);
  render(
    <ToastProvider>
      <TraegerContractImportPage />
    </ToastProvider>,
  );
  fireEvent.change(screen.getByLabelText("Datei wählen"), {
    target: {
      files: [
        new File(["Vorname;Nachname"], "kinder.csv", { type: "text/csv" }),
      ],
    },
  });
  await screen.findByRole("heading", { name: "Spalten zuordnen" });
}

describe("Vertragsimport", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    api.fetchContractImportLog.mockResolvedValue([]);
  });

  it("shows every row with what moto does and takes over the confirmed preview", async () => {
    await choose();
    expect(api.previewContractImport).toHaveBeenCalledWith(
      expect.objectContaining({ mapping: null }),
    );
    expect(screen.getAllByText("Mia Beispiel").length).toBeGreaterThan(0);
    expect(
      screen.getAllByText("Debitorennummer 70001 wird eingetragen.").length,
    ).toBeGreaterThan(0);
    expect(
      screen.getAllByText("Das Kind hat an dieser Schule schon einen Vertrag.")
        .length,
    ).toBeGreaterThan(0);

    const applied = preview({
      applied: true,
      counts: { ...preview().counts, ready: 0, imported: 1 },
    });
    api.applyContractImport.mockResolvedValueOnce(applied);
    api.fetchContractImportLog.mockResolvedValueOnce([
      {
        file: "kinder.csv",
        accountId: "4",
        importedAt: "2026-09-09T10:00:00Z",
        contracts: 1,
      },
    ]);
    fireEvent.click(
      screen.getByRole("button", { name: "1 Vertrag übernehmen" }),
    );
    await screen.findByText("1 Vertrag übernommen");
    expect(api.applyContractImport).toHaveBeenCalledWith(
      expect.objectContaining({
        fingerprint: "preview-1",
        mapping: preview().mapping,
      }),
    );
    expect(
      screen.getByText(/1 schon vorhanden, 0 nicht übernommen/),
    ).toBeInTheDocument();
    expect(await screen.findAllByText("kinder.csv")).not.toHaveLength(0);
  });

  it("asks for the school and the module of a file without those columns", async () => {
    await choose(
      preview({
        mapping: { ...preview().mapping, schoolId: "", moduleName: "" },
        unmapped: ["school", "module"],
        fingerprint: "",
        rows: [],
        counts: { ...preview().counts, ready: 0, duplicate: 0 },
      }),
    );
    expect(
      screen.getByText("Bitte eine Spalte oder eine Schule wählen."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Ordnen Sie alle Pflichtfelder zu. Dann zeigt moto die Zeilen.",
      ),
    ).toBeInTheDocument();
    // Anker: the mapping is on screen, so the preview could show.
    expect(
      screen.queryByRole("heading", { name: "Prüfen und übernehmen" }),
    ).not.toBeInTheDocument();

    api.previewContractImport.mockResolvedValueOnce(preview());
    fireEvent.click(
      screen.getByRole("combobox", { name: /Schule für alle Zeilen/ }),
    );
    fireEvent.click(screen.getByRole("option", { name: "Grundschule Nord" }));
    await screen.findByRole("heading", { name: "Prüfen und übernehmen" });
    expect(api.previewContractImport).toHaveBeenLastCalledWith(
      expect.objectContaining({
        mapping: expect.objectContaining({ schoolId: "1", moduleName: "" }),
      }),
    );
  });

  it("names missing contract conditions as the step that comes first", async () => {
    await choose(
      preview({
        counts: { ...preview().counts, ready: 0, duplicate: 0, invalid: 1 },
        rows: [
          row(2, {
            status: "invalid",
            problems: [{ field: "module", code: "conditions_missing" }],
            debtorNumber: "",
            debtorNumberStatus: "",
          }),
        ],
      }),
    );
    expect(
      screen.getByText("Zuerst die Vertragsbedingungen eintragen"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Zu den Vertragsbedingungen" }),
    ).toHaveAttribute("href", "/traeger/vertraege/bedingungen");
    // Blocked with a reason: nothing is ready.
    expect(
      screen.getByRole("button", { name: "0 Verträge übernehmen" }),
    ).toHaveAttribute("aria-disabled", "true");
  });

  it("reads the file again when the preview changed before the import", async () => {
    await choose();
    api.applyContractImport.mockRejectedValueOnce(
      new ApiError("Request failed (409)", 409, {
        code: "care_contract.import_changed",
      }),
    );
    api.previewContractImport.mockResolvedValueOnce(
      preview({ fingerprint: "preview-2" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "1 Vertrag übernehmen" }),
    );
    await waitFor(() =>
      expect(api.previewContractImport).toHaveBeenCalledTimes(2),
    );
    expect(
      screen.getByRole("heading", { name: "Prüfen und übernehmen" }),
    ).toBeInTheDocument();
  });
});
