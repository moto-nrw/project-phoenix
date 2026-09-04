import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

const ANDROID_UA =
  "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.6478.71 Mobile Safari/537.36";
const SAMSUNG_INTERNET_UA =
  "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/28.0 Chrome/130.0.0.0 Mobile Safari/537.36";
const DESKTOP_CHROME_UA =
  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36";

describe("instrumentation-client", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
    vi.stubEnv("NEXT_PUBLIC_TENANT_DOMAIN", "moto-app.de");
    vi.stubEnv("NEXT_PUBLIC_PARENTS_HOSTNAME", "eltern.moto-app.de");
    vi.stubGlobal("navigator", { userAgent: ANDROID_UA });
    window.location.href = "https://school-a.moto-app.de/dashboard";
    localStorage.clear();
    sessionStorage.clear();
    localStorage.setItem("moto-pwa-install-hint-visits", "1");
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
  });

  it("captures the install prompt before React hydration", async () => {
    await import("./instrumentation-client");
    const { canPromptInstall } = await import("./lib/pwa-install-prompt");
    const event = Object.assign(
      new Event("beforeinstallprompt", { cancelable: true }),
      {
        prompt: vi.fn().mockResolvedValue(undefined),
        userChoice: Promise.resolve({ outcome: "accepted" as const }),
      },
    );

    window.dispatchEvent(event);

    expect(event.defaultPrevented).toBe(true);
    expect(canPromptInstall()).toBe(true);
  });

  it.each([
    "https://school-a.moto-app.de/dashboard",
    "https://eltern.moto-app.de/settings",
  ])(
    "suppresses Samsung Internet installation without caching its prompt on %s",
    async (url) => {
      vi.stubGlobal("navigator", { userAgent: SAMSUNG_INTERNET_UA });
      window.location.href = url;
      await import("./instrumentation-client");
      const { canPromptInstall } = await import("./lib/pwa-install-prompt");
      const event = Object.assign(
        new Event("beforeinstallprompt", { cancelable: true }),
        {
          prompt: vi.fn().mockResolvedValue(undefined),
          userChoice: Promise.resolve({ outcome: "accepted" as const }),
        },
      );

      window.dispatchEvent(event);

      expect(event.defaultPrevented).toBe(true);
      expect(canPromptInstall()).toBe(false);
    },
  );

  it.each([
    "https://school-a.moto-app.de/",
    "https://eltern.moto-app.de/login",
    "https://eltern.moto-app.de/children/42",
  ])(
    "leaves Samsung Internet's native prompt enabled without a replacement on %s",
    async (url) => {
      vi.stubGlobal("navigator", { userAgent: SAMSUNG_INTERNET_UA });
      window.location.href = url;
      await import("./instrumentation-client");
      const { canPromptInstall } = await import("./lib/pwa-install-prompt");
      const event = new Event("beforeinstallprompt", { cancelable: true });

      window.dispatchEvent(event);

      expect(event.defaultPrevented).toBe(false);
      expect(canPromptInstall()).toBe(false);
    },
  );

  it("leaves Chrome's native install prompt enabled on desktop tenant hosts", async () => {
    vi.stubGlobal("navigator", { userAgent: DESKTOP_CHROME_UA });
    await import("./instrumentation-client");
    const { canPromptInstall } = await import("./lib/pwa-install-prompt");
    const event = Object.assign(
      new Event("beforeinstallprompt", { cancelable: true }),
      {
        prompt: vi.fn().mockResolvedValue(undefined),
        userChoice: Promise.resolve({ outcome: "accepted" as const }),
      },
    );

    window.dispatchEvent(event);

    expect(event.defaultPrevented).toBe(false);
    expect(canPromptInstall()).toBe(false);
  });

  it.each([
    "/",
    "/display",
    "/enroll",
    "/enroll/phase-1",
    "/invite",
    "/reset-password",
  ])(
    "leaves the native prompt uncaptured on public tenant path %s",
    async (path) => {
      window.location.href = `https://school-a.moto-app.de${path}`;
      await import("./instrumentation-client");
      const { canPromptInstall } = await import("./lib/pwa-install-prompt");
      const event = Object.assign(
        new Event("beforeinstallprompt", { cancelable: true }),
        {
          prompt: vi.fn().mockResolvedValue(undefined),
          userChoice: Promise.resolve({ outcome: "accepted" as const }),
        },
      );

      window.dispatchEvent(event);

      expect(event.defaultPrevented).toBe(false);
      expect(canPromptInstall()).toBe(false);
    },
  );

  it("leaves the native prompt enabled when the custom hint is not eligible yet", async () => {
    localStorage.clear();
    sessionStorage.clear();
    await import("./instrumentation-client");
    const { canPromptInstall } = await import("./lib/pwa-install-prompt");
    const event = Object.assign(
      new Event("beforeinstallprompt", { cancelable: true }),
      {
        prompt: vi.fn().mockResolvedValue(undefined),
        userChoice: Promise.resolve({ outcome: "accepted" as const }),
      },
    );

    window.dispatchEvent(event);

    expect(event.defaultPrevented).toBe(false);
    expect(canPromptInstall()).toBe(false);
    expect(localStorage.getItem("moto-pwa-install-hint-visits")).toBe("1");
  });

  it("leaves the native prompt enabled after the custom hint was dismissed", async () => {
    localStorage.setItem("moto-pwa-install-hint-dismissed", "1");
    await import("./instrumentation-client");
    const { canPromptInstall } = await import("./lib/pwa-install-prompt");
    const event = Object.assign(
      new Event("beforeinstallprompt", { cancelable: true }),
      {
        prompt: vi.fn().mockResolvedValue(undefined),
        userChoice: Promise.resolve({ outcome: "accepted" as const }),
      },
    );

    window.dispatchEvent(event);

    expect(event.defaultPrevented).toBe(false);
    expect(canPromptInstall()).toBe(false);
  });

  it.each(["/invitations", "/enrollment-form"])(
    "suppresses Chrome's native prompt on protected tenant path %s",
    async (path) => {
      window.location.href = `https://school-a.moto-app.de${path}`;
      await import("./instrumentation-client");
      const { canPromptInstall } = await import("./lib/pwa-install-prompt");
      const event = Object.assign(
        new Event("beforeinstallprompt", { cancelable: true }),
        {
          prompt: vi.fn().mockResolvedValue(undefined),
          userChoice: Promise.resolve({ outcome: "accepted" as const }),
        },
      );

      window.dispatchEvent(event);

      expect(event.defaultPrevented).toBe(true);
      expect(canPromptInstall()).toBe(true);
    },
  );

  it.each([
    "https://operator.moto-app.de/",
    "https://moto-app.de/",
    "https://help.moto-app.de/",
  ])("leaves Chrome's native install prompt enabled on %s", async (url) => {
    window.location.href = url;
    await import("./instrumentation-client");
    const { canPromptInstall } = await import("./lib/pwa-install-prompt");
    const event = Object.assign(
      new Event("beforeinstallprompt", { cancelable: true }),
      {
        prompt: vi.fn().mockResolvedValue(undefined),
        userChoice: Promise.resolve({ outcome: "accepted" as const }),
      },
    );

    window.dispatchEvent(event);

    expect(event.defaultPrevented).toBe(false);
    expect(canPromptInstall()).toBe(false);
  });

  // The parents app is installable in its own right, so it owns the prompt on
  // its host under exactly the tenant rules: Android, a protected path, and a
  // card that is eligible to render.
  it("suppresses Chrome's native prompt on the parents host", async () => {
    window.location.href = "https://eltern.moto-app.de/";
    await import("./instrumentation-client");
    const { canPromptInstall } = await import("./lib/pwa-install-prompt");
    const event = Object.assign(
      new Event("beforeinstallprompt", { cancelable: true }),
      {
        prompt: vi.fn().mockResolvedValue(undefined),
        userChoice: Promise.resolve({ outcome: "accepted" as const }),
      },
    );

    window.dispatchEvent(event);

    expect(event.defaultPrevented).toBe(true);
    expect(canPromptInstall()).toBe(true);
  });

  it.each(["/login", "/reset-password", "/enroll/status/abc"])(
    "leaves the native prompt uncaptured on public parent path %s",
    async (path) => {
      window.location.href = `https://eltern.moto-app.de${path}`;
      await import("./instrumentation-client");
      const { canPromptInstall } = await import("./lib/pwa-install-prompt");
      const event = Object.assign(
        new Event("beforeinstallprompt", { cancelable: true }),
        {
          prompt: vi.fn().mockResolvedValue(undefined),
          userChoice: Promise.resolve({ outcome: "accepted" as const }),
        },
      );

      window.dispatchEvent(event);

      expect(event.defaultPrevented).toBe(false);
      expect(canPromptInstall()).toBe(false);
    },
  );

  // Regression guard: without the eligibility check the parents host would lose
  // Chrome's prompt while our own card stays hidden, leaving no way to install.
  it("leaves the native prompt enabled on the parents host after a dismissal", async () => {
    window.location.href = "https://eltern.moto-app.de/";
    localStorage.setItem("moto-pwa-install-hint-dismissed", "1");
    await import("./instrumentation-client");
    const { canPromptInstall } = await import("./lib/pwa-install-prompt");
    const event = Object.assign(
      new Event("beforeinstallprompt", { cancelable: true }),
      {
        prompt: vi.fn().mockResolvedValue(undefined),
        userChoice: Promise.resolve({ outcome: "accepted" as const }),
      },
    );

    window.dispatchEvent(event);

    expect(event.defaultPrevented).toBe(false);
    expect(canPromptInstall()).toBe(false);
  });
});
