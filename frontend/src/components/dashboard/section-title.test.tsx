import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { SectionTitle } from "./section-title";

describe("SectionTitle", () => {
  it("renders the title text", () => {
    render(<SectionTitle title="Dashboard Overview" />);

    expect(screen.getByText("Dashboard Overview")).toBeInTheDocument();
  });

  it("renders as h2 heading", () => {
    render(<SectionTitle title="Section Heading" />);

    expect(
      screen.getByRole("heading", { level: 2, name: "Section Heading" }),
    ).toBeInTheDocument();
  });

  it("applies gradient text styling", () => {
    render(<SectionTitle title="Styled Title" />);

    const heading = screen.getByRole("heading");
    expect(heading.className).toContain("bg-gradient-to-r");
    expect(heading.className).toContain("bg-clip-text");
    expect(heading.className).toContain("text-transparent");
  });

  it("renders with underline decoration", () => {
    const { container } = render(<SectionTitle title="Title" />);

    // Check for the decorative underline div
    const underline = container.querySelector(".h-1.rounded-full");
    expect(underline).toBeInTheDocument();
  });

  it("has group hover effects", () => {
    const { container } = render(<SectionTitle title="Hover Title" />);

    const wrapper = container.firstChild as HTMLElement;
    expect(wrapper.className).toContain("group");
  });
});
