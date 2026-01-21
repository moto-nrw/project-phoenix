import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { Alert } from "./alert";

describe("Alert", () => {
  it("renders error alert with message", () => {
    render(<Alert type="error" message="Something went wrong" />);

    expect(screen.getByText("Something went wrong")).toBeInTheDocument();
  });

  it("renders success alert with message", () => {
    render(<Alert type="success" message="Operation successful" />);

    expect(screen.getByText("Operation successful")).toBeInTheDocument();
  });

  it("renders warning alert with message", () => {
    render(<Alert type="warning" message="Please be careful" />);

    expect(screen.getByText("Please be careful")).toBeInTheDocument();
  });

  it("renders info alert with message", () => {
    render(<Alert type="info" message="Here is some information" />);

    expect(screen.getByText("Here is some information")).toBeInTheDocument();
  });

  it("returns null when message is empty", () => {
    const { container } = render(<Alert type="error" message="" />);

    expect(container.firstChild).toBeNull();
  });

  it("applies error styles", () => {
    render(<Alert type="error" message="Error message" />);

    const alert = screen.getByText("Error message").parentElement;
    expect(alert?.className).toContain("bg-red-50");
    expect(alert?.className).toContain("text-red-700");
  });

  it("applies success styles", () => {
    render(<Alert type="success" message="Success message" />);

    const alert = screen.getByText("Success message").parentElement;
    expect(alert?.className).toContain("text-[#6BA023]");
  });

  it("applies warning styles", () => {
    render(<Alert type="warning" message="Warning message" />);

    const alert = screen.getByText("Warning message").parentElement;
    expect(alert?.className).toContain("bg-yellow-50");
    expect(alert?.className).toContain("text-yellow-700");
  });

  it("applies info styles", () => {
    render(<Alert type="info" message="Info message" />);

    const alert = screen.getByText("Info message").parentElement;
    expect(alert?.className).toContain("bg-blue-50");
    expect(alert?.className).toContain("text-blue-700");
  });

  it("renders with icon for error type", () => {
    const { container } = render(<Alert type="error" message="Error" />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("renders with icon for success type", () => {
    const { container } = render(<Alert type="success" message="Success" />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("renders with icon for warning type", () => {
    const { container } = render(<Alert type="warning" message="Warning" />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });

  it("renders with icon for info type", () => {
    const { container } = render(<Alert type="info" message="Info" />);

    const svg = container.querySelector("svg");
    expect(svg).toBeInTheDocument();
  });
});
