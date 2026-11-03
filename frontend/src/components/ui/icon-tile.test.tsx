import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { childrenConcept } from "~/lib/concepts/children";

import { ConceptIconTile } from "./concept-icon-tile";
import { IconTile } from "./icon-tile";

describe("IconTile", () => {
  it("zeigt das übergebene Symbol", () => {
    render(<IconTile icon={<svg role="img" aria-label="Glocke" />} />);

    expect(screen.getByRole("img", { name: "Glocke" })).toBeInTheDocument();
  });

  it("reicht data- und aria-Attribute an die Kachel durch", () => {
    render(
      <IconTile
        icon={<svg aria-hidden="true" />}
        data-testid="kachel"
        aria-hidden="true"
      />,
    );

    expect(screen.getByTestId("kachel")).toHaveAttribute("aria-hidden", "true");
  });
});

describe("ConceptIconTile", () => {
  it("zeigt das Symbol des Konzepts im Ton des Konzepts", () => {
    const { container } = render(
      <ConceptIconTile concept={childrenConcept} size="section" />,
    );

    expect(
      container.querySelector('[data-moto-duotone-tone="greenVivid"]'),
    ).toBeInTheDocument();
  });

  it("übernimmt einen abweichenden Symbolton", () => {
    const { container } = render(
      <ConceptIconTile
        concept={childrenConcept}
        size="page"
        iconTone="neutral"
      />,
    );

    expect(
      container.querySelector('[data-moto-duotone-tone="neutral"]'),
    ).toBeInTheDocument();
  });
});
