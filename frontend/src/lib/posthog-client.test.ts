import type { BeforeSendFn, PostHogConfig } from "posthog-js";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  env: {
    NEXT_PUBLIC_POSTHOG_KEY: "phc_test_key_123" as string | undefined,
    NEXT_PUBLIC_TENANT_DOMAIN: "localhost",
    NEXT_PUBLIC_OPERATOR_HOSTNAME: "operator.localhost:3000",
    NEXT_PUBLIC_PARENTS_HOSTNAME: "parents.localhost:3000",
    NEXT_PUBLIC_SCHOOL_HOSTNAME: "schule.localhost:3000",
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

function stubHost(host: string): void {
  vi.stubGlobal("location", {
    ...window.location,
    host,
    hostname: host.split(":")[0],
  });
}

function initOptions(): Partial<PostHogConfig> {
  return mocks.init.mock.calls[0]?.[1] as Partial<PostHogConfig>;
}

describe("posthog-client", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
    mocks.env.NEXT_PUBLIC_POSTHOG_KEY = "phc_test_key_123";
    stubHost("school-a.localhost:3000");
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

  it("queues capture until initialization and asks the analytics module", async () => {
    const { capturePostHog, initializePostHog } =
      await import("./posthog-client");

    capturePostHog("data_exported", { format: "xlsx" });
    expect(mocks.capture).not.toHaveBeenCalled();

    await initializePostHog();

    expect(mocks.init).toHaveBeenCalledWith(
      "phc_test_key_123",
      expect.objectContaining({
        api_host: "/ingest",
        autocapture: true,
        mask_all_text: true,
        capture_pageview: "history_change",
        disable_session_recording: true,
        persistence: "memory",
        disable_persistence: true,
        person_profiles: "never",
        advanced_disable_feature_flags: true,
        tracing_headers: ["school-a.localhost"],
        before_send: expect.any(Function),
      }),
    );
    expect(initOptions()).not.toHaveProperty("advanced_disable_flags");
    expect(mocks.capture).toHaveBeenCalledWith("data_exported", {
      format: "xlsx",
    });
  });

  it("filters with the context the portal registered last", async () => {
    const { initializePostHog, setAnalyticsSurface, clearPostHogContext } =
      await import("./posthog-client");
    await initializePostHog();
    const beforeSend = initOptions().before_send as BeforeSendFn;
    const pageview = () =>
      beforeSend({
        uuid: "u",
        event: "$pageview",
        properties: {
          $pathname: "/children/42",
          $current_url: "http://school-a.localhost:3000/children/42",
        },
      });

    // The tenant host starts as the OGS portal, which has no /children page.
    expect(pageview()?.properties).toMatchObject({
      surface: "ogs",
      $pathname: "/unknown",
    });

    setAnalyticsSurface("parents");
    expect(pageview()?.properties).toMatchObject({
      surface: "parents",
      $pathname: "/children/:id",
      deployment: "localhost",
    });

    clearPostHogContext();
    expect(pageview()?.properties).toMatchObject({ surface: "ogs" });
  });

  it("clears school and role and resets the identity at logout", async () => {
    const { initializePostHog, clearPostHogContext } =
      await import("./posthog-client");
    await initializePostHog();

    clearPostHogContext();

    expect(mocks.unregister).toHaveBeenCalledWith("school_id");
    expect(mocks.unregister).toHaveBeenCalledWith("role");
    expect(mocks.reset).toHaveBeenCalledOnce();
  });

  it("never loads the SDK on the operator host", async () => {
    stubHost("operator.localhost:3000");
    const requestIdleCallback = vi.fn();
    vi.stubGlobal("requestIdleCallback", requestIdleCallback);
    const { capturePostHog, initializePostHog, schedulePostHogInitialization } =
      await import("./posthog-client");

    schedulePostHogInitialization();
    capturePostHog("login_success");
    await initializePostHog();

    expect(requestIdleCallback).not.toHaveBeenCalled();
    expect(mocks.init).not.toHaveBeenCalled();
    expect(mocks.capture).not.toHaveBeenCalled();
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

  // Staging and local development leave the key empty: nothing is sent.
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
