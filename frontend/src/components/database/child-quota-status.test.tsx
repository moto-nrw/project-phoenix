import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ChildQuotaStatus } from "./child-quota-status";

describe("ChildQuotaStatus", () => {
  it("names the numbers as the Kinderkontingent and explains who counts", () => {
    const { container } = render(
      <ChildQuotaStatus quota={{ booked: 50, occupied: 48 }} />,
    );

    expect(container).toHaveTextContent("Kinderkontingent: 48 von 50 belegt");
    expect(container).not.toHaveTextContent("voll");
    expect(screen.getByRole("tooltip")).toHaveTextContent(
      "Ihr Vertrag erlaubt bis zu 50 Kinder. Es zählen aktive Kinder und Kinder, deren Betreuung später beginnt.",
    );
    expect(screen.getByRole("tooltip")).not.toHaveTextContent("moto-Team");
  });

  it("marks a full Kinderkontingent and says what to do", () => {
    const { container } = render(
      <ChildQuotaStatus quota={{ booked: 50, occupied: 50 }} />,
    );

    expect(container).toHaveTextContent(
      "Kinderkontingent: 50 von 50 belegt · voll",
    );
    expect(screen.getByRole("tooltip")).toHaveTextContent(
      "Für weitere Kinder melden Sie sich bitte beim moto-Team.",
    );
  });

  it("renders nothing without a Kinderkontingent", () => {
    const { container } = render(<ChildQuotaStatus quota={null} />);

    expect(container).toBeEmptyDOMElement();
  });
});
