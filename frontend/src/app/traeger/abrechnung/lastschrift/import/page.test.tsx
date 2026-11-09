import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { ImportResult } from "~/lib/traeger-sepa-import-api";
import TraegerSepaImportPage from "./page";

const api = vi.hoisted(() => ({
  previewSepaImport: vi.fn(),
  applySepaImport: vi.fn(),
  fetchSepaImportLog: vi.fn(),
}));
vi.mock("~/lib/traeger-sepa-import-api", async (original) => ({
  ...(await original<object>()),
  ...api,
}));
vi.mock("next-auth/react", () => ({ useSession: () => ({ data: null }) }));
vi.mock("~/lib/auth-utils", () => ({ hasPermission: () => true }));
vi.mock("~/lib/traeger-sepa-api", () => ({
  fetchSepaCreditor: () =>
    Promise.resolve({ creditorId: "DE98ZZZ09999999999" }),
}));
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));
vi.mock("~/components/ui/tenant-page", () => ({
  TenantPage: ({ children }: { children: ReactNode }) => children,
}));

const preview: ImportResult = {
  encoding: "utf-8",
  delimiter: ";",
  headers: ["Referenz", "IBAN", "Anderes Konto"],
  mapping: {
    columns: {
      reference: "Referenz",
      signed_on: "",
      iban: "IBAN",
      bic: "",
      account_holder: "",
      payer: "",
      creditor_id: "",
      sequence: "",
      last_collected_on: "",
    },
    payerKey: "email",
    creditorFromTraeger: true,
  },
  unmapped: [],
  creditorId: "DE98ZZZ09999999999",
  applied: false,
  counts: {
    ready: 1,
    clarify: 0,
    duplicate: 0,
    conflict: 0,
    invalid: 0,
    imported: 0,
  },
  rows: [
    {
      line: 2,
      status: "ready",
      reference: "OGS-1",
      signedOn: "2020-01-01",
      ibanMasked: "DE•• 3000",
      bic: "",
      accountHolder: "Eva Beispiel",
      contractId: "7",
      payerFirstName: "Eva",
      payerLastName: "Beispiel",
      sequence: "recurring",
      lastCollectedOn: null,
      mandateId: null,
      problems: [],
      hints: [],
    },
  ],
};

function pendingPreview() {
  let resolve!: (value: ImportResult) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<ImportResult>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

async function choose(result = preview) {
  api.previewSepaImport.mockResolvedValueOnce(result);
  render(
    <ToastProvider>
      <TraegerSepaImportPage />
    </ToastProvider>,
  );
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Datei wählen" })).toBeEnabled(),
  );
  fireEvent.change(screen.getByLabelText("Datei wählen"), {
    target: {
      files: [new File(["Referenz;IBAN"], "mandate.csv", { type: "text/csv" })],
    },
  });
  await screen.findByRole("heading", { name: "Prüfen und übernehmen" });
}

function importButton() {
  return screen.queryByRole("button", { name: /\d+ Mandat.*übernehmen/ });
}

function expectNoImport() {
  const button = importButton();
  if (button) expect(button).toBeDisabled();
  expect(api.applySepaImport).not.toHaveBeenCalled();
}

describe("SEPA import approval", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    api.fetchSepaImportLog.mockResolvedValue([]);
  });

  it("invalidates old rows on remapping, keeps failure blocked and ignores stale responses", async () => {
    await choose();
    expect(importButton()).toBeEnabled();
    const old = pendingPreview();
    const current = pendingPreview();
    api.previewSepaImport
      .mockReturnValueOnce(old.promise)
      .mockReturnValueOnce(current.promise);
    fireEvent.click(screen.getByRole("combobox", { name: "IBAN" }));
    fireEvent.click(screen.getByRole("option", { name: "Anderes Konto" }));
    expectNoImport();
    fireEvent.click(screen.getByRole("combobox", { name: "IBAN" }));
    fireEvent.click(screen.getByRole("option", { name: "IBAN" }));
    await act(async () => old.resolve(preview));
    expectNoImport();
    await act(async () => current.reject(new Error("unavailable")));
    expectNoImport();
    api.previewSepaImport.mockResolvedValueOnce(preview);
    fireEvent.click(screen.getByRole("button", { name: "Erneut prüfen" }));
    await waitFor(() => expect(importButton()).toBeEnabled());
  });

  it("rechecks answers and shows an expired mandate before approval", async () => {
    await choose({
      ...preview,
      counts: { ...preview.counts, ready: 0, clarify: 1 },
      rows: [
        {
          ...preview.rows[0]!,
          status: "clarify",
          sequence: "",
          problems: [{ field: "sequence", code: "sequence_unknown" }],
        },
      ],
    });
    const check = pendingPreview();
    api.previewSepaImport.mockReturnValueOnce(check.promise);
    fireEvent.click(screen.getAllByRole("button", { name: "Nein" })[0]!);
    expect(api.previewSepaImport).toHaveBeenLastCalledWith(
      expect.objectContaining({ sequences: { 2: "first" } }),
    );
    expectNoImport();
    await act(async () =>
      check.resolve({
        ...preview,
        counts: { ...preview.counts, ready: 0, invalid: 1 },
        rows: [
          {
            ...preview.rows[0]!,
            status: "invalid",
            sequence: "first",
            problems: [{ field: "signed_on", code: "expired" }],
          },
        ],
      }),
    );
    await waitFor(() =>
      expect(screen.getAllByText("Fehlerhaft").length).toBeGreaterThan(0),
    );
    expectNoImport();
    // An answer can be corrected after the backend has classified the row.
    api.previewSepaImport.mockResolvedValueOnce(preview);
    fireEvent.click(screen.getAllByRole("button", { name: "Ja" })[0]!);
    await waitFor(() => expect(importButton()).toBeEnabled());
    api.applySepaImport.mockResolvedValue({
      ...preview,
      applied: true,
      counts: { ...preview.counts, ready: 0, imported: 1 },
    });
    fireEvent.click(importButton()!);
    await screen.findByText("1 Mandat übernommen");
    expect(api.applySepaImport).toHaveBeenCalledWith(
      expect.objectContaining({
        mapping: preview.mapping,
        sequences: { 2: "recurring" },
      }),
    );
  });
});
