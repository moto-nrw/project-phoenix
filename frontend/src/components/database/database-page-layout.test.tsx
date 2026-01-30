import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { DatabasePageLayout } from "./database-page-layout";

vi.mock("~/components/ui/loading", () => ({
  Loading: ({ fullPage }: { fullPage?: boolean }) => (
    <div data-testid="loading" data-full-page={String(fullPage)}>
      Loading...
    </div>
  ),
}));

vi.mock("~/components/ui/mobile-back-button", () => ({
  MobileBackButton: () => <button data-testid="mobile-back">Back</button>,
}));

describe("DatabasePageLayout", () => {
  it("shows loading when sessionLoading is true", () => {
    render(
      <DatabasePageLayout loading={false} sessionLoading={true}>
        <div>Content</div>
      </DatabasePageLayout>,
    );

    expect(screen.getByTestId("loading")).toBeInTheDocument();
    expect(screen.queryByText("Content")).not.toBeInTheDocument();
  });

  it("shows loading when loading is true", () => {
    render(
      <DatabasePageLayout loading={true} sessionLoading={false}>
        <div>Content</div>
      </DatabasePageLayout>,
    );

    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("renders children and mobile back button when not loading", () => {
    render(
      <DatabasePageLayout loading={false} sessionLoading={false}>
        <div>Content</div>
      </DatabasePageLayout>,
    );

    expect(screen.queryByTestId("loading")).not.toBeInTheDocument();
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
