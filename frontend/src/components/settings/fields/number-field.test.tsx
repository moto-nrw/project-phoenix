import { describe, it, expect, vi } from "vitest";
import { render, fireEvent } from "@testing-library/react";
import { NumberField } from "./number-field";

describe("NumberField", () => {
  it("renders with value", () => {
    const { getByRole } = render(<NumberField value={42} onChange={vi.fn()} />);
    const input = getByRole("spinbutton") as HTMLInputElement;
    expect(input.value).toBe("42");
  });

  it("calls onChange on input", () => {
    const onChange = vi.fn();
    const { getByRole } = render(
      <NumberField value={42} onChange={onChange} />,
    );
    fireEvent.change(getByRole("spinbutton"), { target: { value: "99" } });
    expect(onChange).toHaveBeenCalledWith(99);
  });

  it("allows clearing the field", () => {
    const onChange = vi.fn();
    const { getByRole } = render(
      <NumberField value={42} onChange={onChange} />,
    );
    fireEvent.change(getByRole("spinbutton"), { target: { value: "" } });
    // onChange should NOT be called with NaN — field just clears locally
    expect(onChange).not.toHaveBeenCalled();
  });

  it("calls onBlur", () => {
    const onBlur = vi.fn();
    const { getByRole } = render(
      <NumberField value={42} onChange={vi.fn()} onBlur={onBlur} />,
    );
    fireEvent.blur(getByRole("spinbutton"));
    expect(onBlur).toHaveBeenCalled();
  });

  it("respects min/max attributes", () => {
    const { getByRole } = render(
      <NumberField value={5} onChange={vi.fn()} min={0} max={10} />,
    );
    const input = getByRole("spinbutton") as HTMLInputElement;
    expect(input.min).toBe("0");
    expect(input.max).toBe("10");
  });

  it("is disabled when disabled prop is set", () => {
    const { getByRole } = render(
      <NumberField value={42} onChange={vi.fn()} disabled />,
    );
    expect((getByRole("spinbutton") as HTMLInputElement).disabled).toBe(true);
  });

  it("blurs input on Enter key", () => {
    const onBlur = vi.fn();
    const { getByRole } = render(
      <NumberField value={42} onChange={vi.fn()} onBlur={onBlur} />,
    );
    const input = getByRole("spinbutton") as HTMLInputElement;
    input.focus();
    fireEvent.keyDown(input, { key: "Enter" });
    // Enter triggers blur which calls onBlur
    expect(onBlur).toHaveBeenCalled();
  });

  it("does not blur on non-Enter key", () => {
    const onBlur = vi.fn();
    const { getByRole } = render(
      <NumberField value={42} onChange={vi.fn()} onBlur={onBlur} />,
    );
    const input = getByRole("spinbutton") as HTMLInputElement;
    input.focus();
    fireEvent.keyDown(input, { key: "Tab" });
    // Tab keyDown should not trigger blur via onKeyDown handler
    expect(onBlur).not.toHaveBeenCalled();
  });

  it("reverts display text on blur when value is NaN", () => {
    const { getByRole } = render(<NumberField value={42} onChange={vi.fn()} />);
    const input = getByRole("spinbutton") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "" } });
    fireEvent.blur(input);
    expect(input.value).toBe("42");
  });

  it("syncs display text when value prop changes", () => {
    const { getByRole, rerender } = render(
      <NumberField value={42} onChange={vi.fn()} />,
    );
    rerender(<NumberField value={99} onChange={vi.fn()} />);
    expect((getByRole("spinbutton") as HTMLInputElement).value).toBe("99");
  });
});
