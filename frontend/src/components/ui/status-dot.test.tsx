import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";

import { StatusDot } from "./status-dot";

describe("StatusDot", () => {
  it("names a dot without text beside it for screen readers", () => {
    render(<StatusDot color="#83CD2D" label="Läuft" />);

    const label = screen.getByText("Läuft");
    expect(label).toHaveClass("sr-only");
    expect(label.parentElement).toHaveStyle({ backgroundColor: "#83CD2D" });
  });

  it("hides a dot from screen readers when the text beside it says the same", () => {
    const { container } = render(
      <p>
        <StatusDot color="#83CD2D" decorative />
        Raum fehlt
      </p>,
    );

    expect(container.querySelector("span")).toHaveAttribute(
      "aria-hidden",
      "true",
    );
  });

  it("does not compile without a label or the decorative mark", () => {
    // @ts-expect-error: ein Punkt ohne Text braucht ein Label (Entscheidung 5)
    const dot = <StatusDot color="#83CD2D" />;

    expect(dot).toBeDefined();
  });

  it("does not pulse", () => {
    render(<StatusDot className="bg-moto-green" label="Läuft" />);

    expect(screen.getByText("Läuft").parentElement).not.toHaveClass(
      "animate-pulse",
    );
  });
});
