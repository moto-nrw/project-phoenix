import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { InfoCard, InfoItem } from "./info-card";

describe("InfoCard", () => {
  it("renders title", () => {
    render(
      <InfoCard title="Test Card" icon={<span>Icon</span>}>
        Content
      </InfoCard>,
    );

    expect(screen.getByText("Test Card")).toBeInTheDocument();
  });

  it("renders icon", () => {
    render(
      <InfoCard title="Card" icon={<span data-testid="card-icon">Icon</span>}>
        Content
      </InfoCard>,
    );

    expect(screen.getByTestId("card-icon")).toBeInTheDocument();
  });

  it("renders children content", () => {
    render(
      <InfoCard title="Card" icon={<span>Icon</span>}>
        <p>This is the content</p>
      </InfoCard>,
    );

    expect(screen.getByText("This is the content")).toBeInTheDocument();
  });

  it("renders with proper styling", () => {
    const { container } = render(
      <InfoCard title="Card" icon={<span>Icon</span>}>
        Content
      </InfoCard>,
    );

    const card = container.firstChild as HTMLElement;
    expect(card.className).toContain("rounded-2xl");
    expect(card.className).toContain("border");
  });
});

describe("InfoItem", () => {
  it("renders label", () => {
    render(<InfoItem label="Name" value="John Doe" />);

    expect(screen.getByText("Name")).toBeInTheDocument();
  });

  it("renders string value", () => {
    render(<InfoItem label="Name" value="John Doe" />);

    expect(screen.getByText("John Doe")).toBeInTheDocument();
  });

  it("renders ReactNode value", () => {
    render(
      <InfoItem
        label="Status"
        value={<span data-testid="status-badge">Active</span>}
      />,
    );

    expect(screen.getByTestId("status-badge")).toBeInTheDocument();
    expect(screen.getByText("Active")).toBeInTheDocument();
  });

  it("renders with icon when provided", () => {
    render(
      <InfoItem
        label="Email"
        value="test@example.com"
        icon={<span data-testid="email-icon">@</span>}
      />,
    );

    expect(screen.getByTestId("email-icon")).toBeInTheDocument();
  });

  it("renders without icon when not provided", () => {
    const { container } = render(<InfoItem label="Name" value="John" />);

    // Should have 2 children: label and value, no icon container
    const wrapper = container.firstChild as HTMLElement;
    expect(wrapper.children.length).toBe(1); // Just the text container
  });

  it("applies proper styling to label", () => {
    render(<InfoItem label="Field Label" value="Value" />);

    const label = screen.getByText("Field Label");
    expect(label.className).toContain("text-xs");
    expect(label.className).toContain("text-gray-500");
  });

  it("applies proper styling to value", () => {
    render(<InfoItem label="Label" value="Field Value" />);

    const value = screen.getByText("Field Value");
    expect(value.className).toContain("font-medium");
    expect(value.className).toContain("text-gray-900");
  });
});
