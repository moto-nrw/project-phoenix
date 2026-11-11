import { render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { DatevRefusedNotice } from "./finance-export-datev";

vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

it.each(["care", "holiday"] as const)(
  "explains manual DATEV booking for frozen accounts of a %s run",
  (runKind) => {
    render(
      <DatevRefusedNotice
        runKind={runKind}
        runLabel="Lauf Oktober 2026"
        problems={[
          {
            documentNumber: "MB-1",
            field: "revenue_account",
            reason: "invalid",
          },
          { documentNumber: "MB-2", field: "cost_center", reason: "missing" },
        ]}
        onDismiss={vi.fn()}
      />,
    );

    expect(screen.getByText("Kostenstelle fehlt: MB-2")).toBeInTheDocument();
    expect(
      screen.getByText(/Auch ein Storno übernimmt diese Werte/),
    ).toHaveTextContent(
      "Laden Sie dann die Sollstellung dieses Laufs herunter.",
    );
    expect(screen.getByText(/manuell in DATEV buchen/)).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Konten und Kostenstellen" }),
    ).toHaveAttribute("href", "/traeger/abrechnung/exporte/konten");
  },
);

it("uses the manual booking path when account and debtor errors coincide", () => {
  render(
    <DatevRefusedNotice
      runKind="care"
      runLabel="Lauf Oktober 2026"
      problems={[
        { documentNumber: "MB-1", field: "cost_center", reason: "missing" },
        { documentNumber: "MB-2", field: "debtor_number", reason: "missing" },
      ]}
      onDismiss={vi.fn()}
    />,
  );

  expect(screen.getByText("Debitorennummer fehlt: MB-2")).toBeInTheDocument();
  expect(screen.getByText(/manuell in DATEV buchen/)).toHaveTextContent(
    "Der DATEV-Download dieses Laufs bleibt dann gesperrt.",
  );
  expect(
    screen.queryByText(/Danach ist der Lauf wieder frei/),
  ).not.toBeInTheDocument();
});
