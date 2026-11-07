import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { VisibilityBadge } from "./visibility-badge";

describe("VisibilityBadge", () => {
  it("names the parents as the default audience", () => {
    render(<VisibilityBadge hint="Eltern sehen das." />);

    expect(screen.getByText("Für Eltern sichtbar")).toBeInTheDocument();
    expect(screen.getByRole("tooltip")).toHaveTextContent("Eltern sehen das.");
  });

  it("names the OGS with the translated label of the parents portal", () => {
    render(
      <VisibilityBadge
        audience="ogs"
        label="Visible to the OGS"
        hint="Your OGS can see this."
      />,
    );

    expect(screen.getByText("Visible to the OGS")).toBeInTheDocument();
    expect(screen.getByRole("tooltip")).toHaveTextContent(
      "Your OGS can see this.",
    );
  });

  it("keeps the words for assistive technology in the compact marker", () => {
    render(<VisibilityBadge audience="ogs" compact hint="Hinweis" />);

    expect(screen.getByText("Für die OGS sichtbar")).toHaveClass("sr-only");
  });
});
