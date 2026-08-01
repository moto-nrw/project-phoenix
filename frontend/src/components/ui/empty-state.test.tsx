import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { EmptyState } from "./empty-state";

describe("EmptyState", () => {
  it("renders title and description", () => {
    render(
      <EmptyState
        title="Keine Ergebnisse gefunden"
        description="Versuche einen anderen Suchbegriff."
      />,
    );
    expect(screen.getByText("Keine Ergebnisse gefunden")).toBeInTheDocument();
    expect(
      screen.getByText("Versuche einen anderen Suchbegriff."),
    ).toBeInTheDocument();
  });

  it("omits description when not provided", () => {
    const { container } = render(<EmptyState title="Leer" />);
    expect(container.querySelectorAll("p")).toHaveLength(1);
  });

  it("renders the action slot", () => {
    render(
      <EmptyState
        title="Noch keine Beiträge"
        action={<button type="button">Neuer Beitrag</button>}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Neuer Beitrag" }),
    ).toBeInTheDocument();
  });

  it("hides the icon from assistive tech", () => {
    const { container } = render(
      <EmptyState title="Leer" icon={<svg data-testid="icon" />} />,
    );
    const iconWrapper = container.querySelector("span[aria-hidden='true']");
    expect(iconWrapper).not.toBeNull();
  });
});
