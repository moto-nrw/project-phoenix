import { describe, it, expect, vi } from "vitest";
import { render, fireEvent } from "@testing-library/react";
import { TextField } from "./text-field";

describe("TextField", () => {
  it("renders with value", () => {
    const { container } = render(
      <TextField value="hello" onChange={vi.fn()} />,
    );
    const input = container.querySelector(
      "input[type='text']",
    ) as HTMLInputElement;
    expect(input.value).toBe("hello");
  });

  it("calls onChange on input", () => {
    const onChange = vi.fn();
    const { container } = render(<TextField value="" onChange={onChange} />);
    const input = container.querySelector(
      "input[type='text']",
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "world" } });
    expect(onChange).toHaveBeenCalledWith("world");
  });

  it("calls onBlur", () => {
    const onBlur = vi.fn();
    const { container } = render(
      <TextField value="" onChange={vi.fn()} onBlur={onBlur} />,
    );
    fireEvent.blur(container.querySelector("input") as HTMLElement);
    expect(onBlur).toHaveBeenCalled();
  });

  it("is disabled when disabled prop is set", () => {
    const { container } = render(
      <TextField value="" onChange={vi.fn()} disabled />,
    );
    expect(
      (container.querySelector("input") as HTMLInputElement).disabled,
    ).toBe(true);
  });
});
