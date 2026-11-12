import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { ExportSettings } from "~/lib/traeger-finance-exports-api";
import { SollMappingEditor } from "./soll-mapping-editor";

const { saveSollMapping } = vi.hoisted(() => ({ saveSollMapping: vi.fn() }));
vi.mock("~/lib/traeger-finance-exports-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  saveSollMapping,
}));

const settings: ExportSettings = {
  mapping: {
    columns: [
      { field: "invoice_number", header: "Rechnungsnummer", value: "" },
    ],
    delimiter: ",",
    decimalSeparator: ",",
    dateFormat: "dd.mm.yyyy",
    encoding: "utf-8",
    headerRow: true,
    fileNamePattern: "sollstellung-{NR}.csv",
  },
  custom: true,
  updatedAt: null,
  nextFileNumber: 1,
  fileNumbersStarted: false,
  datev: { ready: true, missingSettings: [], invalidSettings: [] },
};

describe("SollMappingEditor", () => {
  it("ties the amounts error to its choice and focuses it", async () => {
    render(
      <ToastProvider>
        <SollMappingEditor
          settings={settings}
          onCancel={vi.fn()}
          onSaved={vi.fn()}
        />
      </ToastProvider>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    const hint = await screen.findByText(/Dezimalzeichen muss sich/);
    const amounts = screen.getByRole("group", { name: "Beträge" });
    expect(amounts).toHaveAttribute("aria-describedby", hint.id);
    expect(amounts).toHaveAccessibleDescription(
      "Das Dezimalzeichen muss sich vom Trennzeichen unterscheiden.",
    );
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "12,50" })).toHaveFocus(),
    );
    expect(saveSollMapping).not.toHaveBeenCalled();
  });
});
