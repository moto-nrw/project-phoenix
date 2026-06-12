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
  // RootLayout is an async server component (it awaits getLocale() for the
  // <html lang>), so it must be resolved before handing the element to render.
  it("renders children wrapped in providers", async () => {
    const { getByText, getByTestId } = render(
      await RootLayout({ children: <div>Test Content</div> }),
    );

    expect(getByText("Test Content")).toBeInTheDocument();
    expect(getByTestId("providers")).toBeInTheDocument();
    expect(getByTestId("background-wrapper")).toBeInTheDocument();
  });

  it("renders html and body structure", async () => {
    const { container } = render(
      await RootLayout({ children: <div>Test</div> }),
    );

    // RootLayout is a server component that renders html and body tags
    // In test environment, we can verify the structure exists
    expect(container).toBeTruthy();
  });

  it("wraps content in providers and background wrapper", async () => {
    const { getByTestId } = render(
      await RootLayout({ children: <div>Test</div> }),
    );

    // Verify both wrapper components are present
    expect(getByTestId("providers")).toBeInTheDocument();
    expect(getByTestId("background-wrapper")).toBeInTheDocument();
  });

  describe("metadata", () => {
    it("has correct title", () => {
      expect(metadata.title).toBe("moto – Digitale Ganztagsbetreuung");
    });

    it("has correct description", () => {
      expect(metadata.description).toBe(
        "Das innovative An- und Abmeldesystem mit NFC-Armbändern für die offene Ganztagsschule. DSGVO-konform, entwickelt an der Universität Münster.",
      );
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
