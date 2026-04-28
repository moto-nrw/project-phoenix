import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { RoomColorField } from "./room-color-field";
import { LOCATION_COLORS } from "@/lib/location-helper";

describe("RoomColorField", () => {
  it("renders the configured hex when value is set", () => {
    render(
      <RoomColorField
        value="#a3d977"
        onChange={vi.fn()}
        label="Farbe"
        required={false}
      />,
    );
    // Display value is uppercased so audit + uniqueness checks stay consistent.
    expect(screen.getByText("#A3D977")).toBeInTheDocument();
  });

  it("shows the Standard placeholder when no value is set", () => {
    render(<RoomColorField value={null} onChange={vi.fn()} label="Farbe" />);
    expect(screen.getByText("Standard")).toBeInTheDocument();
    // Reset button only appears when a value is set
    expect(screen.queryByRole("button", { name: /Zurücksetzen/i })).toBeNull();
  });

  it("uppercases the hex on change", () => {
    const onChange = vi.fn();
    render(<RoomColorField value={null} onChange={onChange} label="Farbe" />);
    const input = screen.getByLabelText("Farbe") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "#a3d977" } });
    expect(onChange).toHaveBeenCalledWith("#A3D977");
  });

  it("clears the value to null when Zurücksetzen is clicked", () => {
    const onChange = vi.fn();
    render(
      <RoomColorField value="#A3D977" onChange={onChange} label="Farbe" />,
    );
    fireEvent.click(screen.getByRole("button", { name: /Zurücksetzen/i }));
    expect(onChange).toHaveBeenCalledWith(null);
  });

  it("uses the OTHER_ROOM blue as the default picker preview", () => {
    render(<RoomColorField value={null} onChange={vi.fn()} label="Farbe" />);
    const input = screen.getByLabelText("Farbe") as HTMLInputElement;
    // The picker shows the fallback blue when nothing is set so admins see
    // the same color the badge would render today.
    expect(input.value.toLowerCase()).toBe(
      LOCATION_COLORS.OTHER_ROOM.toLowerCase(),
    );
  });
});
