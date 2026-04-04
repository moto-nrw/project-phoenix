import { describe, it, expect, vi } from "vitest";
import { render, fireEvent } from "@testing-library/react";
import { BooleanField } from "./boolean-field";

describe("BooleanField", () => {
  it("renders as toggle switch", () => {
    const { getByRole } = render(
      <BooleanField value={false} onChange={vi.fn()} />,
    );
    const toggle = getByRole("switch");
    expect(toggle).toBeDefined();
    expect(toggle.getAttribute("aria-checked")).toBe("false");
  });

  it("shows checked state", () => {
    const { getByRole } = render(
      <BooleanField value={true} onChange={vi.fn()} />,
    );
    expect(getByRole("switch").getAttribute("aria-checked")).toBe("true");
  });

  it("calls onChange with toggled value", () => {
    const onChange = vi.fn();
    const { getByRole } = render(
      <BooleanField value={false} onChange={onChange} />,
    );
    fireEvent.click(getByRole("switch"));
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it("does not call onChange when disabled", () => {
    const onChange = vi.fn();
    const { getByRole } = render(
      <BooleanField value={false} onChange={onChange} disabled />,
    );
    fireEvent.click(getByRole("switch"));
    expect(onChange).not.toHaveBeenCalled();
  });
});
