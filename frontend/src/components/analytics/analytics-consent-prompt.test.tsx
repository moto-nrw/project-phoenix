import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  env: {
    NEXT_PUBLIC_POSTHOG_KEY: "phc_test_key_123" as string | undefined,
    NEXT_PUBLIC_TENANT_DOMAIN: "localhost",
  },
  demo: false,
  setAnalyticsContext: vi.fn(),
}));

vi.mock("~/env.client", () => ({ clientEnv: mocks.env }));
vi.mock("~/lib/demo-access", () => ({ isDemoBuild: () => mocks.demo }));
vi.mock("~/lib/posthog-client", () => ({
  analyticsHostSurface: () => "ogs",
  setAnalyticsContext: mocks.setAnalyticsContext,
}));

function answer(granted: boolean | null, status = 200): Response {
  return Response.json(
    status === 200
      ? { data: { granted } }
      : { status, code: "general.internal" },
    { status },
  );
}

/** An OGS session whose stored answer and school switch are given. */
async function signedIn(granted: boolean | null, schoolAllows = true) {
  vi.mocked(fetch).mockResolvedValueOnce(answer(granted));
  const consent = await import("~/lib/analytics-consent");
  consent.startAnalyticsConsent("tenant", "7");
  consent.setAnalyticsConsentSchool("tenant", schoolAllows);
  await act(async () => {});
  const { AnalyticsConsentPrompt } = await import("./analytics-consent-prompt");
  const { germanAnalyticsConsentCopy } =
    await import("./analytics-consent-copy");
  // Same module instance as the component after vi.resetModules().
  const { ToastProvider } = await import("~/contexts/ToastContext");
  const copy = germanAnalyticsConsentCopy("Profil");
  return {
    consent,
    copy,
    Prompt: AnalyticsConsentPrompt,
    render: (recording = false) =>
      render(
        <AnalyticsConsentPrompt
          portal="tenant"
          copy={germanAnalyticsConsentCopy("Profil")}
          recording={recording}
        />,
        { wrapper: ToastProvider },
      ),
  };
}

function lastConsent(): unknown {
  return (
    mocks.setAnalyticsContext.mock.lastCall?.[0] as
      { consent?: boolean } | undefined
  )?.consent;
}

describe("AnalyticsConsentPrompt", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
    mocks.env.NEXT_PUBLIC_POSTHOG_KEY = "phc_test_key_123";
    mocks.demo = false;
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("asks an undecided person with two equal answers and nothing preselected", async () => {
    const view = await signedIn(null);
    view.render();

    const dialog = await screen.findByRole("dialog", {
      name: "Nutzungsanalyse erlauben?",
    });
    expect(dialog).toHaveTextContent(
      "Wenn Sie ablehnen, können Sie moto genauso nutzen.",
    );
    expect(dialog).toHaveTextContent(
      "Namen und Eingaben sieht moto dabei nicht.",
    );
    const allow = screen.getByRole("button", { name: "Erlauben" });
    const refuse = screen.getByRole("button", { name: "Ablehnen" });
    expect(allow.className).toBe(refuse.className);
    expect(document.activeElement).not.toBe(allow);
    expect(document.activeElement).not.toBe(refuse);
    // No recording sentence without the school's Analyse-Freigabe.
    expect(dialog).not.toHaveTextContent("Aufzeichnungen");
    expect(lastConsent()).toBe(false);
  });

  it("names the recordings while the school has the Analyse-Freigabe", async () => {
    const view = await signedIn(null);
    view.render(true);

    expect(
      await screen.findByText(
        /Ihre Schule hat zusätzlich Aufzeichnungen erlaubt/,
      ),
    ).toBeInTheDocument();
  });

  it.each([true, false])(
    "stores the answer and closes (%s)",
    async (granted) => {
      const view = await signedIn(null);
      view.render();
      await screen.findByRole("dialog");

      vi.mocked(fetch).mockResolvedValueOnce(answer(granted));
      fireEvent.click(
        screen.getByRole("button", { name: granted ? "Erlauben" : "Ablehnen" }),
      );

      await waitFor(() =>
        expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
      );
      expect(fetch).toHaveBeenLastCalledWith(
        "/api/analytics-consent",
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify({ granted }),
        }),
      );
      expect(lastConsent()).toBe(granted);
    },
  );

  it("closes without an answer and stores nothing", async () => {
    const view = await signedIn(null);
    view.render();
    await screen.findByRole("dialog");

    fireEvent.click(screen.getByRole("button", { name: "Später entscheiden" }));

    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    expect(fetch).toHaveBeenCalledOnce();
    expect(lastConsent()).toBe(false);
  });

  it("keeps the dialog with the reason when the answer cannot be saved", async () => {
    const view = await signedIn(null);
    view.render();
    await screen.findByRole("dialog");

    vi.mocked(fetch).mockResolvedValueOnce(answer(null, 500));
    fireEvent.click(screen.getByRole("button", { name: "Erlauben" }));

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(lastConsent()).toBe(false);
  });

  it.each([
    { name: "already allowed", granted: true, schoolAllows: true },
    { name: "already refused", granted: false, schoolAllows: true },
    { name: "school switched off", granted: null, schoolAllows: false },
  ])("does not ask when $name", async ({ granted, schoolAllows }) => {
    const view = await signedIn(granted, schoolAllows);
    view.render();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    // Counterpart in the same render: an undecided account of a school with
    // analytics is asked.
    vi.mocked(fetch).mockResolvedValueOnce(answer(null));
    await act(async () => {
      view.consent.stopAnalyticsConsent("tenant");
      view.consent.startAnalyticsConsent("tenant", "8");
      view.consent.setAnalyticsConsentSchool("tenant", true);
    });
    expect(
      await screen.findByRole("dialog", { name: "Nutzungsanalyse erlauben?" }),
    ).toBeInTheDocument();
  });

  it.each([
    {
      name: "without analytics on this deployment",
      key: undefined,
      demo: false,
    },
    { name: "in the public demo", key: "phc_test_key_123", demo: true },
  ])("does not ask $name", async ({ key, demo }) => {
    mocks.env.NEXT_PUBLIC_POSTHOG_KEY = key;
    mocks.demo = demo;
    const view = await signedIn(null);
    const rendered = view.render();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    // Counterpart: the same session on a school deployment with analytics.
    mocks.env.NEXT_PUBLIC_POSTHOG_KEY = "phc_test_key_123";
    mocks.demo = false;
    rendered.rerender(
      <view.Prompt portal="tenant" copy={view.copy} recording={false} />,
    );
    expect(
      await screen.findByRole("dialog", { name: "Nutzungsanalyse erlauben?" }),
    ).toBeInTheDocument();
  });
});
