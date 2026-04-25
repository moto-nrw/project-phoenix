import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { StudentsGroupingToggle } from "./students-grouping-toggle";

describe("StudentsGroupingToggle", () => {
  it("renders the label for the current value", () => {
    render(<StudentsGroupingToggle value="group" onChange={vi.fn()} />);

    expect(screen.getByText("Gruppe")).toBeInTheDocument();
  });

  it("falls back to 'Klasse' for an unknown mode", () => {
    render(
      <StudentsGroupingToggle
        // @ts-expect-error testing fallback for unknown values
        value="unknown-mode"
        onChange={vi.fn()}
      />,
    );

    expect(screen.getByText("Klasse")).toBeInTheDocument();
  });

  it("opens the listbox on button click", () => {
    render(<StudentsGroupingToggle value="class" onChange={vi.fn()} />);

    const trigger = screen.getByRole("button", { expanded: false });
    fireEvent.click(trigger);

    expect(screen.getByRole("listbox")).toBeInTheDocument();
    expect(trigger).toHaveAttribute("aria-expanded", "true");
  });

  it("shows an active check on the selected option", () => {
    render(<StudentsGroupingToggle value="class" onChange={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { expanded: false }));

    const options = screen.getAllByRole("option");
    const active = options.find(
      (opt) => opt.getAttribute("aria-selected") === "true",
    );
    expect(active).toBeDefined();
    expect(active!.textContent).toContain("Klasse");
    expect(active!.textContent).toContain("✓");
  });

  it("calls onChange and closes when another option picked", () => {
    const onChange = vi.fn();
    render(<StudentsGroupingToggle value="class" onChange={onChange} />);

    fireEvent.click(screen.getByRole("button", { expanded: false }));
    const groupOption = screen.getByRole("option", { name: /Gruppe/ });
    fireEvent.click(groupOption);

    expect(onChange).toHaveBeenCalledWith("group");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("closes when clicking outside the component", () => {
    render(
      <div>
        <div data-testid="outside">Outside</div>
        <StudentsGroupingToggle value="class" onChange={vi.fn()} />
      </div>,
    );

    fireEvent.click(screen.getByRole("button", { expanded: false }));
    expect(screen.getByRole("listbox")).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByTestId("outside"));
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("does not close when clicking inside the component", () => {
    render(<StudentsGroupingToggle value="class" onChange={vi.fn()} />);

    const trigger = screen.getByRole("button", { expanded: false });
    fireEvent.click(trigger);

    fireEvent.mouseDown(screen.getByRole("listbox"));
    expect(screen.getByRole("listbox")).toBeInTheDocument();
  });

  it("toggling the trigger twice closes the panel", () => {
    render(<StudentsGroupingToggle value="none" onChange={vi.fn()} />);

    const trigger = screen.getByRole("button");
    fireEvent.click(trigger);
    expect(screen.getByRole("listbox")).toBeInTheDocument();

    fireEvent.click(trigger);
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });
});
