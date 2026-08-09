import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { TrackingIndicators } from "./tracking-indicators";

describe("TrackingIndicators", () => {
  it("returns null when labels are empty", () => {
    const { container } = render(
      <TrackingIndicators labels={[]} results={[]} />,
    );
    expect(container.innerHTML).toBe("");
  });

  it("renders one indicator per label", () => {
    render(
      <TrackingIndicators
        labels={["Hausaufgaben", "Mensa"]}
        results={[true, false]}
      />,
    );

    expect(screen.getByText("Hausaufgaben")).toBeInTheDocument();
    expect(screen.getByText("Mensa")).toBeInTheDocument();
  });

  it("renders green checkmark SVG for matched labels", () => {
    render(<TrackingIndicators labels={["Hausaufgaben"]} results={[true]} />);

    const svg = screen.getByLabelText("Hausaufgaben: erledigt");
    expect(svg).toBeInTheDocument();
    expect(svg).toHaveClass("text-moto-green");
  });

  it("renders gray circle SVG for unmatched labels", () => {
    render(<TrackingIndicators labels={["Mensa"]} results={[false]} />);

    const svg = screen.getByLabelText("Mensa: ausstehend");
    expect(svg).toBeInTheDocument();
    expect(svg).toHaveClass("text-gray-300");
  });

  it("treats missing results as false (unmatched)", () => {
    render(
      <TrackingIndicators
        labels={["Hausaufgaben", "Sport"]}
        results={[true]}
      />,
    );

    // First label matched
    expect(screen.getByLabelText("Hausaufgaben: erledigt")).toBeInTheDocument();
    // Second label has no result entry → defaults to false
    expect(screen.getByLabelText("Sport: ausstehend")).toBeInTheDocument();
  });

  it("renders multiple indicators with mixed results", () => {
    render(
      <TrackingIndicators
        labels={["Hausaufgaben", "Mensa", "Sport"]}
        results={[true, false, true]}
      />,
    );

    expect(screen.getByLabelText("Hausaufgaben: erledigt")).toBeInTheDocument();
    expect(screen.getByLabelText("Mensa: ausstehend")).toBeInTheDocument();
    expect(screen.getByLabelText("Sport: erledigt")).toBeInTheDocument();
  });

  it("renders label text with correct styling", () => {
    render(<TrackingIndicators labels={["Hausaufgaben"]} results={[false]} />);

    const label = screen.getByText("Hausaufgaben");
    expect(label.tagName).toBe("SPAN");
    expect(label).toHaveClass("text-xs", "font-medium", "text-gray-500");
  });
});
