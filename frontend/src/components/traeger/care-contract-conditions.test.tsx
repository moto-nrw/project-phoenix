import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { OfferingTemplate } from "~/lib/traeger-care-contracts-api";

const { api } = vi.hoisted(() => ({
  api: { saveContractTemplate: vi.fn() },
}));

vi.mock("~/lib/traeger-care-contracts-api", () => api);

import { CareContractConditions } from "./care-contract-conditions";

const OPEN: OfferingTemplate = {
  schoolId: "3",
  schoolName: "OGS Am Lindenhof",
  offeringId: "5",
  offeringName: "Offene Ganztagsschule",
  phaseName: "2027/28",
  priceCents: 9500,
  active: true,
  template: null,
};

describe("CareContractConditions", () => {
  beforeEach(() => api.saveContractTemplate.mockReset());

  it("shows the saved enrollment text read-only, separate from unsaved conditions", () => {
    render(
      <ToastProvider>
        <CareContractConditions
          offerings={[
            {
              ...OPEN,
              acceptanceText: "Vollständige gespeicherte Bedingungen",
              template: {
                id: "7",
                updatedAt: "2026-09-09T10:00:00Z",
                termKind: "school_year",
                termMonths: null,
                noticeMonths: 1,
                noticeTo: "month_end",
                contractPartner: "Träger",
                specialTermination: false,
                specialTerminationNote: "",
                statuteRequired: false,
                statuteTitle: "",
                statuteUrl: "",
              },
            },
          ]}
          onSaved={vi.fn()}
        />
      </ToastProvider>,
    );
    const text = screen.getByRole("textbox", {
      name: "Gespeicherter Text für das Anmeldeformular",
    });
    expect(text).toHaveAttribute("readonly");
    expect(text).toHaveValue("Vollständige gespeicherte Bedingungen");
    fireEvent.change(screen.getByLabelText("Vertragspartner"), {
      target: { value: "Anderer Träger" },
    });
    expect(text).toHaveValue("Vollständige gespeicherte Bedingungen");
  });

  it("selects a fixed term through the shared width-constrained selector", async () => {
    const onSaved = vi.fn();
    api.saveContractTemplate.mockResolvedValue(OPEN);
    render(
      <ToastProvider>
        <CareContractConditions offerings={[OPEN]} onSaved={onSaved} />
      </ToastProvider>,
    );
    fireEvent.click(screen.getByRole("combobox", { name: "Laufzeit" }));
    fireEvent.click(screen.getByRole("option", { name: "Feste Monate" }));
    fireEvent.change(screen.getByLabelText("Anzahl der Monate"), {
      target: { value: "6" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Bedingungen speichern" }),
    );
    await waitFor(() =>
      expect(api.saveContractTemplate).toHaveBeenCalledWith(
        "3",
        "5",
        expect.objectContaining({ termKind: "fixed_months", termMonths: 6 }),
      ),
    );
  });

  it("opens the first offering without conditions and saves them", async () => {
    const onSaved = vi.fn();
    api.saveContractTemplate.mockImplementation(
      (_school: string, _offering: string, terms: object) =>
        Promise.resolve({
          ...OPEN,
          template: { ...terms, id: "1", updatedAt: "" },
        }),
    );
    render(
      <ToastProvider>
        <CareContractConditions offerings={[OPEN]} onSaved={onSaved} />
      </ToastProvider>,
    );

    expect(screen.getByText("Fehlt")).toBeInTheDocument();
    fireEvent.click(
      screen.getByText("Eltern dürfen in besonderen Fällen früher kündigen"),
    );
    fireEvent.change(screen.getByLabelText(/Wann ist das möglich/), {
      target: { value: "Bei Umzug" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Bedingungen speichern" }),
    );

    await waitFor(() =>
      expect(api.saveContractTemplate).toHaveBeenCalledWith(
        "3",
        "5",
        expect.objectContaining({
          termKind: "school_year",
          noticeMonths: 1,
          specialTermination: true,
          specialTerminationNote: "Bei Umzug",
        }),
      ),
    );
    expect(onSaved).toHaveBeenCalled();
  });
});
