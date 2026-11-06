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
