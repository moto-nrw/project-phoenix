import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  PwaInstallHint,
  isAndroidDevice,
  isIosDevice,
  isIosSafari,
  isStandaloneDisplay,
} from "./pwa-install-hint";
import { GROUP_ROOM_SHADES } from "~/lib/location-helper";
import {
  canPromptInstall,
  recordVisit,
  resetInstallPromptForTests,
} from "~/lib/pwa-install-prompt";

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

/**
 * Puts storage in the state of someone opening the app for a second browser
 * session, which is the earliest point the hint is allowed to appear.
 */
function seedReturningVisitor() {
  localStorage.setItem("moto-pwa-install-hint-visits", "1");
  sessionStorage.clear();
}

/** Fires the Chrome event the install-prompt module captures at import time. */
function dispatchInstallPrompt(outcome: "accepted" | "dismissed" = "accepted") {
  const prompt = vi.fn().mockResolvedValue(undefined);
  const event = Object.assign(new Event("beforeinstallprompt"), {
    prompt,
    userChoice: Promise.resolve({ outcome }),
  });
  window.dispatchEvent(event);
  return prompt;
}

describe("recordVisit", () => {
  beforeEach(() => {
    localStorage.clear();
    sessionStorage.clear();
  });

  it("counts one visit per browser session, not per call", () => {
    expect(recordVisit(window)).toBe(1);
    expect(recordVisit(window)).toBe(1);
    sessionStorage.clear();
    expect(recordVisit(window)).toBe(2);
  });
});

describe("PwaInstallHint", () => {
  beforeEach(() => {
    vi.stubEnv("NEXT_PUBLIC_TENANT_DOMAIN", "moto-app.de");
    window.location.href = "https://school-a.moto-app.de/dashboard";
    localStorage.clear();
    sessionStorage.clear();
    resetInstallPromptForTests();
    seedReturningVisitor();
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
    resetInstallPromptForTests();
  });

  it("renders Safari instructions in a centered floating card", () => {
    stubNavigator({ userAgent: IPHONE_UA });
    stubMatchMedia(false);
    const { container } = render(<PwaInstallHint />);
    expect(screen.getByText("moto als App nutzen")).toBeInTheDocument();
    expect(screen.getByText("Zum Home-Bildschirm")).toBeInTheDocument();

    const card = container.firstElementChild;
    expect(card).toHaveClass("fixed", "rounded-2xl");
    // Brand-tinted surface with a 2px brand border so the promotion is
    // distinguishable from the neutral gray content cards. `moto-content-surface`
    // is deliberately absent: it is unlayered and would force white + gray-200.
    expect(card).toHaveClass("border-2");
    expect(card).toHaveStyle({
      borderColor: GROUP_ROOM_SHADES.base,
      backgroundColor: GROUP_ROOM_SHADES.bgHover,
    });
    expect(card?.className).not.toMatch(/moto-content-surface/);
    // Centered at every width, not an edge-to-edge banner that snaps to the
    // right at sm.
    expect(card).toHaveClass("left-1/2", "-translate-x-1/2", "max-w-md");
    expect(card?.className).not.toMatch(/\binset-x-/);
    expect(card?.className).not.toMatch(/\bsm:right-/);
  });

  it("clears responsive bottom controls only while they are visible", () => {
    stubNavigator({ userAgent: IPHONE_UA });
    stubMatchMedia(false);
    const { container } = render(<PwaInstallHint />);
    const card = container.firstElementChild;

    // The FAB contribution is a CSS variable defaulting to 0, so pages
    // without a FAB do not reserve room for one.
    expect(card).toHaveClass(
      "bottom-[calc(5.75rem+var(--moto-floating-fab-offset,0rem)+var(--moto-checkin-bar-offset,0rem)+env(safe-area-inset-bottom))]",
    );
    // At lg the centered card and right-aligned FAB no longer intersect, so
    // the hint can use the shell's normal desktop bottom spacing.
    expect(card).toHaveClass("lg:bottom-8");
    expect(card?.className).not.toMatch(/\bxl:bottom-/);
    expect(card?.className).not.toMatch(/10\.5rem/);
  });

  it("stays hidden on the very first visit", () => {
    localStorage.clear();
    sessionStorage.clear();
    stubNavigator({ userAgent: IPHONE_UA });
    stubMatchMedia(false);
    render(<PwaInstallHint />);
    expect(screen.queryByText("moto als App nutzen")).not.toBeInTheDocument();
  });

  it("appears on the second visit", () => {
    localStorage.clear();
    sessionStorage.clear();
    stubNavigator({ userAgent: IPHONE_UA });
    stubMatchMedia(false);

    const first = render(<PwaInstallHint />);
    expect(screen.queryByText("moto als App nutzen")).not.toBeInTheDocument();
    first.unmount();

    sessionStorage.clear(); // a new browser session
    render(<PwaInstallHint />);
    expect(screen.getByText("moto als App nutzen")).toBeInTheDocument();
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

  it("renders browser-menu instructions on Android when no install event was captured", () => {
    stubNavigator({ userAgent: ANDROID_UA, platform: "Linux armv81" });
    stubMatchMedia(false);
    render(<PwaInstallHint />);
    expect(screen.getByText("moto als App nutzen")).toBeInTheDocument();
    expect(screen.getByText("App installieren")).toBeInTheDocument();
    // Fallback path: instructions only, no actionable button.
    expect(
      screen.queryByRole("button", { name: "App installieren" }),
    ).not.toBeInTheDocument();
  });

  it("offers a one-tap install on Android once Chrome fires beforeinstallprompt", () => {
    stubNavigator({ userAgent: ANDROID_UA, platform: "Linux armv81" });
    stubMatchMedia(false);
    dispatchInstallPrompt();
    render(<PwaInstallHint />);
    expect(
      screen.getByText("Installieren Sie moto direkt aus dieser Ansicht.", {
        exact: false,
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "App installieren" }),
    ).toBeInTheDocument();
    // The manual browser-menu instructions are replaced, not stacked on top.
    expect(
      screen.queryByText("Zum Startbildschirm hinzufügen"),
    ).not.toBeInTheDocument();
  });

  it("captures the one-tap install prompt on the protected parents host", () => {
    vi.stubEnv("NEXT_PUBLIC_PARENTS_HOSTNAME", "parents.moto-app.de");
    window.location.href = "https://parents.moto-app.de/settings";
    stubNavigator({ userAgent: ANDROID_UA, platform: "Linux armv81" });
    stubMatchMedia(false);

    dispatchInstallPrompt();

    expect(canPromptInstall()).toBe(true);
  });

  it("replays the captured prompt and hides the card once install is accepted", async () => {
    stubNavigator({ userAgent: ANDROID_UA, platform: "Linux armv81" });
    stubMatchMedia(false);
    const prompt = dispatchInstallPrompt("accepted");
    render(<PwaInstallHint />);

    fireEvent.click(screen.getByRole("button", { name: "App installieren" }));

    await waitFor(() => {
      expect(prompt).toHaveBeenCalledTimes(1);
    });
    await waitFor(() => {
      expect(screen.queryByText("moto als App nutzen")).not.toBeInTheDocument();
    });
  });

  it("keeps the card in place when the install prompt is declined", async () => {
    stubNavigator({ userAgent: ANDROID_UA, platform: "Linux armv81" });
    stubMatchMedia(false);
    const prompt = dispatchInstallPrompt("dismissed");
    render(<PwaInstallHint />);

    fireEvent.click(screen.getByRole("button", { name: "App installieren" }));

    await waitFor(() => {
      expect(prompt).toHaveBeenCalledTimes(1);
    });
    // Falls back to the manual instructions because the one-shot event is
    // spent, but the promotion itself must not disappear silently.
    expect(screen.getByText("moto als App nutzen")).toBeInTheDocument();
  });

  it("hides and persistently dismisses the card after browser-menu installation", async () => {
    stubNavigator({ userAgent: ANDROID_UA, platform: "Linux armv81" });
    stubMatchMedia(false);
    render(<PwaInstallHint />);

    fireEvent(window, new Event("appinstalled"));

    await waitFor(() => {
      expect(screen.queryByText("moto als App nutzen")).not.toBeInTheDocument();
    });
    expect(localStorage.getItem("moto-pwa-install-hint-dismissed")).toBe("1");
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

  it("does not render in desktop Safari on macOS", () => {
    // Closest thing to a false positive: iPadOS Safari sends this exact UA.
    // Only maxTouchPoints tells the two apart, so a Mac must stay excluded
    // even though the wide iPad layout looks the same.
    stubNavigator({
      userAgent: IPAD_SAFARI_DESKTOP_UA,
      platform: "MacIntel",
      maxTouchPoints: 0,
    });
    stubMatchMedia(false);
    render(<PwaInstallHint />);
    expect(screen.queryByText("moto als App nutzen")).not.toBeInTheDocument();
  });

  it("does not render on a touchscreen Windows laptop", () => {
    stubNavigator({
      userAgent:
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36",
      platform: "Win32",
      maxTouchPoints: 10,
    });
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
