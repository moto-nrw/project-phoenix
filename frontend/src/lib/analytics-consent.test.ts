import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  env: {
    NEXT_PUBLIC_POSTHOG_KEY: "phc_test_key_123" as string | undefined,
    NEXT_PUBLIC_TENANT_DOMAIN: "localhost",
  },
  demo: false,
  hostSurface: "ogs" as string | null,
  setAnalyticsContext: vi.fn(),
}));

vi.mock("~/env.client", () => ({ clientEnv: mocks.env }));
vi.mock("~/lib/demo-access", () => ({ isDemoBuild: () => mocks.demo }));
vi.mock("~/lib/posthog-client", () => ({
  analyticsHostSurface: () => mocks.hostSurface,
  setAnalyticsContext: mocks.setAnalyticsContext,
}));

type Consent = typeof import("./analytics-consent");

function answer(granted: boolean | null, status = 200): Response {
  return new Response(
    status === 200
      ? JSON.stringify({ success: true, data: { granted } })
      : JSON.stringify({ status, code: "general.internal" }),
    { status, headers: { "Content-Type": "application/json" } },
  );
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

/** The consent values handed to the analytics context, in order. */
function published(): unknown[] {
  return mocks.setAnalyticsContext.mock.calls.map(
    ([update]) => (update as { consent?: boolean }).consent,
  );
}

async function load(): Promise<Consent> {
  return import("./analytics-consent");
}

describe("analytics consent", () => {
  const fetchMock = vi.fn<typeof fetch>();

  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
    fetchMock.mockReset();
    mocks.env.NEXT_PUBLIC_POSTHOG_KEY = "phc_test_key_123";
    mocks.demo = false;
    mocks.hostSurface = "ogs";
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(async () => {
    // Ends the session of this test's module, with its visibility listener.
    const consent = await load();
    for (const portal of ["tenant", "parent", "school"] as const)
      consent.stopAnalyticsConsent(portal);
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it.each([
    { granted: true, consent: true },
    { granted: false, consent: false },
    { granted: null, consent: false },
  ])(
    "keeps collection off until the stored answer arrives ($granted)",
    async ({ granted, consent }) => {
      const reply = deferred<Response>();
      fetchMock.mockReturnValueOnce(reply.promise);
      const consentModule = await load();

      consentModule.startAnalyticsConsent("tenant", "7");
      expect(published()).toEqual([false]);
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/analytics-consent",
        expect.objectContaining({ cache: "no-store" }),
      );

      reply.resolve(answer(granted));
      await vi.waitFor(() => expect(published()).toEqual([false, consent]));
    },
  );

  it("reads the portal's own answer only on its own host", async () => {
    mocks.hostSurface = "parents";
    const { startAnalyticsConsent } = await load();

    startAnalyticsConsent("tenant", "7");
    // Anker: the parents portal on its own host does read.
    fetchMock.mockResolvedValueOnce(answer(true));
    startAnalyticsConsent("parent", "7");

    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/parent/analytics-consent",
      expect.anything(),
    );
  });

  it("denies collection when the answer cannot be read, and retries", async () => {
    mocks.hostSurface = "school";
    fetchMock.mockResolvedValueOnce(answer(null, 500));
    const { startAnalyticsConsent, loadAnalyticsConsent } = await load();

    startAnalyticsConsent("school", "9");
    await vi.waitFor(() => expect(published()).toEqual([false, false]));
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/school/analytics-consent",
      expect.anything(),
    );

    fetchMock.mockResolvedValueOnce(answer(true));
    await loadAnalyticsConsent();
    expect(published()).toEqual([false, false, true]);
  });

  it("stops collection before a refusal is saved, allows only after a consent is saved", async () => {
    fetchMock.mockResolvedValueOnce(answer(true));
    const { startAnalyticsConsent, decideAnalyticsConsent } = await load();
    startAnalyticsConsent("tenant", "7");
    await vi.waitFor(() => expect(published()).toEqual([false, true]));

    const refusal = deferred<Response>();
    fetchMock.mockReturnValueOnce(refusal.promise);
    const revoking = decideAnalyticsConsent(false);
    expect(published()).toEqual([false, true, false]);
    expect(fetchMock).toHaveBeenLastCalledWith(
      "/api/analytics-consent",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({ granted: false }),
      }),
    );
    refusal.resolve(answer(false));
    await revoking;
    expect(published()).toEqual([false, true, false, false]);

    const grant = deferred<Response>();
    fetchMock.mockReturnValueOnce(grant.promise);
    const granting = decideAnalyticsConsent(true);
    expect(published()).toEqual([false, true, false, false]);
    grant.resolve(answer(true));
    await granting;
    expect(published()).toEqual([false, true, false, false, true]);
  });

  it("keeps collection off and reports the error when a consent cannot be saved", async () => {
    fetchMock.mockResolvedValueOnce(answer(null));
    const { startAnalyticsConsent, decideAnalyticsConsent } = await load();
    startAnalyticsConsent("tenant", "7");
    await vi.waitFor(() => expect(published()).toEqual([false, false]));

    fetchMock.mockResolvedValueOnce(answer(null, 500));
    await expect(decideAnalyticsConsent(true)).rejects.toMatchObject({
      status: 500,
    });
    expect(published()).toEqual([false, false]);
  });

  it("ignores a read that a newer decision overtook", async () => {
    const stale = deferred<Response>();
    fetchMock.mockReturnValueOnce(stale.promise);
    const { startAnalyticsConsent, decideAnalyticsConsent } = await load();
    startAnalyticsConsent("tenant", "7");

    fetchMock.mockResolvedValueOnce(answer(false));
    await decideAnalyticsConsent(false);
    stale.resolve(answer(true));
    await new Promise((done) => setTimeout(done, 0));

    expect(published()).not.toContain(true);
  });

  it("forgets the answer at logout and republishes it for the same session", async () => {
    fetchMock.mockResolvedValueOnce(answer(true));
    const { startAnalyticsConsent, stopAnalyticsConsent } = await load();
    startAnalyticsConsent("tenant", "7");
    await vi.waitFor(() => expect(published()).toEqual([false, true]));

    // A cleared analytics context of the same session gets the answer back.
    startAnalyticsConsent("tenant", "7");
    expect(published()).toEqual([false, true, true]);
    expect(fetchMock).toHaveBeenCalledOnce();

    stopAnalyticsConsent("tenant");
    expect(published()).toEqual([false, true, true, false]);
    fetchMock.mockResolvedValueOnce(answer(null));
    startAnalyticsConsent("tenant", "8");
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("follows a revocation from another tab when the page becomes visible", async () => {
    fetchMock.mockResolvedValueOnce(answer(true));
    const { startAnalyticsConsent, stopAnalyticsConsent } = await load();
    startAnalyticsConsent("tenant", "7");
    await vi.waitFor(() => expect(published()).toEqual([false, true]));

    // The other tab revoked; this one is hidden, so it does not ask yet.
    const visibility = vi.spyOn(document, "visibilityState", "get");
    visibility.mockReturnValue("hidden");
    document.dispatchEvent(new Event("visibilitychange"));
    expect(fetchMock).toHaveBeenCalledOnce();

    fetchMock.mockResolvedValueOnce(answer(false));
    visibility.mockReturnValue("visible");
    document.dispatchEvent(new Event("visibilitychange"));
    await vi.waitFor(() => expect(published()).toEqual([false, true, false]));

    // A failed re-read denies as well.
    fetchMock.mockResolvedValueOnce(answer(null, 500));
    document.dispatchEvent(new Event("visibilitychange"));
    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
    expect(published().at(-1)).toBe(false);

    // Logout stops the re-reads.
    stopAnalyticsConsent("tenant");
    document.dispatchEvent(new Event("visibilitychange"));
    expect(fetchMock).toHaveBeenCalledTimes(3);
    visibility.mockRestore();
  });

  it("keeps collection off after a failed re-read until a read succeeds", async () => {
    fetchMock.mockResolvedValueOnce(answer(true));
    const { startAnalyticsConsent } = await load();
    startAnalyticsConsent("tenant", "7");
    await vi.waitFor(() => expect(published()).toEqual([false, true]));

    fetchMock.mockResolvedValueOnce(answer(null, 500));
    vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
    document.dispatchEvent(new Event("visibilitychange"));
    await vi.waitFor(() => expect(published()).toEqual([false, true, false]));

    // A remount of the same session must not restore the stale answer.
    startAnalyticsConsent("tenant", "7");
    expect(published().at(-1)).toBe(false);

    fetchMock.mockResolvedValueOnce(answer(true));
    document.dispatchEvent(new Event("visibilitychange"));
    await vi.waitFor(() => expect(published().at(-1)).toBe(true));
  });

  it("keeps a revocation that could not be saved in force on this page", async () => {
    fetchMock.mockResolvedValueOnce(answer(true));
    const { startAnalyticsConsent, decideAnalyticsConsent } = await load();
    startAnalyticsConsent("tenant", "7");
    await vi.waitFor(() => expect(published()).toEqual([false, true]));

    fetchMock.mockResolvedValueOnce(answer(null, 500));
    await expect(decideAnalyticsConsent(false)).rejects.toMatchObject({
      status: 500,
    });
    expect(published().at(-1)).toBe(false);

    // Neither the session's republish nor a re-read with the old stored
    // answer turns collection back on.
    startAnalyticsConsent("tenant", "7");
    expect(published().at(-1)).toBe(false);
    fetchMock.mockResolvedValueOnce(answer(true));
    vi.spyOn(document, "visibilityState", "get").mockReturnValue("visible");
    document.dispatchEvent(new Event("visibilitychange"));
    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
    await new Promise((done) => setTimeout(done, 0));
    expect(published().at(-1)).toBe(false);

    // A saved answer ends the block.
    fetchMock.mockResolvedValueOnce(answer(true));
    await decideAnalyticsConsent(true);
    expect(published().at(-1)).toBe(true);
  });

  it.each([
    { key: "phc_test_key_123", demo: false, applies: true },
    { key: undefined, demo: false, applies: false },
    { key: "phc_test_key_123", demo: true, applies: false },
  ])(
    "asks only on a school deployment with analytics ($key, demo $demo)",
    async ({ key, demo, applies }) => {
      mocks.env.NEXT_PUBLIC_POSTHOG_KEY = key;
      mocks.demo = demo;
      const { analyticsConsentApplies } = await load();
      expect(analyticsConsentApplies()).toBe(applies);
    },
  );
});
