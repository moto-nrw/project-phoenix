import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { SepaCreditor } from "~/lib/traeger-sepa-api";
import { SepaCreditorCard } from "./sepa-creditor-card";

// The Träger host is configuration; the path is what the card decides.
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

const creditor: SepaCreditor = {
  creditorId: "DE98ZZZ09999999999",
  prenotificationDays: 14,
  missingMandate: "transfer",
  updatedAt: null,
};

function renderCard(onlineMandate: boolean | null) {
  return render(
    <SepaCreditorCard
      creditor={creditor}
      onlineMandate={onlineMandate}
      onChanged={vi.fn()}
    />,
    { wrapper: ToastProvider },
  );
}

// The switch of the mandate in the parents portal (#4696) shows its state
// and leads to the setting; unknown, it shows nothing.
describe("SepaCreditorCard online mandate", () => {
  it("shows the switch on and where to change it", () => {
    renderCard(true);
    expect(screen.getByText("Eingeschaltet")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "In den Einstellungen ändern" }),
    ).toHaveAttribute(
      "href",
      expect.stringContaining("einstellungen?bereich=monatsbeitrag"),
    );
  });

  it("names the paper way while the switch is off", () => {
    renderCard(false);
    expect(screen.getByText("Ausgeschaltet")).toBeInTheDocument();
    expect(
      screen.getByText(/Eltern geben das Mandat auf Papier ab/),
    ).toBeInTheDocument();
  });

  it("shows nothing about it while the state is unknown", () => {
    renderCard(null);
    expect(screen.getByText("DE98ZZZ09999999999")).toBeInTheDocument();
    expect(screen.queryByText("Mandat in der Eltern-App")).toBeNull();
  });
});
