import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  env: {
    NEXT_PUBLIC_POSTHOG_KEY: "phc_test_key_123" as string | undefined,
    NEXT_PUBLIC_POSTHOG_HOST: "https://eu.i.posthog.com" as string | undefined,
  },
  init: vi.fn(),
  capture: vi.fn(),
  register: vi.fn(),
  reset: vi.fn(),
  unregister: vi.fn(),
}));

vi.mock("~/env.client", () => ({ clientEnv: mocks.env }));
vi.mock("posthog-js", () => ({
  default: {
    init: mocks.init,
    capture: mocks.capture,
    register: mocks.register,
    reset: mocks.reset,
    unregister: mocks.unregister,
  },
}));

describe("posthog-client", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
    mocks.env.NEXT_PUBLIC_POSTHOG_KEY = "phc_test_key_123";
    mocks.env.NEXT_PUBLIC_POSTHOG_HOST = "https://eu.i.posthog.com";
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("loads the SDK only from the idle callback", async () => {
    let idleCallback: IdleRequestCallback | undefined;
    vi.stubGlobal(
      "requestIdleCallback",
      vi.fn((callback: IdleRequestCallback) => {
        idleCallback = callback;
        return 1;
      }),
    );
    const { schedulePostHogInitialization } = await import("./posthog-client");

    schedulePostHogInitialization();

    expect(mocks.init).not.toHaveBeenCalled();
    idleCallback?.({ didTimeout: false, timeRemaining: () => 50 });
    await vi.waitFor(() => expect(mocks.init).toHaveBeenCalledOnce());
  });

  it("queues capture until initialization and preserves the privacy config", async () => {
    const { capturePostHog, initializePostHog } =
      await import("./posthog-client");

    capturePostHog("data_exported", { format: "xlsx" });
    expect(mocks.capture).not.toHaveBeenCalled();

    await initializePostHog();

    expect(mocks.init).toHaveBeenCalledWith(
      "phc_test_key_123",
      expect.objectContaining({
        api_host: "https://eu.i.posthog.com",
        autocapture: false,
        capture_pageview: false,
        disable_session_recording: true,
        persistence: "memory",
        disable_persistence: true,
        person_profiles: "never",
        advanced_disable_flags: true,
        before_send: expect.any(Function),
      }),
    );
    expect(mocks.capture).toHaveBeenCalledWith("data_exported", {
      format: "xlsx",
    });
  });

  it("drops a tenant-switch capture when its identity reset fails", async () => {
    mocks.reset.mockImplementationOnce(() => {
      throw new Error("reset failed");
    });
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { initializePostHog, resetAndCapturePostHog } =
      await import("./posthog-client");

    resetAndCapturePostHog("tenant_switched", { school_id: "42" });
    await initializePostHog();

    expect(mocks.capture).not.toHaveBeenCalled();
    expect(warnSpy).toHaveBeenCalledWith("posthog_operation_failed", {
      operation: "reset_and_capture",
      error: "reset failed",
    });
    warnSpy.mockRestore();
  });

  it("logs initialization failures and drops queued work", async () => {
    mocks.init.mockImplementationOnce(() => {
      throw new Error("init failed");
    });
    const warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
    const { capturePostHog, initializePostHog } =
      await import("./posthog-client");

    capturePostHog("login_success");
    await initializePostHog();
    capturePostHog("login_failed");

    expect(mocks.capture).not.toHaveBeenCalled();
    expect(warnSpy).toHaveBeenCalledWith("posthog_initialization_failed", {
      error: "init failed",
    });
    warnSpy.mockRestore();
  });

  it("does not schedule or queue work when analytics is disabled", async () => {
    mocks.env.NEXT_PUBLIC_POSTHOG_KEY = undefined;
    const requestIdleCallback = vi.fn();
    vi.stubGlobal("requestIdleCallback", requestIdleCallback);
    const { capturePostHog, schedulePostHogInitialization } =
      await import("./posthog-client");

    capturePostHog("login_success");
    schedulePostHogInitialization();

    expect(requestIdleCallback).not.toHaveBeenCalled();
    expect(mocks.capture).not.toHaveBeenCalled();
  });
});
