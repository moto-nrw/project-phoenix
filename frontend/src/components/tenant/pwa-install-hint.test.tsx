import { render, screen, fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  PwaInstallHint,
  isAndroidDevice,
  isIosDevice,
  isIosSafari,
  isStandaloneDisplay,
} from "./pwa-install-hint";

const IPHONE_UA =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1";
const IOS_CHROME_UA =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/126.0.6478.71 Mobile/15E148 Safari/604.1";
const IOS_CHROME_DESKTOP_UA =
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/126 Version/17.5 Safari/605.1.15";
const IOS_FIREFOX_UA =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/127.0 Mobile/15E148 Safari/605.1.15";
const IPAD_SAFARI_DESKTOP_UA =
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15";
const IOS_WEBVIEW_UA =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148";
const ANDROID_UA =
  "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.6478.71 Mobile Safari/537.36";
const ANDROID_WEBVIEW_UA =
  "Mozilla/5.0 (Linux; Android 14; Pixel 8 Build/AP2A.240705.005; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/126.0.6478.71 Mobile Safari/537.36";
const ANDROID_WEBVIEW_WITHOUT_WV_UA =
  "Mozilla/5.0 (Linux; Android 8.1; Pixel 2 Build/OPM1.171019.011) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/67.0.3396.87 Mobile Safari/537.36";
const DESKTOP_UA =
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36";

function stubNavigator(overrides: Partial<Navigator>) {
  vi.stubGlobal("navigator", {
    userAgent: DESKTOP_UA,
    platform: "MacIntel",
    maxTouchPoints: 0,
    ...overrides,
  });
}

function stubMatchMedia(standalone: boolean) {
  vi.stubGlobal(
    "matchMedia",
    vi.fn((query: string) => ({
      matches: standalone && query === "(display-mode: standalone)",
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  );
}

describe("isIosDevice", () => {
  it("detects iPhone user agents", () => {
    expect(isIosDevice({ userAgent: IPHONE_UA } as Navigator)).toBe(true);
  });

  it("detects iPadOS masquerading as MacIntel with touch", () => {
    expect(
      isIosDevice({
        userAgent: DESKTOP_UA,
        platform: "MacIntel",
        maxTouchPoints: 5,
      } as Navigator),
    ).toBe(true);
  });

  it("rejects desktop browsers", () => {
    expect(
      isIosDevice({
        userAgent: DESKTOP_UA,
        platform: "MacIntel",
        maxTouchPoints: 0,
      } as Navigator),
    ).toBe(false);
  });
});

describe("isIosSafari", () => {
  it("detects Safari on iOS", () => {
    expect(isIosSafari({ userAgent: IPHONE_UA } as Navigator)).toBe(true);
    expect(
      isIosSafari({
        userAgent: IPAD_SAFARI_DESKTOP_UA,
        platform: "MacIntel",
        maxTouchPoints: 5,
      } as Navigator),
    ).toBe(true);
  });

  it("rejects other iOS browsers and embedded webviews", () => {
    expect(isIosSafari({ userAgent: IOS_CHROME_UA } as Navigator)).toBe(false);
    expect(
      isIosSafari({
        userAgent: IOS_CHROME_DESKTOP_UA,
        platform: "MacIntel",
        maxTouchPoints: 5,
      } as Navigator),
    ).toBe(false);
    expect(isIosSafari({ userAgent: IOS_FIREFOX_UA } as Navigator)).toBe(false);
    expect(isIosSafari({ userAgent: IOS_WEBVIEW_UA } as Navigator)).toBe(false);
  });
});

describe("isAndroidDevice", () => {
  it("detects Android browser user agents", () => {
    expect(isAndroidDevice({ userAgent: ANDROID_UA } as Navigator)).toBe(true);
  });

  it("rejects Android WebView user agents", () => {
    expect(
      isAndroidDevice({ userAgent: ANDROID_WEBVIEW_UA } as Navigator),
    ).toBe(false);
    expect(
      isAndroidDevice({
        userAgent: ANDROID_WEBVIEW_WITHOUT_WV_UA,
      } as Navigator),
    ).toBe(false);
  });

  it("rejects iOS and desktop user agents", () => {
    expect(isAndroidDevice({ userAgent: IPHONE_UA } as Navigator)).toBe(false);
    expect(isAndroidDevice({ userAgent: DESKTOP_UA } as Navigator)).toBe(false);
  });
});

describe("isStandaloneDisplay", () => {
  it("detects the standalone display-mode media query", () => {
    stubMatchMedia(true);
    expect(isStandaloneDisplay(window)).toBe(true);
  });

  it("detects the legacy navigator.standalone flag", () => {
    stubMatchMedia(false);
    stubNavigator({ standalone: true } as Partial<Navigator>);
    expect(isStandaloneDisplay(window)).toBe(true);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });
});

describe("PwaInstallHint", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders Safari instructions above mobile check-in controls", () => {
    stubNavigator({ userAgent: IPHONE_UA });
    stubMatchMedia(false);
    const { container } = render(<PwaInstallHint />);
    expect(screen.getByText("moto als App nutzen")).toBeInTheDocument();
    expect(screen.getByText("Zum Home-Bildschirm")).toBeInTheDocument();
    expect(container.firstElementChild).toHaveClass(
      "bottom-[calc(10.5rem+env(safe-area-inset-bottom))]",
      "xl:bottom-6",
    );
  });

  it("does not render Safari instructions in other iOS browsers", () => {
    stubNavigator({ userAgent: IOS_CHROME_UA });
    stubMatchMedia(false);
    render(<PwaInstallHint />);
    expect(screen.queryByText("moto als App nutzen")).not.toBeInTheDocument();
  });

  it("does not render inside embedded iOS webviews", () => {
    stubNavigator({ userAgent: IOS_WEBVIEW_UA });
    stubMatchMedia(false);
    render(<PwaInstallHint />);
    expect(screen.queryByText("moto als App nutzen")).not.toBeInTheDocument();
  });

  it("renders browser-menu instructions on Android in browser mode", () => {
    stubNavigator({ userAgent: ANDROID_UA, platform: "Linux armv81" });
    stubMatchMedia(false);
    render(<PwaInstallHint />);
    expect(screen.getByText("moto als App nutzen")).toBeInTheDocument();
    expect(screen.getByText("App installieren")).toBeInTheDocument();
  });

  it("does not render inside Android WebViews", () => {
    stubNavigator({
      userAgent: ANDROID_WEBVIEW_UA,
      platform: "Linux armv81",
    });
    stubMatchMedia(false);
    render(<PwaInstallHint />);
    expect(screen.queryByText("moto als App nutzen")).not.toBeInTheDocument();
  });

  it("does not render on desktop devices", () => {
    stubNavigator({ userAgent: DESKTOP_UA });
    stubMatchMedia(false);
    render(<PwaInstallHint />);
    expect(screen.queryByText("moto als App nutzen")).not.toBeInTheDocument();
  });

  it("does not render inside an installed PWA", () => {
    stubNavigator({ userAgent: IPHONE_UA });
    stubMatchMedia(true);
    render(<PwaInstallHint />);
    expect(screen.queryByText("moto als App nutzen")).not.toBeInTheDocument();
  });

  it("does not render inside an installed PWA on Android", () => {
    stubNavigator({ userAgent: ANDROID_UA, platform: "Linux armv81" });
    stubMatchMedia(true);
    render(<PwaInstallHint />);
    expect(screen.queryByText("moto als App nutzen")).not.toBeInTheDocument();
  });

  it("stays hidden after dismissal", () => {
    stubNavigator({ userAgent: IPHONE_UA });
    stubMatchMedia(false);
    const { unmount } = render(<PwaInstallHint />);
    fireEvent.click(screen.getByLabelText("Hinweis schließen"));
    expect(screen.queryByText("moto als App nutzen")).not.toBeInTheDocument();
    unmount();

    render(<PwaInstallHint />);
    expect(screen.queryByText("moto als App nutzen")).not.toBeInTheDocument();
  });
});
