import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ClosingDayChip } from "./closing-day-marker";

describe("ClosingDayChip", () => {
  it("shows label and reason in the kit pill, full text in the tooltip", () => {
    render(<ClosingDayChip reason="Brückentag" />);

    const pill = screen.getByText("Schließtag · Brückentag").closest("[title]");
    expect(pill).toHaveAttribute("title", "Schließtag: Brückentag");
    expect(pill).toHaveClass("rounded-full");
    expect(pill).toHaveClass("text-xs");
  });

  it("keeps the full statement for screen readers behind a short text", () => {
    render(<ClosingDayChip label="3 Schließtage" reason="Ferien" text="3" />);

    expect(screen.getByText("3 Schließtage: Ferien")).toHaveClass("sr-only");
    expect(screen.getByText("3")).toBeVisible();
  });

  it("breaks a long reason instead of cutting it when asked to wrap", () => {
    render(<ClosingDayChip reason="Pädagogischer Tag" wrap />);

    const text = screen.getByText("Schließtag · Pädagogischer Tag");
    expect(text).toHaveClass("whitespace-normal");
  });

  it("hands the spacing of the caller to its wrapper", () => {
    const { container } = render(
      <ClosingDayChip reason="Brückentag" className="mt-1.5" />,
    );

    expect(container.firstElementChild).toHaveClass("mt-1.5");
  });
});
