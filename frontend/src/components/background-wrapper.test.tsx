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
});
