import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { ImportResult, ImportRow } from "~/lib/traeger-sepa-import-api";
import { SepaImportPreviewCard } from "./sepa-import-preview-card";

import { importableCount } from "./sepa-import-format";

function row(line: number, overrides: Partial<ImportRow>): ImportRow {
  return {
    line,
    status: "ready",
    reference: `OGS-${line}`,
    signedOn: "2024-08-15",
    ibanMasked: "DE•• •••• •••• •••• ••30 00",
    bic: "",
    accountHolder: "Eva Beispiel",
    contractId: "7",
    payerFirstName: "Eva",
    payerLastName: `Beispiel ${line}`,
    sequence: "recurring",
    lastCollectedOn: "2026-10-01",
    mandateId: null,
    problems: [],
    hints: [],
    debtorNumber: "",
    debtorNumberStatus: "",
    ...overrides,
  };
}

const result: ImportResult = {
  encoding: "windows-1252",
  delimiter: ";",
  headers: ["Mandatsreferenz"],
  mapping: {
    columns: {
      reference: "Mandatsreferenz",
      signed_on: "",
      iban: "",
      bic: "",
      account_holder: "",
      payer: "",
      creditor_id: "",
      sequence: "",
      last_collected_on: "",
      debtor_number: "",
    },
    payerKey: "email",
    creditorFromTraeger: true,
    confirmedCreditorId: "DE98ZZZ09999999999",
  },
  unmapped: [],
  creditorId: "DE98ZZZ09999999999",
  fingerprint: "preview-1",
  applied: false,
  counts: {
    ready: 1,
    clarify: 1,
    duplicate: 0,
    conflict: 0,
    invalid: 1,
    imported: 0,
    debtorNumbers: 0,
    debtorConflicts: 0,
    withoutMandate: 0,
  },
  rows: [
    row(2, {}),
    row(3, {
      status: "clarify",
      sequence: "",
      lastCollectedOn: null,
      problems: [{ field: "sequence", code: "sequence_unknown" }],
    }),
    row(4, {
      status: "invalid",
      ibanMasked: "",
      payerFirstName: "",
      payerLastName: "",
      problems: [{ field: "iban", code: "invalid" }],
    }),
  ],
};

describe("SepaImportPreviewCard", () => {
  it("counts only rows the backend has marked ready", () => {
    expect(importableCount(result)).toBe(1);
  });

  it("asks the open row and names the problem without the value", () => {
    const onAnswer = vi.fn();
    render(
      <SepaImportPreviewCard
        result={result}
        answers={{}}
        applying={false}
        onAnswer={onAnswer}
        onApply={vi.fn()}
      />,
    );
    expect(
      screen.getAllByText("Die Datei sagt nicht, ob schon abgebucht wurde.")
        .length,
    ).toBeGreaterThan(0);
    expect(screen.getAllByText("IBAN stimmt nicht.").length).toBeGreaterThan(0);
    expect(
      screen.getByRole("button", { name: "1 Mandat übernehmen" }),
    ).toBeEnabled();
    expect(screen.getByText(/Noch 1 Zeile offen/)).toBeInTheDocument();

    const question = screen.getAllByRole("group", {
      name: "Zeile 3: Wurde schon abgebucht?",
    })[0];
    expect(question).toBeDefined();
    fireEvent.click(screen.getAllByRole("button", { name: "Ja" })[0]!);
    expect(onAnswer).toHaveBeenCalledWith(3, "recurring");
  });

  it("counts the rows without a mandate once, not as errors (#7009)", () => {
    render(
      <SepaImportPreviewCard
        result={{
          ...result,
          counts: { ...result.counts, withoutMandate: 517 },
        }}
        answers={{}}
        applying={false}
        onAnswer={vi.fn()}
        onApply={vi.fn()}
      />,
    );
    expect(
      screen.getByText(/^517 Zeilen ohne Mandat: .* Das ist kein Fehler\.$/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Fehler 1" }),
    ).toBeInTheDocument();
  });

  it("names the debtor number siblings disagree on (#4693)", () => {
    render(
      <SepaImportPreviewCard
        result={{
          ...result,
          rows: [
            row(2, {
              debtorNumber: "20 0001",
              debtorNumberStatus: "siblings_differ",
            }),
            row(3, { debtorNumber: "10 0001", debtorNumberStatus: "new" }),
          ],
        }}
        answers={{}}
        applying={false}
        onAnswer={vi.fn()}
        onApply={vi.fn()}
      />,
    );
    expect(
      screen.getAllByText(/Geschwister haben verschiedene Nummern/).length,
    ).toBeGreaterThan(0);
    expect(
      screen.getAllByText("Debitorennummer 10 0001 wird eingetragen.").length,
    ).toBeGreaterThan(0);
  });

  it("allows importing only missing debtor numbers for existing mandates", () => {
    const onApply = vi.fn();
    const debtorsOnly = {
      ...result,
      counts: {
        ...result.counts,
        ready: 0,
        clarify: 0,
        invalid: 0,
        duplicate: 2,
        debtorNumbers: 1,
        debtorConflicts: 0,
      },
      rows: [
        row(2, {
          status: "duplicate",
          debtorNumber: "20 0001",
          debtorNumberStatus: "new",
        }),
        row(3, {
          status: "duplicate",
          debtorNumber: "20 0001",
          debtorNumberStatus: "new",
        }),
      ],
    };
    render(
      <SepaImportPreviewCard
        result={debtorsOnly}
        answers={{}}
        applying={false}
        onAnswer={vi.fn()}
        onApply={onApply}
      />,
    );
    const button = screen.getByRole("button", {
      name: "1 Debitorennummer übernehmen",
    });
    expect(button).toBeEnabled();
    fireEvent.click(button);
    expect(onApply).toHaveBeenCalledOnce();
  });

  it("filters the rows with problems", () => {
    render(
      <SepaImportPreviewCard
        result={result}
        answers={{ 3: "first" }}
        applying={false}
        onAnswer={vi.fn()}
        onApply={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", { name: "1 Mandat übernehmen" }),
    ).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: "Fehler 1" }));
    expect(screen.getAllByText("Fehlerhaft").length).toBeGreaterThan(0);
    expect(screen.queryAllByText("Beispiel 2")).toHaveLength(0);
    expect(screen.getAllByText("Eva Beispiel").length).toBeGreaterThan(0);
  });
});
