import type { BeforeSendFn, PostHogConfig } from "posthog-js";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  env: {
    NEXT_PUBLIC_POSTHOG_KEY: "phc_test_key_123" as string | undefined,
    NEXT_PUBLIC_TENANT_DOMAIN: "localhost",
    NEXT_PUBLIC_OPERATOR_HOSTNAME: "operator.localhost:3000",
    NEXT_PUBLIC_PARENTS_HOSTNAME: "parents.localhost:3000",
    NEXT_PUBLIC_SCHOOL_HOSTNAME: "schule.localhost:3000",
    NEXT_PUBLIC_TRAEGER_HOSTNAME: "traeger.localhost:3000",
  },
  init: vi.fn(),
  capture: vi.fn(),
  register: vi.fn(),
  reset: vi.fn(),
  unregister: vi.fn(),
  setConfig: vi.fn(),
  identify: vi.fn(),
  stopRecording: vi.fn(),
  optOut: vi.fn(),
  optIn: vi.fn(),
  distinctId: vi.fn(() => "0199aa2b-anon"),
}));

vi.mock("~/env.client", () => ({ clientEnv: mocks.env }));
vi.mock("posthog-js", () => ({
  default: {
    init: mocks.init,
    stopSessionRecording: mocks.stopRecording,
    opt_out_capturing: mocks.optOut,
    opt_in_capturing: mocks.optIn,
    capture: mocks.capture,
    register: mocks.register,
    reset: mocks.reset,
    unregister: mocks.unregister,
    set_config: mocks.setConfig,
    identify: mocks.identify,
    get_distinct_id: mocks.distinctId,
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
  beforeEach(async () => {
    vi.resetModules();
    vi.clearAllMocks();
    mocks.env.NEXT_PUBLIC_POSTHOG_KEY = "phc_test_key_123";
    mocks.distinctId.mockReturnValue("0199aa2b-anon");
    stubHost("school-a.localhost:3000");
    vi.stubGlobal("requestIdleCallback", vi.fn());
    const { setAnalyticsContext } = await import("./posthog-client");
    setAnalyticsContext({ enabled: true, consent: true });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it.each([false, undefined])(
    "never initializes while the school switch is %s",
    async (enabled) => {
      const {
        setAnalyticsContext,
        initializePostHog,
        capturePostHog,
        schedulePostHogInitialization,
      } = await import("./posthog-client");
      setAnalyticsContext({ enabled });
      schedulePostHogInitialization();
      capturePostHog("login_failed");
      await initializePostHog();
      expect(mocks.init).not.toHaveBeenCalled();
      expect(mocks.capture).not.toHaveBeenCalled();
    },
  );

  it.each([false, undefined])(
    "never loads the SDK while the person's consent is %s",
    async (consent) => {
      const {
        setAnalyticsContext,
        initializePostHog,
        capturePostHog,
        schedulePostHogInitialization,
      } = await import("./posthog-client");
      setAnalyticsContext({ consent });
      schedulePostHogInitialization();
      capturePostHog("login_failed");
      await initializePostHog();
      expect(mocks.init).not.toHaveBeenCalled();
      expect(mocks.capture).not.toHaveBeenCalled();
    },
  );

  it("sends the session header only after consent and ends it on revocation", async () => {
    const { setAnalyticsContext, initializePostHog, capturePostHog } =
      await import("./posthog-client");
    await initializePostHog();
    expect(initOptions().tracing_headers).toEqual(["school-a.localhost"]);

    setAnalyticsContext({ consent: false });
    capturePostHog("login_failed");

    expect(mocks.optOut).toHaveBeenCalledOnce();
    expect(mocks.stopRecording).toHaveBeenCalledOnce();
    expect(mocks.setConfig).toHaveBeenLastCalledWith(
      expect.objectContaining({
        tracing_headers: [],
        disable_session_recording: true,
      }),
    );
    expect(mocks.capture).not.toHaveBeenCalled();
    const beforeSend = initOptions().before_send as BeforeSendFn;
    expect(
      beforeSend({ uuid: "u", event: "$pageview", properties: {} }),
    ).toBeNull();

    setAnalyticsContext({ consent: true });
    expect(mocks.optIn).toHaveBeenLastCalledWith({ captureEventName: false });
    expect(mocks.setConfig).toHaveBeenLastCalledWith(
      expect.objectContaining({ tracing_headers: ["school-a.localhost"] }),
    );
  });

  it("registers the session after an embedded school's veto is removed", async () => {
    const { setTenantAnalyticsPolicy, setAnalyticsContext, initializePostHog } =
      await import("./posthog-client");
    const host = Symbol();
    const embedded = Symbol();
    setTenantAnalyticsPolicy(host, { tenantSlug: "school-a", enabled: true });
    setTenantAnalyticsPolicy(embedded, {
      tenantSlug: "school-b",
      enabled: false,
    });
    setAnalyticsContext(
      { surface: "ogs", enabled: true, role: "staff" },
      {
        properties: { school_id: "42", role: "staff" },
        resetFirst: true,
      },
    );
    await initializePostHog();
    expect(mocks.init).not.toHaveBeenCalled();
    setTenantAnalyticsPolicy(embedded, null);
    await initializePostHog();
    expect(mocks.init).toHaveBeenCalledOnce();
    expect(mocks.register).toHaveBeenCalledWith({
      school_id: "42",
      role: "staff",
    });
    expect(mocks.reset).toHaveBeenCalledOnce();
  });

  it("stops collection and drops future events when a school opts out", async () => {
    const { setAnalyticsContext, initializePostHog, capturePostHog } =
      await import("./posthog-client");
    await initializePostHog();
    expect(mocks.init).toHaveBeenCalledOnce();
    setAnalyticsContext({ enabled: false });
    capturePostHog("login_failed");
    expect(mocks.stopRecording).toHaveBeenCalledOnce();
    expect(mocks.optOut).toHaveBeenCalledOnce();
    expect(mocks.capture).not.toHaveBeenCalled();
    const beforeSend = initOptions().before_send as BeforeSendFn;
    expect(
      beforeSend({ uuid: "u", event: "$pageview", properties: {} }),
    ).toBeNull();
  });

  it("keeps collection off even if SDK shutdown throws", async () => {
    const { setAnalyticsContext, initializePostHog, capturePostHog } =
      await import("./posthog-client");
    await initializePostHog();
    expect(mocks.init).toHaveBeenCalledOnce();
    mocks.stopRecording.mockImplementationOnce(() => {
      throw new Error("recorder unavailable");
    });
    expect(() => setAnalyticsContext({ enabled: false })).not.toThrow();
    capturePostHog("login_failed");
    expect(mocks.optOut).toHaveBeenCalledOnce();
    expect(mocks.capture).not.toHaveBeenCalled();
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
        // The OGS portal may learn the Analyse-Freigabe after the start
        // (#3603); without identify no person is processed either.
        person_profiles: "identified_only",
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

  it("rejects another portal's session on the OGS host", async () => {
    const { initializePostHog, setAnalyticsContext, clearPostHogContext } =
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

    setAnalyticsContext({ surface: "parents" });
    expect(pageview()?.properties).toMatchObject({
      surface: "ogs",
      $pathname: "/unknown",
      deployment: "localhost",
    });

    // Logout ends the session's consent (#4901): the anonymous host
    // collects nothing, whatever the previous portal was.
    clearPostHogContext();
    expect(pageview()).toBeNull();
  });

  it.each(["parents", "school"] as const)(
    "restores collection after initialization on the %s portal",
    async (surface) => {
      vi.resetModules();
      stubHost(
        `${surface === "parents" ? "parents" : "schule"}.localhost:3000`,
      );
      const { initializePostHog, setAnalyticsContext } =
        await import("./posthog-client");
      setAnalyticsContext({ surface, enabled: true, consent: true });
      await initializePostHog();

      expect(mocks.init).toHaveBeenCalledOnce();
      expect(mocks.optIn).toHaveBeenCalledWith({ captureEventName: false });
      expect(mocks.optIn.mock.invocationCallOrder[0]).toBeGreaterThan(
        mocks.init.mock.invocationCallOrder[0]!,
      );
    },
  );

  it.each(["parents", "school"] as const)(
    "clears the previous %s session even while collection is blocked",
    async (surface) => {
      vi.resetModules();
      stubHost(
        `${surface === "parents" ? "parents" : "schule"}.localhost:3000`,
      );
      const { initializePostHog, setAnalyticsContext, clearPostHogContext } =
        await import("./posthog-client");
      setAnalyticsContext(
        { surface, enabled: true, consent: true },
        {
          properties: { role: "previous-role" },
          resetFirst: false,
        },
      );
      await initializePostHog();
      expect(mocks.register).toHaveBeenCalledWith({ role: "previous-role" });
      setAnalyticsContext({ enabled: false });
      clearPostHogContext();

      expect(mocks.unregister).toHaveBeenCalledWith("school_id");
      expect(mocks.unregister).toHaveBeenCalledWith("role");
      expect(mocks.reset).toHaveBeenCalledOnce();
      expect(mocks.setConfig).toHaveBeenLastCalledWith(
        expect.objectContaining({ disable_session_recording: true }),
      );
    },
  );

  it("discards a logged-out session's buffered registration before SDK initialization", async () => {
    vi.resetModules();
    stubHost("parents.localhost:3000");
    const { initializePostHog, setAnalyticsContext, clearPostHogContext } =
      await import("./posthog-client");
    setAnalyticsContext(
      { surface: "parents", enabled: true, consent: true },
      {
        properties: { role: "previous-role" },
        resetFirst: false,
      },
    );
    clearPostHogContext();
    setAnalyticsContext(
      { surface: "parents", enabled: true, consent: true },
      {
        properties: { role: "guardian" },
        resetFirst: false,
      },
    );
    await initializePostHog();

    expect(mocks.register).toHaveBeenCalledExactlyOnceWith({
      role: "guardian",
    });
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

  it("records and identifies only while the context has the Analyse-Freigabe", async () => {
    const person = "pseudo_4b9630678fc1afce76ce690721aaf949";
    const { initializePostHog, setAnalyticsContext, clearPostHogContext } =
      await import("./posthog-client");
    await initializePostHog();
    const lastConfig = () =>
      mocks.setConfig.mock.lastCall?.[0] as Partial<PostHogConfig>;
    expect(lastConfig()).toMatchObject({ disable_session_recording: true });

    setAnalyticsContext({
      surface: "ogs",
      role: "staff",
      analyseFreigabe: true,
      recordingSamplePercent: 25,
      person,
    });

    expect(lastConfig()).toMatchObject({
      disable_session_recording: false,
      enable_recording_console_log: false,
      session_recording: {
        maskAllInputs: true,
        maskTextSelector: "*",
        sampleRate: 0.25,
      },
    });
    expect(mocks.identify).toHaveBeenCalledWith(person, { role: "staff" });

    mocks.distinctId.mockReturnValue(person);
    clearPostHogContext();

    expect(mocks.reset).toHaveBeenCalled();
    expect(lastConfig()).toMatchObject({ disable_session_recording: true });
  });

  it("drops a pseudonymous identity when the Freigabe is revoked", async () => {
    const person = "pseudo_4b9630678fc1afce76ce690721aaf949";
    mocks.distinctId.mockReturnValue(person);
    const { initializePostHog, setAnalyticsContext } =
      await import("./posthog-client");
    await initializePostHog();
    mocks.reset.mockClear();

    setAnalyticsContext({ analyseFreigabe: false, person });

    expect(mocks.reset).toHaveBeenCalledOnce();
    expect(mocks.identify).not.toHaveBeenCalled();
  });

  it("never loads the SDK on the operator host", async () => {
    vi.resetModules();
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
