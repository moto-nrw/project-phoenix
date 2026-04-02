import { describe, it, expect, vi } from "vitest";
import { render, fireEvent } from "@testing-library/react";
import { PasswordField } from "./password-field";

describe("PasswordField", () => {
  it("shows masked text when value exists", () => {
    const { getByText } = render(
      <PasswordField hasValue={true} onChange={vi.fn()} />,
    );
    expect(getByText("••••••")).toBeDefined();
  });

  it("shows 'Nicht gesetzt' when no value", () => {
    const { getByText } = render(
      <PasswordField hasValue={false} onChange={vi.fn()} />,
    );
    expect(getByText("Nicht gesetzt")).toBeDefined();
  });

  it("shows 'Ändern' button when value exists", () => {
    const { getByText } = render(
      <PasswordField hasValue={true} onChange={vi.fn()} />,
    );
    expect(getByText("Ändern")).toBeDefined();
  });

  it("shows 'Setzen' button when no value", () => {
    const { getByText } = render(
      <PasswordField hasValue={false} onChange={vi.fn()} />,
    );
    expect(getByText("Setzen")).toBeDefined();
  });

  it("switches to edit mode on button click", () => {
    const { getByText, container } = render(
      <PasswordField hasValue={true} onChange={vi.fn()} />,
    );
    fireEvent.click(getByText("Ändern"));
    expect(container.querySelector("input[type='password']")).toBeDefined();
  });

  it("calls onChange and exits edit mode on save", () => {
    const onChange = vi.fn();
    const { getByText, container } = render(
      <PasswordField hasValue={false} onChange={onChange} />,
    );

    // Enter edit mode
    fireEvent.click(getByText("Setzen"));

    // Type value
    const input = container.querySelector(
      "input[type='password']",
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "1234" } });

    // Save
    fireEvent.click(getByText("Speichern"));
    expect(onChange).toHaveBeenCalledWith("1234");
  });

  it("cancels edit mode without saving", () => {
    const onChange = vi.fn();
    const { getByText } = render(
      <PasswordField hasValue={true} onChange={onChange} />,
    );

    fireEvent.click(getByText("Ändern"));
    fireEvent.click(getByText("Abbrechen"));

    expect(onChange).not.toHaveBeenCalled();
    expect(getByText("••••••")).toBeDefined();
  });
});
