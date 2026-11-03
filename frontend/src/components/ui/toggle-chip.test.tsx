import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ToggleChip } from "./toggle-chip";

describe("ToggleChip", () => {
  it("keeps a 44 px touch target for coarse pointers", () => {
    render(
      <ToggleChip pressed={false} onPressedChange={vi.fn()}>
        Krankheit
      </ToggleChip>,
    );

    const classes = screen.getByRole("button", { name: "Krankheit" }).className;
    expect(classes).toContain("h-10");
    expect(classes).toContain("pointer-coarse:min-h-11");
  });
});
