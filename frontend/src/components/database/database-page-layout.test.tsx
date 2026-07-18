import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { DatabasePageLayout } from "./database-page-layout";

vi.mock("./master-detail-skeleton", () => ({
  MasterDetailSkeleton: () => (
    <div data-testid="master-detail-skeleton">Loading...</div>
  ),
}));

vi.mock("~/components/ui/mobile-back-button", () => ({
  MobileBackButton: () => (
    <button type="button" data-testid="mobile-back">
      Back
    </button>
  ),
}));

describe("DatabasePageLayout", () => {
  it("shows loading when sessionLoading is true", () => {
    render(
      <DatabasePageLayout loading={false} sessionLoading={true}>
        <div>Content</div>
      </DatabasePageLayout>,
    );

    expect(screen.getByTestId("master-detail-skeleton")).toBeInTheDocument();
    expect(screen.queryByText("Content")).not.toBeInTheDocument();
  });

  it("shows loading when loading is true", () => {
    render(
      <DatabasePageLayout loading={true} sessionLoading={false}>
        <div>Content</div>
      </DatabasePageLayout>,
    );

    expect(screen.getByTestId("master-detail-skeleton")).toBeInTheDocument();
  });

  it("renders children and mobile back button when not loading", () => {
    render(
      <DatabasePageLayout loading={false} sessionLoading={false}>
        <div>Content</div>
      </DatabasePageLayout>,
    );

    expect(
      screen.queryByTestId("master-detail-skeleton"),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Content")).toBeInTheDocument();
    expect(screen.getByTestId("mobile-back")).toBeInTheDocument();
  });

  it("applies default className", () => {
    const { container } = render(
      <DatabasePageLayout loading={false} sessionLoading={false}>
        <div>Content</div>
      </DatabasePageLayout>,
    );

    expect(container.firstChild).toHaveClass("w-full");
  });

  it("applies custom className", () => {
    const { container } = render(
      <DatabasePageLayout
        loading={false}
        sessionLoading={false}
        className="custom-class"
      >
        <div>Content</div>
      </DatabasePageLayout>,
    );

    expect(container.firstChild).toHaveClass("custom-class");
  });
});
