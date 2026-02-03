import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { ModalLoadingState } from "./modal-loading-state";

describe("ModalLoadingState", () => {
  it("renders with default gray accent color", () => {
    const { container } = render(<ModalLoadingState />);

    const spinner = container.querySelector(".border-t-gray-500");
    expect(spinner).toBeInTheDocument();
  });

  it("renders with default message", () => {
    render(<ModalLoadingState />);

    expect(screen.getByText("Daten werden geladen...")).toBeInTheDocument();
  });

  it("renders with custom message", () => {
    render(<ModalLoadingState message="Bitte warten..." />);

    expect(screen.getByText("Bitte warten...")).toBeInTheDocument();
  });

  it("renders with orange accent color", () => {
    const { container } = render(<ModalLoadingState accentColor="orange" />);

    const spinner = container.querySelector(".border-t-\\[\\#F78C10\\]");
    expect(spinner).toBeInTheDocument();
  });

  it("renders with indigo accent color", () => {
    const { container } = render(<ModalLoadingState accentColor="indigo" />);

    const spinner = container.querySelector(".border-t-indigo-500");
    expect(spinner).toBeInTheDocument();
  });

  it("renders with blue accent color", () => {
    const { container } = render(<ModalLoadingState accentColor="blue" />);

    const spinner = container.querySelector(".border-t-\\[\\#5080D8\\]");
    expect(spinner).toBeInTheDocument();
  });

  it("renders with green accent color", () => {
    const { container } = render(<ModalLoadingState accentColor="green" />);

    const spinner = container.querySelector(".border-t-green-500");
    expect(spinner).toBeInTheDocument();
  });

  it("has spinner with animate-spin class", () => {
    const { container } = render(<ModalLoadingState />);

    const spinner = container.querySelector(".animate-spin");
    expect(spinner).toBeInTheDocument();
  });
});
