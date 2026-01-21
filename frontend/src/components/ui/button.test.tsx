import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { Button } from "./button";

describe("Button", () => {
  it("renders children correctly", () => {
    render(<Button>Click me</Button>);

    expect(screen.getByRole("button", { name: "Click me" })).toBeInTheDocument();
  });

  it("renders with default type submit", () => {
    render(<Button>Submit</Button>);

    expect(screen.getByRole("button")).toHaveAttribute("type", "submit");
  });

  it("shows loading text when isLoading is true", () => {
    render(<Button isLoading>Submit</Button>);

    expect(screen.getByRole("button")).toHaveTextContent("Loading...");
    expect(screen.queryByText("Submit")).not.toBeInTheDocument();
  });

  it("shows custom loading text when provided", () => {
    render(
      <Button isLoading loadingText="Saving...">
        Submit
      </Button>,
    );

    expect(screen.getByRole("button")).toHaveTextContent("Saving...");
  });

  it("is disabled when isLoading is true", () => {
    render(<Button isLoading>Submit</Button>);

    expect(screen.getByRole("button")).toBeDisabled();
  });

  it("calls onClick handler when clicked", () => {
    const handleClick = vi.fn();
    render(<Button onClick={handleClick}>Click</Button>);

    fireEvent.click(screen.getByRole("button"));

    expect(handleClick).toHaveBeenCalledTimes(1);
  });

  it("applies primary variant styles by default", () => {
    render(<Button>Primary</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("bg-gray-900");
    expect(button.className).toContain("text-white");
  });

  it("applies secondary variant styles", () => {
    render(<Button variant="secondary">Secondary</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("bg-gray-200");
    expect(button.className).toContain("text-gray-800");
  });

  it("applies outline variant styles", () => {
    render(<Button variant="outline">Outline</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("bg-transparent");
  });

  it("applies outline_danger variant styles", () => {
    render(<Button variant="outline_danger">Danger Outline</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("bg-red-50");
    expect(button.className).toContain("text-red-600");
  });

  it("applies danger variant styles", () => {
    render(<Button variant="danger">Danger</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("bg-red-600");
    expect(button.className).toContain("text-white");
  });

  it("applies success variant styles", () => {
    render(<Button variant="success">Success</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("bg-green-600");
    expect(button.className).toContain("text-white");
  });

  it("applies sm size styles", () => {
    render(<Button size="sm">Small</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("text-sm");
  });

  it("applies base size styles by default", () => {
    render(<Button>Base</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("text-base");
  });

  it("applies lg size styles", () => {
    render(<Button size="lg">Large</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("text-lg");
  });

  it("applies xl size styles", () => {
    render(<Button size="xl">Extra Large</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("text-xl");
  });

  it("passes custom className", () => {
    render(<Button className="custom-class">Custom</Button>);

    const button = screen.getByRole("button");
    expect(button.className).toContain("custom-class");
  });

  it("passes additional props", () => {
    render(
      <Button data-testid="custom-button" aria-label="Custom action">
        Custom
      </Button>,
    );

    expect(screen.getByTestId("custom-button")).toBeInTheDocument();
    expect(screen.getByLabelText("Custom action")).toBeInTheDocument();
  });

  it("can be disabled via disabled prop", () => {
    render(<Button disabled>Disabled</Button>);

    expect(screen.getByRole("button")).toBeDisabled();
  });
});
