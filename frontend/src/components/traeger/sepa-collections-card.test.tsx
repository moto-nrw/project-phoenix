import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { SepaMandate } from "~/lib/traeger-sepa-api";
import { SepaCollectionsCard } from "./sepa-collections-card";

const fetchCollections = vi.hoisted(() => vi.fn());
vi.mock("~/lib/traeger-sepa-debits-api", () => ({
  fetchSepaMandateCollections: fetchCollections,
}));

const mandate: SepaMandate = {
  id: "9",
  contractId: "42",
  familyEmail: "eva@example.org",
  partyFirstName: "Eva",
  partyLastName: "Beispiel",
  reference: "SJR-2026-0001",
  signedOn: "2026-09-01",
  kind: "recurring",
  sequence: "first",
  creditorId: "DE98ZZZ09999999999",
  creditorCurrent: true,
  accountHolder: "Eva Beispiel",
  ibanMasked: "DE•• 1234",
  bic: "",
  source: "office",
  revokedOn: null,
  lastCollectedOn: null,
  expiresOn: "2029-09-01",
  createdAt: "2026-09-01T10:00:00+02:00",
};

it("reloads collections when a mandate is revoked without changing its ID", async () => {
  const debit = {
    id: "1",
    mandateId: "9",
    contractId: "42",
    invoiceNumber: "RE-1",
    recipientName: "Eva Beispiel",
    amountCents: 1000,
    dueOn: "2026-11-21",
    collectOn: null,
    sequenceType: "FRST",
    mandateReference: mandate.reference,
    ibanMasked: mandate.ibanMasked,
    status: "pending",
    left: null,
    submittedAt: null,
  };
  fetchCollections
    .mockResolvedValueOnce([debit])
    .mockResolvedValueOnce([{ ...debit, left: "revoked" }]);
  const view = render(
    <ToastProvider>
      <SepaCollectionsCard mandates={[mandate]} />
    </ToastProvider>,
  );
  expect(await screen.findByText("Noch nicht eingereicht")).toBeInTheDocument();
  view.rerender(
    <ToastProvider>
      <SepaCollectionsCard
        mandates={[{ ...mandate, revokedOn: "2026-11-20" }]}
      />
    </ToastProvider>,
  );
  expect(
    await screen.findByText("Das Mandat ist widerrufen."),
  ).toBeInTheDocument();
});
