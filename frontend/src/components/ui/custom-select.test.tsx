import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { CustomSelect } from "./custom-select";

const options = [
  { value: "", label: "Bitte wählen" },
  { value: "first", label: "Erste Option" },
  { value: "second", label: "Zweite Option" },
  { value: "disabled", label: "Gesperrt", disabled: true },
] as const;

describe("CustomSelect", () => {
  it("renders the selected label", () => {
    render(
      <CustomSelect
        value="first"
        options={options}
        onChange={vi.fn()}
        ariaLabel="Auswahl"
      />,
    );

    expect(screen.getByRole("combobox", { name: "Auswahl" })).toHaveTextContent(
      "Erste Option",
    );
  });

  it("forwards ariaDescribedBy to the trigger", () => {
    render(
      <CustomSelect
        value="first"
        options={options}
        onChange={vi.fn()}
        ariaLabel="Auswahl"
        ariaDescribedBy="auswahl-error auswahl-hint"
      />,
    );

    expect(screen.getByRole("combobox", { name: "Auswahl" })).toHaveAttribute(
      "aria-describedby",
      "auswahl-error auswahl-hint",
    );
  });

  it("opens and selects an option", () => {
    const onChange = vi.fn();
    render(
      <CustomSelect
        value=""
        options={options}
        onChange={onChange}
        ariaLabel="Auswahl"
      />,
    );

    fireEvent.click(screen.getByRole("combobox", { name: "Auswahl" }));
    fireEvent.click(screen.getByRole("option", { name: "Zweite Option" }));

    expect(onChange).toHaveBeenCalledWith("second");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("keeps the listbox inside the local select container", () => {
    render(
      <div data-testid="surface" className="moto-content-surface">
        <CustomSelect
          value=""
          options={options}
          onChange={vi.fn()}
          ariaLabel="Auswahl"
        />
      </div>,
    );

    fireEvent.click(screen.getByRole("combobox", { name: "Auswahl" }));

    const listbox = screen.getByRole("listbox");
    expect(screen.getByTestId("surface")).toContainElement(listbox);
  });

  it("closes with Escape", () => {
    render(
      <CustomSelect
        value=""
        options={options}
        onChange={vi.fn()}
        ariaLabel="Auswahl"
      />,
    );

    const trigger = screen.getByRole("combobox", { name: "Auswahl" });
    fireEvent.click(trigger);
    expect(screen.getByRole("listbox")).toBeInTheDocument();

    fireEvent.keyDown(trigger, { key: "Escape" });

    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("keeps Escape from reaching document listeners while the trigger owns an open listbox", () => {
    const onDocumentKeyDown = vi.fn();
    document.addEventListener("keydown", onDocumentKeyDown);

    try {
      render(
        <CustomSelect
          value=""
          options={options}
          onChange={vi.fn()}
          ariaLabel="Auswahl"
        />,
      );

      const trigger = screen.getByRole("combobox", { name: "Auswahl" });
      fireEvent.click(trigger);
      fireEvent.keyDown(trigger, { key: "Escape" });

      expect(onDocumentKeyDown).not.toHaveBeenCalled();
      expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    } finally {
      document.removeEventListener("keydown", onDocumentKeyDown);
    }
  });

  it("keeps Escape from reaching document listeners while an option owns focus", () => {
    const onDocumentKeyDown = vi.fn();
    document.addEventListener("keydown", onDocumentKeyDown);

    try {
      render(
        <CustomSelect
          value=""
          options={options}
          onChange={vi.fn()}
          ariaLabel="Auswahl"
        />,
      );

      fireEvent.click(screen.getByRole("combobox", { name: "Auswahl" }));
      fireEvent.keyDown(screen.getByRole("option", { name: "Erste Option" }), {
        key: "Escape",
      });

      expect(onDocumentKeyDown).not.toHaveBeenCalled();
      expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    } finally {
      document.removeEventListener("keydown", onDocumentKeyDown);
    }
  });

  it("ignores disabled options", () => {
    const onChange = vi.fn();
    render(
      <CustomSelect
        value=""
        options={options}
        onChange={onChange}
        ariaLabel="Auswahl"
      />,
    );

    fireEvent.click(screen.getByRole("combobox", { name: "Auswahl" }));
    fireEvent.click(screen.getByRole("option", { name: "Gesperrt" }));

    expect(onChange).not.toHaveBeenCalled();
  });

  it("supports keyboard selection", () => {
    const onChange = vi.fn();
    render(
      <CustomSelect
        value=""
        options={options}
        onChange={onChange}
        ariaLabel="Auswahl"
      />,
    );

    const trigger = screen.getByRole("combobox", { name: "Auswahl" });
    fireEvent.keyDown(trigger, { key: "ArrowDown" });
    fireEvent.keyDown(trigger, { key: "ArrowDown" });
    fireEvent.keyDown(trigger, { key: "Enter" });

    expect(onChange).toHaveBeenCalledWith("first");
  });

  it("keeps a hidden form value", () => {
    render(
      <CustomSelect
        name="phase"
        value="second"
        options={options}
        onChange={vi.fn()}
        ariaLabel="Auswahl"
      />,
    );

    expect(document.querySelector('input[name="phase"]')).toHaveValue("second");
  });
});
