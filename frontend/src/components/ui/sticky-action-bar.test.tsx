import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { StickyActionBar } from "./sticky-action-bar";

describe("StickyActionBar", () => {
  it("shows the actions", () => {
    render(
      <StickyActionBar>
        <button type="button">Importieren</button>
      </StickyActionBar>,
    );
    expect(
      screen.getByRole("button", { name: "Importieren" }),
    ).toBeInTheDocument();
  });

  it("names what the actions refer to", () => {
    render(
      <StickyActionBar summary="2 Tage geändert">
        <button type="button">Speichern</button>
      </StickyActionBar>,
    );
    expect(screen.getByText("2 Tage geändert")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Speichern" }),
    ).toBeInTheDocument();
  });
});
