import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { CareRule, CareRuleSet } from "~/lib/traeger-care-reductions-api";
import { CareRuleModal } from "./care-rule-modal";

const { saveCareRule } = vi.hoisted(() => ({ saveCareRule: vi.fn() }));
vi.mock("~/lib/traeger-care-reductions-api", () => ({ saveCareRule }));

const rule: CareRule = {
  id: "7",
  kind: "sibling",
  name: "Geschwister",
  validFrom: "2026-09-01",
  validUntil: null,
  schoolIds: ["11"],
  careOfferingIds: ["999"],
  countSchoolIds: [],
  siblingFromChild: 2,
  discountBasisPoints: 5000,
  discountCents: null,
  priceCents: null,
  capCents: null,
};
const data: CareRuleSet = {
  rules: [rule],
  schools: [{ id: "11", name: "Schule" }],
  modules: [],
};

describe("CareRuleModal", () => {
  it("preserves module restrictions missing from the current choices when renaming", async () => {
    saveCareRule.mockResolvedValue(data);
    render(
      <ToastProvider>
        <CareRuleModal
          data={data}
          open={{ kind: "sibling", rule }}
          currentMonth="2026-09-01"
          onClose={vi.fn()}
          onSaved={vi.fn()}
        />
      </ToastProvider>,
    );
    fireEvent.change(screen.getByLabelText(/Name auf der Rechnung/), {
      target: { value: "Geschwister neu" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Regel speichern" }));
    await waitFor(() =>
      expect(saveCareRule).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          name: "Geschwister neu",
          careOfferingIds: ["999"],
        }),
      ),
    );
  });
});
