import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { RoomColorPicker } from "./room-color-picker";
import { ROOM_COLOR_PALETTE } from "~/lib/location-helper";

describe("RoomColorPicker", () => {
  it("renders the 'Keine Farbe' option plus all palette swatches", () => {
    render(
      <RoomColorPicker value={null} onChange={() => {}} label="Badge-Farbe" />,
    );
    // 1 "Keine Farbe" + N palette buttons
    const buttons = screen.getAllByRole("button");
    expect(buttons).toHaveLength(ROOM_COLOR_PALETTE.length + 1);
    expect(screen.getByLabelText("Keine Farbe")).toBeDefined();
    for (const color of ROOM_COLOR_PALETTE) {
      expect(screen.getByLabelText(`Farbe ${color}`)).toBeDefined();
    }
  });

  it("marks 'Keine Farbe' as pressed when value is null", () => {
    render(
      <RoomColorPicker value={null} onChange={() => {}} label="Badge-Farbe" />,
    );
    expect(screen.getByLabelText("Keine Farbe")).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it("marks the matching swatch as pressed when value is in the palette", () => {
    const palette = ROOM_COLOR_PALETTE[0]!;
    render(
      <RoomColorPicker
        value={palette}
        onChange={() => {}}
        label="Badge-Farbe"
      />,
    );
    expect(screen.getByLabelText(`Farbe ${palette}`)).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it("calls onChange with null when 'Keine Farbe' is clicked", () => {
    const onChange = vi.fn();
    render(
      <RoomColorPicker
        value={ROOM_COLOR_PALETTE[0]}
        onChange={onChange}
        label="Badge-Farbe"
      />,
    );
    fireEvent.click(screen.getByLabelText("Keine Farbe"));
    expect(onChange).toHaveBeenCalledWith(null);
  });

  it("calls onChange with the picked color when a swatch is clicked", () => {
    const onChange = vi.fn();
    const palette = ROOM_COLOR_PALETTE[2]!;
    render(
      <RoomColorPicker value={null} onChange={onChange} label="Badge-Farbe" />,
    );
    fireEvent.click(screen.getByLabelText(`Farbe ${palette}`));
    expect(onChange).toHaveBeenCalledWith(palette);
  });

  it("does NOT auto-call onChange on mount (no auto-pick)", () => {
    const onChange = vi.fn();
    render(
      <RoomColorPicker value={null} onChange={onChange} label="Badge-Farbe" />,
    );
    expect(onChange).not.toHaveBeenCalled();
  });

  it("treats a non-palette legacy value as 'Keine Farbe' (cleared swatch selected)", () => {
    render(
      <RoomColorPicker
        value="#4F46E5"
        onChange={() => {}}
        label="Badge-Farbe"
      />,
    );
    expect(screen.getByLabelText("Keine Farbe")).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    // No palette swatch should be marked as selected
    for (const color of ROOM_COLOR_PALETTE) {
      expect(screen.getByLabelText(`Farbe ${color}`)).toHaveAttribute(
        "aria-pressed",
        "false",
      );
    }
  });

  it("never shows an 'außerhalb der Palette' hint, regardless of value", () => {
    for (const value of [null, "#4F46E5", ROOM_COLOR_PALETTE[0]] as const) {
      const { unmount } = render(
        <RoomColorPicker
          value={value}
          onChange={() => {}}
          label="Badge-Farbe"
        />,
      );
      expect(screen.queryByText(/außerhalb der Palette/i)).toBeNull();
      unmount();
    }
  });
});
