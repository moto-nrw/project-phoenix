import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SamsungChromeInstallSection } from "./samsung-chrome-install-section";

const SAMSUNG_INTERNET_UA =
  "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/28.0 Chrome/130.0.0.0 Mobile Safari/537.36";
const CHROME_UA =
  "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.6478.71 Mobile Safari/537.36";

function stubNavigator(userAgent: string) {
  vi.stubGlobal("navigator", { userAgent });
}

function stubStandalone(standalone: boolean) {
  vi.stubGlobal(
    "matchMedia",
    vi.fn((query: string) => ({
      matches: standalone && query === "(display-mode: standalone)",
      media: query,
    })),
  );
}

describe("SamsungChromeInstallSection", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("offers a Chrome installation path in Samsung Internet", async () => {
    stubNavigator(SAMSUNG_INTERNET_UA);
    window.location.href = "https://eltern.moto-app.de/settings";

    render(<SamsungChromeInstallSection />);

    expect(
      await screen.findByRole("heading", { name: "moto als App installieren" }),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("listitem")).toHaveLength(3);
    expect(screen.getByText(/eltern\.moto-app\.de/)).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "In Chrome öffnen" }),
    ).toHaveAttribute(
      "href",
      expect.stringContaining("package=com.android.chrome"),
    );
  });

  it("stays hidden in Chrome", () => {
    stubNavigator(CHROME_UA);

    render(<SamsungChromeInstallSection />);

    expect(
      screen.queryByRole("heading", { name: "moto als App installieren" }),
    ).not.toBeInTheDocument();
  });

  it("stays hidden when moto is already running as an installed app", () => {
    stubNavigator(SAMSUNG_INTERNET_UA);
    stubStandalone(true);

    render(<SamsungChromeInstallSection />);

    expect(
      screen.queryByRole("heading", { name: "moto als App installieren" }),
    ).not.toBeInTheDocument();
  });
});
