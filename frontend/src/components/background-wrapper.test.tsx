import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { BackgroundWrapper } from "./background-wrapper";

describe("BackgroundWrapper", () => {
  it("renders children", () => {
    render(
      <BackgroundWrapper>
        <div>Test content</div>
      </BackgroundWrapper>,
    );

    expect(screen.getByText("Test content")).toBeInTheDocument();
  });

  it("does not duplicate the backdrop owned by the modal primitive", () => {
    const { container } = render(
      <BackgroundWrapper>
        <div>Content</div>
      </BackgroundWrapper>,
    );

    expect(container.querySelector('[aria-hidden="true"]')).toBeNull();
  });
});
