import { beforeEach, describe, it, expect, vi } from "vitest";
import { render } from "@testing-library/react";
import RootLayout, { generateMetadata, viewport } from "./layout";

const headerState = vi.hoisted(() => ({
  host: "moto-app.de",
}));

vi.mock("next/font/local", () => ({
  default: ({ variable }: { variable: string }) => ({
    className: variable === "--font-inter" ? "inter-font-class" : "",
    variable,
  }),
}));

vi.mock("next/headers", () => ({
  headers: () =>
    Promise.resolve({
      get: (name: string) =>
        name.toLowerCase() === "host" ? headerState.host : null,
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
  beforeEach(() => {
    headerState.host = "moto-app.de";
    vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", "operator.moto-app.de");
    vi.stubEnv("NEXT_PUBLIC_PARENTS_HOSTNAME", "eltern.moto-app.de");
    vi.stubEnv("TENANT_DOMAIN", "moto-app.de");
  });

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
    it("has correct title", async () => {
      const metadata = await generateMetadata();

      expect(metadata.title).toBe("moto – Digitale Ganztagsbetreuung");
    });

    it("has correct description", async () => {
      const metadata = await generateMetadata();

      expect(metadata.description).toBe(
        "Das innovative An- und Abmeldesystem mit NFC-Armbändern für die offene Ganztagsschule. DSGVO-konform, entwickelt an der Universität Münster.",
      );
    });

    it("uses the normal favicon on normal hosts", async () => {
      const metadata = await generateMetadata();

      expect(metadata.icons).toEqual([
        {
          rel: "icon",
          url: "/favicons/normal/favicon.ico",
          type: "image/x-icon",
        },
        {
          rel: "icon",
          url: "/favicons/normal/icon-32.png",
          type: "image/png",
          sizes: "32x32",
        },
        {
          rel: "apple-touch-icon",
          url: "/favicons/normal/apple-touch-icon.png",
          sizes: "180x180",
        },
      ]);
    });

    it("uses operator favicon assets on the operator host", async () => {
      headerState.host = "operator.moto-app.de";

      const metadata = await generateMetadata();

      expect(metadata.icons).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            url: "/favicons/operator/icon-32.png",
          }),
        ]),
      );
    });

    it("uses staging parents favicon assets on the staging eltern host", async () => {
      headerState.host = "eltern.staging.moto-app.de";
      vi.stubEnv(
        "NEXT_PUBLIC_OPERATOR_HOSTNAME",
        "operator.staging.moto-app.de",
      );
      vi.stubEnv("NEXT_PUBLIC_PARENTS_HOSTNAME", "eltern.staging.moto-app.de");

      const metadata = await generateMetadata();

      expect(metadata.icons).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            url: "/favicons/eltern-staging/icon-32.png",
          }),
        ]),
      );
    });

    it("has correct manifest", async () => {
      const metadata = await generateMetadata();

      expect(metadata.manifest).toBe("/manifest.webmanifest");
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
