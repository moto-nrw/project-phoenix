import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { DatabaseGroupingToggle } from "./database-grouping-toggle";

const options = [
  { value: "none", label: "Keine" },
  { value: "type", label: "Typ" },
  { value: "room", label: "Raum" },
] as const;

describe("DatabaseGroupingToggle", () => {
  it("shows the active option label and opens the dropdown on click", () => {
    const onChange = vi.fn();
    render(
      <DatabaseGroupingToggle
        value="type"
        options={[...options]}
        onChange={onChange}
      />,
    );

    expect(screen.getByText("Typ")).toBeInTheDocument();
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Gruppieren/ }));
    expect(screen.getByRole("listbox")).toBeInTheDocument();
  });

  it("calls onChange and closes the dropdown when an option is selected", () => {
    const onChange = vi.fn();
    render(
      <DatabaseGroupingToggle
        value="none"
        options={[...options]}
        onChange={onChange}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Gruppieren/ }));
    fireEvent.click(screen.getByRole("option", { name: "Raum" }));

    expect(onChange).toHaveBeenCalledWith("room");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("closes the dropdown when the user clicks outside of it", () => {
    const onChange = vi.fn();
    render(
      <div>
        <button type="button" data-testid="outside">
          elsewhere
        </button>
        <DatabaseGroupingToggle
          value="none"
          options={[...options]}
          onChange={onChange}
        />
      </div>,
    );

    fireEvent.click(screen.getByRole("button", { name: /Gruppieren/ }));
    expect(screen.getByRole("listbox")).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByTestId("outside"));

    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();
  });

  it("keeps the dropdown open when the user clicks inside the popover", () => {
    const onChange = vi.fn();
    render(
      <DatabaseGroupingToggle
        value="none"
        options={[...options]}
        onChange={onChange}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Gruppieren/ }));
    const listbox = screen.getByRole("listbox");
    fireEvent.mouseDown(listbox);

    expect(screen.getByRole("listbox")).toBeInTheDocument();
  });

  it("falls back to the first option label when value matches nothing", () => {
    // Cast options as a wider string union so we can pass a value that no
    // option has — exercising the safety fallback in the toggle.
    const wideOptions = options as unknown as readonly {
      value: string;
      label: string;
    }[];
    render(
      <DatabaseGroupingToggle
        value="bogus"
        options={[...wideOptions]}
        onChange={vi.fn()}
      />,
    );

    // First option label is "Keine"
    expect(screen.getByText("Keine")).toBeInTheDocument();
  });
});
