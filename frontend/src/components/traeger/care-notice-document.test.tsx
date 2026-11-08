import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type {
  CareNotice,
  CareNoticeLetter,
} from "~/lib/traeger-care-notices-api";
import { CareNoticeDocument } from "./care-notice-document";

// Page 1 of the Hardt notice MK-007: 01.09.–31.12.2025, 12,00 € and
// 20,00 € a month, 48,00 € and 80,00 € for the period, 128,00 € in four
// rates of 32,00 €, September due at once, collected by direct debit.
const MK007: CareNotice = {
  id: "41",
  number: "1.2025.000041.1",
  issuedOn: "2025-09-24",
  kind: "initial",
  contractId: "71",
  schoolId: "3",
  schoolName: "Grundschule Hardt",
  childFirstName: "Lena",
  childLastName: "Muster",
  recipient: {
    firstName: "Eva",
    lastName: "Muster",
    email: "eva@example.org",
    secondName: "",
    address: { street: "Am Bach 3", postalCode: "78739", city: "Hardt" },
  },
  paymentMethod: "direct_debit",
  year: 2025,
  periodFrom: "2025-09-01",
  periodUntil: "2025-12-31",
  lines: [
    ["Verlässliche Grundschule", 1200, 4800],
    ["Nachmittagsbetreuung 13 - 16 Uhr", 2000, 8000],
  ].map(([label, monthly, annual]) => ({
    kind: "module" as const,
    label: label as string,
    detail: "an 1 Tag/Woche (Mi)",
    periodFrom: "2025-09-01",
    periodUntil: "2025-12-31",
    months: 4,
    quantity: 1,
    monthlyPreviousCents: 0,
    monthlyCents: monthly as number,
    annualPreviousCents: 0,
    annualCents: annual as number,
    deltaCents: annual as number,
  })),
  installments: ["09", "10", "11", "12"].map((month) => ({
    month: `2025-${month}-01`,
    dueOn: month === "09" ? "2025-09-24" : `2025-${month}-01`,
    immediatelyDue: month === "09",
    previousCents: 0,
    amountCents: 3200,
    deltaCents: 3200,
  })),
  previousCents: 0,
  totalCents: 12800,
  deltaCents: 12800,
  amends: null,
  amendedBy: null,
  legalRemedy:
    "Gegen diesen Bescheid können Sie innerhalb eines Monats ab Bekanntgabe Widerspruch einlegen.",
};

function letter(notice: CareNotice = MK007): CareNoticeLetter {
  return {
    notice,
    senderText: "Gemeinde Hardt, Mariazeller Str. 1, 78739 Hardt",
    footerText: "",
    contactName: "Gudrun Wimmer",
    contactPhone: "07422 1234",
    contactEmail: "info@example.org",
    accentColor: "",
    logoDataUrl: "",
  };
}

describe("CareNoticeDocument", () => {
  it("shows the most important things of MK-007 on page 1", () => {
    render(<CareNoticeDocument letter={letter()} />);

    expect(
      screen.getByRole("heading", {
        name: "Gebührenbescheid für die Betreuung 2025",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("1.2025.000041.1")).toBeInTheDocument();
    expect(
      screen.getByText("Bitte bei allen Zahlungen und Rückfragen angeben."),
    ).toBeInTheDocument();
    expect(screen.getByText("Am Bach 3")).toBeInTheDocument();

    const calculation = screen
      .getByRole("heading", { name: "I. Berechnung" })
      .closest("section") as HTMLElement;
    const lines = within(within(calculation).getByRole("table"));
    const first = lines
      .getByText("Verlässliche Grundschule")
      .closest("tr") as HTMLElement;
    expect(within(first).getByText(/12,00\s€/)).toBeInTheDocument();
    expect(within(first).getAllByText(/48,00\s€/)).toHaveLength(2);
    const second = lines
      .getByText("Nachmittagsbetreuung 13 - 16 Uhr")
      .closest("tr") as HTMLElement;
    expect(within(second).getByText(/20,00\s€/)).toBeInTheDocument();
    expect(
      within(calculation).getByText("Gesamtbetrag").parentElement,
    ).toHaveTextContent(/128,00\s€/);

    const payment = screen
      .getByRole("heading", { name: "II. Abrechnung" })
      .closest("section") as HTMLElement;
    expect(
      within(payment).getByText(/Bitte nicht überweisen/),
    ).toBeInTheDocument();
    const rates = within(within(payment).getByRole("table"));
    expect(rates.getAllByText(/32,00\s€/)).toHaveLength(4);
    expect(rates.getByText("sofort fällig")).toBeInTheDocument();
    expect(rates.getByText("am 01.10.2025")).toBeInTheDocument();

    expect(
      screen.getByRole("heading", { name: "Rechtsbehelfsbelehrung" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/Widerspruch einlegen/)).toBeInTheDocument();
  });

  it("shows before, now and the difference on a change notice", () => {
    render(
      <CareNoticeDocument
        letter={letter({
          ...MK007,
          kind: "change",
          number: "",
          issuedOn: null,
          amends: {
            id: "41",
            number: "1.2025.000041.1",
            issuedOn: "2025-09-24",
          },
          previousCents: 12800,
          totalCents: 14400,
          deltaCents: 1600,
        })}
      />,
    );

    expect(
      screen.getByRole("heading", {
        name: "Änderungsbescheid für die Betreuung 2025",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Ändert den Bescheid Nr. 1.2025.000041.1 vom 24.09.2025.",
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Monat bisher").length).toBeGreaterThan(0);
    expect(screen.getByText("Bisher festgesetzt")).toBeInTheDocument();
    expect(screen.getByText(/\+16,00\s€/)).toBeInTheDocument();
    expect(screen.getByText("wird vergeben")).toBeInTheDocument();
  });
});
