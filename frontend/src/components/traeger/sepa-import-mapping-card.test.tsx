import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { ImportMapping } from "~/lib/traeger-sepa-import-api";
import { SepaImportMappingCard } from "./sepa-import-mapping-card";

const mapping: ImportMapping = {
  columns: {
    reference: "Mandatsreferenz",
    signed_on: "Datum",
    iban: "IBAN",
    bic: "",
    account_holder: "Kontoinhaber",
    payer: "E-Mail",
    creditor_id: "",
    sequence: "",
    last_collected_on: "",
    debtor_number: "",
  },
  payerKey: "email",
  creditorFromTraeger: false,
  confirmedCreditorId: "",
};

describe("SepaImportMappingCard", () => {
  // The confirmation is bound to the identifier on the screen: when it
  // changes before the import, the backend asks again (#6198).
  it("confirms the creditor ID it shows", () => {
    const onChange = vi.fn();
    render(
      <SepaImportMappingCard
        headers={["Mandatsreferenz", "Datum", "IBAN", "Kontoinhaber", "E-Mail"]}
        mapping={mapping}
        unmapped={["creditor_id"]}
        creditorId="DE98ZZZ09999999999"
        disabled={false}
        onChange={onChange}
      />,
    );
    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Alle Mandate gehören zu unserer Gläubiger-ID DE98ZZZ09999999999/,
      }),
    );
    expect(onChange).toHaveBeenCalledWith({
      ...mapping,
      creditorFromTraeger: true,
      confirmedCreditorId: "DE98ZZZ09999999999",
    });
  });
});
