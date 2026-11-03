import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ConceptPageHeader } from "./concept-page-header";
import { parentConversationsConcept } from "~/lib/concepts/parentConversations";

describe("ConceptPageHeader", () => {
  it("renders a page heading with the concept icon surface", () => {
    render(
      <ConceptPageHeader
        title="Nachrichten"
        eyebrow="Austausch mit der OGS"
        concept={parentConversationsConcept}
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
