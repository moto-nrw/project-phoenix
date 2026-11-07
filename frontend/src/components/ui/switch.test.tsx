import { describe, it, expect, vi } from "vitest";
import { render, fireEvent, screen } from "@testing-library/react";
import { Switch, SwitchRow } from "./switch";

describe("Switch", () => {
  it("renders as toggle switch", () => {
    const { getByRole } = render(
      <Switch checked={false} onCheckedChange={vi.fn()} ariaLabel="Schalter" />,
    );
    const toggle = getByRole("switch");
    expect(toggle).toBeDefined();
    expect(toggle.getAttribute("aria-checked")).toBe("false");
  });

  it("shows checked state", () => {
    const { getByRole } = render(
      <Switch checked onCheckedChange={vi.fn()} ariaLabel="Schalter" />,
    );
    expect(getByRole("switch").getAttribute("aria-checked")).toBe("true");
  });

  it("calls onCheckedChange with toggled value", () => {
    const onCheckedChange = vi.fn();
    const { getByRole } = render(
      <Switch
        checked={false}
        onCheckedChange={onCheckedChange}
        ariaLabel="Schalter"
      />,
    );
    fireEvent.click(getByRole("switch"));
    expect(onCheckedChange).toHaveBeenCalledWith(true);
  });

  it("does not call onCheckedChange when disabled", () => {
    const onCheckedChange = vi.fn();
    const { getByRole } = render(
      <Switch
        checked={false}
        onCheckedChange={onCheckedChange}
        ariaLabel="Schalter"
        disabled
      />,
    );
    fireEvent.click(getByRole("switch"));
    expect(onCheckedChange).not.toHaveBeenCalled();
  });

  it("carries its name from aria-label", () => {
    render(
      <Switch checked onCheckedChange={vi.fn()} ariaLabel="Benachrichtigung" />,
    );
    expect(
      screen.getByRole("switch", { name: "Benachrichtigung" }),
    ).toBeInTheDocument();
  });
});

describe("SwitchRow", () => {
  it("names the switch by its label and describes it by the description", () => {
    render(
      <SwitchRow
        label="Halber Tag"
        description="Nur ein halber Arbeitstag."
        checked={false}
        onCheckedChange={vi.fn()}
      />,
    );
    const toggle = screen.getByRole("switch", { name: "Halber Tag" });
    expect(toggle).toHaveAccessibleDescription("Nur ein halber Arbeitstag.");
  });

  it("toggles when the label is clicked", () => {
    const onCheckedChange = vi.fn();
    render(
      <SwitchRow
        label="Halber Tag"
        checked={false}
        onCheckedChange={onCheckedChange}
      />,
    );
    fireEvent.click(screen.getByText("Halber Tag"));
    expect(onCheckedChange).toHaveBeenCalledWith(true);
  });

  it("does not toggle from the label while disabled", () => {
    const onCheckedChange = vi.fn();
    render(
      <SwitchRow
        label="Halber Tag"
        checked={false}
        onCheckedChange={onCheckedChange}
        disabled
      />,
    );
    fireEvent.click(screen.getByText("Halber Tag"));
    expect(onCheckedChange).not.toHaveBeenCalled();
    expect(screen.getByRole("switch")).toBeDisabled();
  });

  it("keeps a stable name through ariaLabel when the label follows the state", () => {
    render(
      <SwitchRow
        label="Verborgen"
        ariaLabel="Sichtbarkeit umschalten"
        checked={false}
        onCheckedChange={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("switch", { name: "Sichtbarkeit umschalten" }),
    ).toBeInTheDocument();
  });
});
