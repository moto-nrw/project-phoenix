import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { PageIntro } from "./page-intro";

describe("PageIntro", () => {
  it("rendert Kicker, h1-Titel, Text und Aktionen in einer Karte", () => {
    render(
      <PageIntro
        kicker="Elternportal"
        title="Guten Tag, Sabine"
        description="Das Wichtigste auf einen Blick."
        actions={<button type="button">Aktion</button>}
      />,
    );
    expect(screen.getByText("Elternportal")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { level: 1, name: "Guten Tag, Sabine" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Das Wichtigste auf einen Blick."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Aktion" })).toBeInTheDocument();
  });

  it("hebt den Titel mit prominent eine Stufe an", () => {
    render(<PageIntro title="Guten Morgen" prominent />);
    expect(screen.getByRole("heading", { level: 1 })).toHaveClass("text-2xl");
  });

  it("nutzt ohne prominent die normale Seitentitelgröße", () => {
    render(<PageIntro title="Nachrichten" />);
    expect(screen.getByRole("heading", { level: 1 })).toHaveClass("text-xl");
  });
});
