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
});
