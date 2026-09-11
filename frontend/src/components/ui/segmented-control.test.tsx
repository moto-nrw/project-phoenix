import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import { SegmentedControl } from "./segmented-control";

describe("SegmentedControl", () => {
  it("uses the centralized palette for active colored pills", () => {
    render(
      <SegmentedControl
        variant="pills"
        value="present"
        onChange={vi.fn()}
        items={[{ value: "present", label: "Vor Ort", tone: "green" }]}
      />,
    );

    expect(screen.getByRole("button", { name: "Vor Ort" })).toHaveStyle({
      backgroundColor: MOTO_COLOR_PALETTE.green.soft,
      color: MOTO_COLOR_PALETTE.green.strong,
    });
  });
});
