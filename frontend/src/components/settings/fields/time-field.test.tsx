import { describe, it, expect, vi } from "vitest";
import { render, fireEvent } from "@testing-library/react";
import { TimeField } from "./time-field";

describe("TimeField", () => {
  it("renders with value", () => {
    const { container } = render(
      <TimeField value="18:00" onChange={vi.fn()} />,
    );
    const input = container.querySelector(
      "input[type='time']",
    ) as HTMLInputElement;
    expect(input.value).toBe("18:00");
  });

  it("calls onChange on input", () => {
    const onChange = vi.fn();
    const { container } = render(
      <TimeField value="18:00" onChange={onChange} />,
    );
    const input = container.querySelector(
      "input[type='time']",
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "16:30" } });
    expect(onChange).toHaveBeenCalledWith("16:30");
  });

  it("calls onBlur", () => {
    const onBlur = vi.fn();
    const { container } = render(
      <TimeField value="18:00" onChange={vi.fn()} onBlur={onBlur} />,
    );
    const input = container.querySelector(
      "input[type='time']",
    ) as HTMLInputElement;
    fireEvent.blur(input);
    expect(onBlur).toHaveBeenCalled();
  });
});
