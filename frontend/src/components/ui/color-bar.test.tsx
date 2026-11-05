import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";

import { ColorBar } from "./color-bar";

describe("ColorBar", () => {
  it("shows the given colour and stays hidden from screen readers", () => {
    const { container } = render(<ColorBar color="#3F6F12" />);

    const bar = container.querySelector('[aria-hidden="true"]');
    expect(bar).toHaveStyle({ backgroundColor: "#3F6F12" });
  });

  it("falls back to the neutral edge without a colour", () => {
    const { container } = render(<ColorBar />);

    const bar = container.querySelector('[aria-hidden="true"]');
    expect(bar).toHaveStyle({ backgroundColor: "#D1D5DB" });
  });
});
