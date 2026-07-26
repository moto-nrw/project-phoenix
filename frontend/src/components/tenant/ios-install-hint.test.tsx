import { render, screen, fireEvent } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  IosInstallHint,
  isIosDevice,
  isStandaloneDisplay,
} from "./ios-install-hint";

const IPHONE_UA =
  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1";
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

describe("IosInstallHint", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("renders on iOS in browser mode", () => {
    stubNavigator({ userAgent: IPHONE_UA });
    stubMatchMedia(false);
    render(<IosInstallHint />);
    expect(screen.getByText("moto als App nutzen")).toBeInTheDocument();
  });

  it("does not render on non-iOS devices", () => {
    stubNavigator({ userAgent: DESKTOP_UA });
    stubMatchMedia(false);
    render(<IosInstallHint />);
    expect(screen.queryByText("moto als App nutzen")).not.toBeInTheDocument();
  });

  it("does not render inside an installed PWA", () => {
    stubNavigator({ userAgent: IPHONE_UA });
    stubMatchMedia(true);
    render(<IosInstallHint />);
    expect(screen.queryByText("moto als App nutzen")).not.toBeInTheDocument();
  });

  it("stays hidden after dismissal", () => {
    stubNavigator({ userAgent: IPHONE_UA });
    stubMatchMedia(false);
    const { unmount } = render(<IosInstallHint />);
    fireEvent.click(screen.getByLabelText("Hinweis schließen"));
    expect(screen.queryByText("moto als App nutzen")).not.toBeInTheDocument();
    unmount();

    render(<IosInstallHint />);
    expect(screen.queryByText("moto als App nutzen")).not.toBeInTheDocument();
  });
});
