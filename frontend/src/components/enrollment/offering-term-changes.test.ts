import { describe, expect, it } from "vitest";

import type { AdminRequestChildOffering } from "~/lib/enrollment-admin-api";

import { submittedTermsMessage } from "./offering-term-changes";

function offering(
  overrides: Partial<AdminRequestChildOffering>,
): AdminRequestChildOffering {
  return {
    offering_id: "7",
    offering_name: "Ganztag",
    days_of_week_mode: "fixed",
    submitted_terms: {
      name: "Ganztag",
      days_of_week_mode: "fixed",
      available_days: ["mon", "tue"],
    },
    ...overrides,
  };
}

describe("submittedTermsMessage", () => {
  it("stays empty without a change", () => {
    expect(submittedTermsMessage(offering({ changed_terms: [] }))).toBe("");
  });

  it("names a removed price and a changed activity", () => {
    expect(
      submittedTermsMessage(offering({ changed_terms: ["price", "activity"] })),
    ).toBe(
      "Seit der Anmeldung geändert. Die Eltern haben gewählt: ohne Preis. Das Angebot gehört jetzt zu einer anderen Aktivität.",
    );
  });

  it("names the days a parent could choose from", () => {
    expect(
      submittedTermsMessage(
        offering({
          changed_terms: ["days"],
          submitted_terms: {
            name: "Ganztag",
            days_of_week_mode: "parent_choice",
            available_days: ["mon", "fri"],
          },
        }),
      ),
    ).toBe(
      "Seit der Anmeldung geändert. Die Eltern haben gewählt: Tage zur Wahl: Mo, Fr.",
    );
  });
});
