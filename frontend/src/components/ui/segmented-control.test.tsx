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

  it("renders symbols with the label as accessible name in iconOnly mode (#3834)", () => {
    const onChange = vi.fn();
    render(
      <SegmentedControl
        iconOnly
        ariaLabel="Ansicht"
        value="tiles"
        onChange={onChange}
        items={[
          {
            value: "tiles",
            label: "Kacheln",
            icon: <svg data-testid="grid" />,
          },
          { value: "table", label: "Liste", icon: <svg data-testid="list" /> },
        ]}
      />,
    );

    const list = screen.getByRole("button", { name: "Liste" });
    expect(list).toHaveAttribute("title", "Liste");
    expect(list).toHaveAttribute("aria-pressed", "false");
    expect(list).not.toHaveTextContent("Liste");
    expect(screen.getByTestId("list")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Kacheln" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );

    list.click();
    expect(onChange).toHaveBeenCalledWith("table");
  });
});
