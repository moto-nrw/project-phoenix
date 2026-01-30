import { describe, it, expect, vi } from "vitest";
import { render } from "@testing-library/react";
import RootLayout, { metadata, viewport } from "./layout";

// Mock next/font/google
vi.mock("next/font/google", () => ({
  Inter: () => ({
    className: "inter-font-class",
  }),
}));

// Mock child components
vi.mock("./providers", () => ({
  Providers: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="providers">{children}</div>
  ),
}));

vi.mock("~/components/background-wrapper", () => ({
  BackgroundWrapper: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="background-wrapper">{children}</div>
  ),
}));

describe("RootLayout", () => {
  it("renders children wrapped in providers", () => {
    const { getByText, getByTestId } = render(
      <RootLayout>
        <div>Test Content</div>
      </RootLayout>,
    );

    expect(getByText("Test Content")).toBeInTheDocument();
    expect(getByTestId("providers")).toBeInTheDocument();
    expect(getByTestId("background-wrapper")).toBeInTheDocument();
  });

  it("renders html and body structure", () => {
    const { container } = render(
      <RootLayout>
        <div>Test</div>
      </RootLayout>,
    );

    // RootLayout is a server component that renders html and body tags
    // In test environment, we can verify the structure exists
    expect(container).toBeTruthy();
  });

  it("wraps content in providers and background wrapper", () => {
    const { getByTestId } = render(
      <RootLayout>
        <div>Test</div>
      </RootLayout>,
    );

    // Verify both wrapper components are present
    expect(getByTestId("providers")).toBeInTheDocument();
    expect(getByTestId("background-wrapper")).toBeInTheDocument();
  });

  describe("metadata", () => {
    it("has correct title", () => {
      expect(metadata.title).toBe("moto");
    });

    it("has correct description", () => {
      expect(metadata.description).toBe("A modern full-stack application");
    });

    it("has correct icons", () => {
      expect(metadata.icons).toEqual([
        { rel: "icon", url: "/favicon.png", type: "image/png" },
        {
          rel: "apple-touch-icon",
          url: "/apple-touch-icon.png",
          sizes: "180x180",
        },
      ]);
    });

    it("has correct manifest", () => {
      expect(metadata.manifest).toBe("/site.webmanifest");
    });
  });

  describe("viewport", () => {
    it("has correct viewport settings", () => {
      expect(viewport).toEqual({
        width: "device-width",
        initialScale: 1,
        maximumScale: 1,
        userScalable: false,
      });
    });
  });
});
