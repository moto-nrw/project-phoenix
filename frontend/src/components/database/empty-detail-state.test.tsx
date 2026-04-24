import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { EmptyDetailState } from "./empty-detail-state";

describe("EmptyDetailState", () => {
  it("renders default German copy", () => {
    render(<EmptyDetailState />);

    expect(screen.getByText("Kein Eintrag ausgewählt")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Wähle einen Eintrag aus der Liste, um Details zu sehen.",
      ),
    ).toBeInTheDocument();
  });

  it("renders custom title and description", () => {
    render(<EmptyDetailState title="Custom" description="Custom desc" />);

    expect(screen.getByText("Custom")).toBeInTheDocument();
    expect(screen.getByText("Custom desc")).toBeInTheDocument();
  });

  it("marks the icon as decorative", () => {
    const { container } = render(<EmptyDetailState />);
    const svg = container.querySelector("svg");
    expect(svg).toHaveAttribute("aria-hidden", "true");
  });
});
