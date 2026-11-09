import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { BillingChild, BillingDocument } from "~/lib/traeger-billing-api";

import { BillingCorrectionModal } from "./billing-correction-modal";

const CHILD: BillingChild = {
  childId: "5",
  firstName: "Lotta",
  lastName: "Schneider",
  positions: [],
  corrections: [],
  carriedCorrections: [],
  totalCents: 12000,
};

describe("BillingCorrectionModal credit (#6348)", () => {
  it("names the rest a claim before payment, not an arrear", () => {
    render(
      <ToastProvider>
        <BillingCorrectionModal
          runId="3"
          target={{
            kind: "credit",
            document: { id: "11", number: "FE-0003" } as BillingDocument,
            child: CHILD,
            leftCents: 11500,
          }}
          onClose={vi.fn()}
        />
      </ToastProvider>,
    );

    expect(
      screen.getByText(
        /^Offene Forderung für Lotta: 115,00\s€\. Das ist der Stand vor Zahlungseingang\./,
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Noch offen/)).not.toBeInTheDocument();
  });
});
