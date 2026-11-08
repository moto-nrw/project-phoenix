import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { CareDuplicate } from "~/lib/traeger-care-billing-api";

vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path,
}));

import { CareDuplicatesCard } from "./care-run-invoices";

const FIRST = {
  contractId: "11",
  schoolId: "3",
  schoolName: "OGS Am Lindenhof",
};
const SECOND = { ...FIRST, contractId: "12" };
const THIRD = { ...FIRST, contractId: "13" };
const CONTRACTS = [FIRST, SECOND];

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

  it("shows a child with three contracts once, each contract once", () => {
    const pair = (a: number, b: number): CareDuplicate => ({
      ...duplicate("mon"),
      contracts: [FIRST, SECOND, THIRD].filter((_, i) => i === a || i === b),
    });
    render(
      <CareDuplicatesCard duplicates={[pair(0, 1), pair(1, 2), pair(0, 2)]} />,
    );
    expect(screen.getByText("1 Kind")).toBeInTheDocument();
    expect(screen.getAllByRole("listitem")).toHaveLength(1);
    expect(
      screen.getAllByRole("link").map((link) => link.getAttribute("href")),
    ).toEqual([
      "/traeger/vertraege/11",
      "/traeger/vertraege/12",
      "/traeger/vertraege/13",
    ]);
  });

  it("shows a child a released run held back with its stored contracts", () => {
    render(
      <CareDuplicatesCard
        released
        duplicates={[
          {
            childFirstName: "Mia",
            childLastName: "Beispiel",
            moduleName: "",
            weekday: "mon",
            contracts: [FIRST, SECOND, THIRD],
          },
        ]}
      />,
    );
    expect(
      screen.getByText("In diesem Lauf nicht abgerechnet."),
    ).toBeInTheDocument();
    expect(
      screen.getAllByRole("link").map((link) => link.getAttribute("href")),
    ).toEqual([
      "/traeger/vertraege/11",
      "/traeger/vertraege/12",
      "/traeger/vertraege/13",
    ]);
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
