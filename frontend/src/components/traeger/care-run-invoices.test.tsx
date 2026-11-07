import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { CareDuplicate } from "~/lib/traeger-care-billing-api";

vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path,
}));

import { CareDuplicatesCard } from "./care-run-invoices";

const CONTRACTS = [
  { contractId: "11", schoolId: "3", schoolName: "OGS Am Lindenhof" },
  { contractId: "12", schoolId: "3", schoolName: "OGS Am Lindenhof" },
];

function duplicate(weekday: CareDuplicate["weekday"]): CareDuplicate {
  return {
    childFirstName: "Mia",
    childLastName: "Beispiel",
    moduleName: "Betreuung bis 14:00 Uhr",
    weekday,
    contracts: CONTRACTS,
  };
}

describe("CareDuplicatesCard", () => {
  it("shows a child with two contracts once, with a way to each contract", () => {
    render(
      <CareDuplicatesCard duplicates={[duplicate("mon"), duplicate("tue")]} />,
    );
    expect(
      screen.getByRole("heading", { name: "Doppelter Vertrag – bitte prüfen" }),
    ).toBeInTheDocument();
    const row = screen.getByText("Mia Beispiel").closest("li");
    expect(row).not.toBeNull();
    const item = within(row as HTMLElement);
    expect(
      item.getByText("Betreuung bis 14:00 Uhr: Montag, Dienstag"),
    ).toBeInTheDocument();
    expect(
      item.getAllByRole("link").map((link) => link.getAttribute("href")),
    ).toEqual(["/traeger/vertraege/11", "/traeger/vertraege/12"]);
    expect(screen.getByText("1 Kind")).toBeInTheDocument();
  });

  it("names the children a released run left out", () => {
    render(<CareDuplicatesCard duplicates={[duplicate("mon")]} released />);
    expect(
      screen.getByRole("heading", {
        name: "Nicht abgerechnet – Doppelter Vertrag",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Sie sind in diesem Lauf nicht abgerechnet/),
    ).toBeInTheDocument();
  });

  it("shows nothing without a duplicate", () => {
    const { container } = render(
      <div data-testid="anchor">
        <CareDuplicatesCard duplicates={[]} />
      </div>,
    );
    expect(screen.getByTestId("anchor")).toBeInTheDocument();
    expect(container.querySelector("li")).toBeNull();
  });
});
