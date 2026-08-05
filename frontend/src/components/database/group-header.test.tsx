import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { GroupHeader } from "./group-header";

describe("GroupHeader", () => {
  function baseProps() {
    return {
      title: "Group",
      count: 3,
      isOpen: false,
      onToggle: vi.fn(),
    };
  }

  it("shows title and count", () => {
    render(<GroupHeader {...baseProps()} />);

    expect(screen.getByText("Group")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
  });

  it("appends countSuffix when provided", () => {
    render(<GroupHeader {...baseProps()} count={5} countSuffix="Kinder" />);

    expect(screen.getByText("5 Kinder")).toBeInTheDocument();
  });

  it("calls onToggle when button clicked", () => {
    const onToggle = vi.fn();
    render(<GroupHeader {...baseProps()} onToggle={onToggle} />);

    fireEvent.click(screen.getByRole("button"));
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it("reflects open state via aria-expanded", () => {
    const { rerender } = render(
      <GroupHeader {...baseProps()} isOpen={false} />,
    );
    expect(screen.getByRole("button")).toHaveAttribute(
      "aria-expanded",
      "false",
    );

    rerender(<GroupHeader {...baseProps()} isOpen={true} />);
    expect(screen.getByRole("button")).toHaveAttribute("aria-expanded", "true");
  });

  it.each([
    ["neutral", "bg-gray-50"],
    ["warning", "bg-[#FFF8ED]"],
    ["success", "bg-moto-green-soft/40"],
  ] as const)("applies %s variant classes", (variant, expected) => {
    const { container } = render(
      <GroupHeader {...baseProps()} variant={variant} />,
    );
    expect(container.firstChild).toHaveClass(expected);
  });

  it("renders bulkAction when supplied", () => {
    render(
      <GroupHeader
        {...baseProps()}
        bulkAction={<button type="button">Action</button>}
      />,
    );

    expect(screen.getByRole("button", { name: "Action" })).toBeInTheDocument();
  });

  it("omits bulkAction slot when no action passed", () => {
    const { container } = render(<GroupHeader {...baseProps()} />);
    const buttons = container.querySelectorAll("button");
    expect(buttons).toHaveLength(1);
  });
});
