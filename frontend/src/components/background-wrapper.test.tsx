import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { BackgroundWrapper } from "./background-wrapper";

// Mock AnimatedBackground
vi.mock("./animated-background", () => ({
  AnimatedBackground: () => <div data-testid="animated-background" />,
}));

// Mock useModal hook
const mockIsModalOpen = vi.fn();
vi.mock("./dashboard/modal-context", () => ({
  useModal: () => ({
    isModalOpen: mockIsModalOpen() as boolean,
  }),
}));

describe("BackgroundWrapper", () => {
  it("renders children", () => {
    mockIsModalOpen.mockReturnValue(false);

    render(
      <BackgroundWrapper>
        <div>Test content</div>
      </BackgroundWrapper>,
    );

    expect(screen.getByText("Test content")).toBeInTheDocument();
  });

  it("renders AnimatedBackground", () => {
    mockIsModalOpen.mockReturnValue(false);

    render(
      <BackgroundWrapper>
        <div>Content</div>
      </BackgroundWrapper>,
    );

    expect(screen.getByTestId("animated-background")).toBeInTheDocument();
  });

  it("shows modal overlay when modal is open", () => {
    mockIsModalOpen.mockReturnValue(true);

    const { container } = render(
      <BackgroundWrapper>
        <div>Content</div>
      </BackgroundWrapper>,
    );

    const overlay = container.querySelector(".bg-black\\/5.opacity-100");
    expect(overlay).toBeInTheDocument();
  });

  it("hides modal overlay when modal is closed", () => {
    mockIsModalOpen.mockReturnValue(false);

    const { container } = render(
      <BackgroundWrapper>
        <div>Content</div>
      </BackgroundWrapper>,
    );

    const overlay = container.querySelector(".opacity-0");
    expect(overlay).toBeInTheDocument();
  });

  it("has backdrop-blur on overlay", () => {
    mockIsModalOpen.mockReturnValue(false);

    const { container } = render(
      <BackgroundWrapper>
        <div>Content</div>
      </BackgroundWrapper>,
    );

    const overlay = container.querySelector(".backdrop-blur-sm");
    expect(overlay).toBeInTheDocument();
  });

  it("has pointer-events-none on overlay", () => {
    mockIsModalOpen.mockReturnValue(false);

    const { container } = render(
      <BackgroundWrapper>
        <div>Content</div>
      </BackgroundWrapper>,
    );

    const overlay = container.querySelector(".pointer-events-none");
    expect(overlay).toBeInTheDocument();
  });
});
