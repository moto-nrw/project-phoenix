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

  it("renders the title as a heading when asked", () => {
    render(<EmptyState titleAs="h1" title="Bitte kurz warten" />);
    expect(
      screen.getByRole("heading", { level: 1, name: "Bitte kurz warten" }),
    ).toBeInTheDocument();
  });

  it("hides the icon from assistive tech", () => {
    const { container } = render(
      <EmptyState title="Leer" icon={<svg data-testid="icon" />} />,
    );
    const iconWrapper = container.querySelector("span[aria-hidden='true']");
    expect(iconWrapper).not.toBeNull();
  });

  it("frames the dashed placeholder with title and description", () => {
    render(
      <EmptyState
        variant="dashed"
        title="Noch keine Kinder"
        description="Sie erscheinen hier nach der Anmeldung."
      />,
    );
    expect(screen.getByText("Noch keine Kinder")).toBeInTheDocument();
    expect(
      screen.getByText("Sie erscheinen hier nach der Anmeldung."),
    ).toBeInTheDocument();
  });

  it("shows the tone's icon on a medallion hidden from assistive technology", () => {
    render(
      <EmptyState
        tone="success"
        icon={<svg data-testid="medallion-icon" />}
        title="Konto erstellt"
      />,
    );
    expect(
      screen.getByTestId("medallion-icon").closest("[aria-hidden]"),
    ).toHaveAttribute("aria-hidden", "true");
  });

  it("renders a result without a title below a page head", () => {
    render(
      <EmptyState
        tone="error"
        icon={<svg />}
        description="Der Link ist abgelaufen."
      />,
    );
    expect(screen.getByText("Der Link ist abgelaufen.")).toBeInTheDocument();
    expect(screen.queryByRole("heading")).not.toBeInTheDocument();
  });

  it("renders the progress slot", () => {
    render(
      <EmptyState
        title="Konto erstellt"
        progress={<span role="progressbar" />}
      />,
    );
    expect(screen.getByRole("progressbar")).toBeInTheDocument();
  });
});
