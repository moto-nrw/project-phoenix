import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import {
  ConceptPageHeader,
  ConceptSectionHeader,
} from "./concept-section-header";

describe("ConceptSectionHeader", () => {
  it("renders the shared neutral icon surface and concept tone", () => {
    render(
      <ConceptSectionHeader
        title="Nachrichten mit den Eltern"
        concept="parentConversations"
      />,
    );

    const title = screen.getByRole("heading", {
      name: "Nachrichten mit den Eltern",
    });
    const iconSurface = title.parentElement?.previousElementSibling;

    expect(iconSurface).toHaveClass("rounded-xl", "bg-gray-100");
    expect(
      iconSurface?.querySelector('[data-moto-duotone-tone="blue"]'),
    ).toBeInTheDocument();
  });

  it("renders a page heading with the same concept mapping", () => {
    render(
      <ConceptPageHeader
        title="Nachrichten"
        eyebrow="Austausch mit der OGS"
        concept="parentConversations"
      />,
    );

    const title = screen.getByRole("heading", {
      level: 1,
      name: "Nachrichten",
    });
    const iconSurface = title.parentElement?.previousElementSibling;

    expect(screen.getByText("Austausch mit der OGS")).toBeInTheDocument();
    expect(iconSurface).toHaveClass("rounded-xl", "bg-gray-100");
    expect(
      iconSurface?.querySelector('[data-moto-duotone-tone="blue"]'),
    ).toBeInTheDocument();
  });
  it("renders h2 by default and honours an explicit level", () => {
    // Eine fest verdrahtete h2 erzeugt dort Ebenenspruenge, wo die Umgebung
    // eine andere Ebene vorgibt: im Dialog steht der Titel als h3, und
    // Abschnitte mit eigenen Unterkarten brauchen Platz fuer deren h4/h5.
    const { rerender } = render(
      <ConceptSectionHeader title="Standard" concept="schools" />,
    );
    expect(
      screen.getByRole("heading", { level: 2, name: "Standard" }),
    ).toBeInTheDocument();

    for (const level of [3, 4] as const) {
      rerender(
        <ConceptSectionHeader
          title={`Ebene ${level}`}
          concept="schools"
          level={level}
        />,
      );
      expect(
        screen.getByRole("heading", { level, name: `Ebene ${level}` }),
      ).toBeInTheDocument();
    }
  });
});
