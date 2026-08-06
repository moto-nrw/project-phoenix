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
});
