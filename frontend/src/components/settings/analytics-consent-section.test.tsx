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
  setAnalyticsContext: vi.fn(),
  useTenant: vi.fn(),
}));

vi.mock("~/env.client", () => ({ clientEnv: mocks.env }));
vi.mock("~/lib/demo-access", () => ({ isDemoBuild: () => false }));
vi.mock("~/lib/posthog-client", () => ({
  analyticsHostSurface: () => "ogs",
  setAnalyticsContext: mocks.setAnalyticsContext,
}));
vi.mock("~/lib/tenant-context", () => ({ useTenant: mocks.useTenant }));

function answer(granted: boolean | null, status = 200): Response {
  return Response.json(
    status === 200
      ? { data: { granted } }
      : { status, code: "general.internal" },
    { status },
  );
}

function lastConsent(): unknown {
  return (
    mocks.setAnalyticsContext.mock.lastCall?.[0] as
      { consent?: boolean } | undefined
  )?.consent;
}

/** The OGS profile card of a session whose stored answer is given. */
async function renderCard(
  stored: Response | null,
  { schoolAllows = true, freigabe = false } = {},
) {
  mocks.useTenant.mockReturnValue({
    tenant: { analyticsEnabled: true, analyticsFreigabe: freigabe },
  });
  const consent = await import("~/lib/analytics-consent");
  if (stored) {
    vi.mocked(fetch).mockResolvedValueOnce(stored);
    consent.startAnalyticsConsent("tenant", "7");
    consent.setAnalyticsConsentSchool("tenant", schoolAllows);
  }
  const { TenantAnalyticsConsentSection } =
    await import("./analytics-consent-section");
  // Same module instance as the component after vi.resetModules().
  const { ToastProvider } = await import("~/contexts/ToastContext");
  const view = render(<TenantAnalyticsConsentSection />, {
    wrapper: ToastProvider,
  });
  await act(async () => {});
  return view;
}

describe("AnalyticsConsentSection", () => {
  beforeEach(() => {
    vi.resetModules();
    vi.clearAllMocks();
    mocks.env.NEXT_PUBLIC_POSTHOG_KEY = "phc_test_key_123";
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(async () => {
    // Ends the session of this test's module, with its visibility listener.
    const consent = await import("~/lib/analytics-consent");
    consent.stopAnalyticsConsent("tenant");
    vi.unstubAllGlobals();
  });

  it("shows an undecided person the switch off and stores a consent", async () => {
    await renderCard(answer(null));

    const toggle = screen.getByRole("switch", {
      name: "Nutzungsanalyse erlauben",
    });
    expect(toggle).toHaveAttribute("aria-checked", "false");

    vi.mocked(fetch).mockResolvedValueOnce(answer(true));
    fireEvent.click(toggle);

    await waitFor(() => expect(toggle).toHaveAttribute("aria-checked", "true"));
    expect(fetch).toHaveBeenLastCalledWith(
      "/api/analytics-consent",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({ granted: true }),
      }),
    );
    expect(lastConsent()).toBe(true);
  });

  it("revokes at once, before the refusal is saved", async () => {
    await renderCard(answer(true));
    const toggle = screen.getByRole("switch", {
      name: "Nutzungsanalyse erlauben",
    });
    expect(toggle).toHaveAttribute("aria-checked", "true");

    let save!: (response: Response) => void;
    vi.mocked(fetch).mockReturnValueOnce(
      new Promise((resolve) => {
        save = resolve;
      }),
    );
    fireEvent.click(toggle);
    expect(lastConsent()).toBe(false);

    await act(async () => save(answer(false)));
    expect(toggle).toHaveAttribute("aria-checked", "false");
  });

  it("reports a failed save as a toast and keeps the stored answer", async () => {
    await renderCard(answer(null));
    const toggle = screen.getByRole("switch", {
      name: "Nutzungsanalyse erlauben",
    });

    vi.mocked(fetch).mockResolvedValueOnce(answer(null, 500));
    fireEvent.click(toggle);

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(toggle).toHaveAttribute("aria-checked", "false");
    expect(lastConsent()).toBe(false);
  });

  it("says when the school has switched the analytics off", async () => {
    await renderCard(answer(true), { schoolAllows: false });

    expect(
      screen.getByText(/Ihre Schule hat die Nutzungsanalyse ausgeschaltet/),
    ).toBeInTheDocument();
    // The choice stays the person's own.
    expect(screen.getByRole("switch")).toHaveAttribute("aria-checked", "true");
  });

  it("names the recordings only with the school's Analyse-Freigabe", async () => {
    await renderCard(answer(null), { freigabe: true });
    expect(
      screen.getByText(/Ihre Schule hat zusätzlich Aufzeichnungen erlaubt/),
    ).toBeInTheDocument();
  });

  it("shows a failed read with retry instead of a guessed switch", async () => {
    await renderCard(answer(null, 500));

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.queryByRole("switch")).not.toBeInTheDocument();

    vi.mocked(fetch).mockResolvedValueOnce(answer(false));
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(await screen.findByRole("switch")).toHaveAttribute(
      "aria-checked",
      "false",
    );
  });

  it("stays hidden without a portal session of this host", async () => {
    await renderCard(null);
    expect(
      screen.queryByRole("heading", { name: "Nutzungsanalyse" }),
    ).not.toBeInTheDocument();

    // Counterpart in the same render: a signed-in session shows the card.
    vi.mocked(fetch).mockResolvedValueOnce(answer(null));
    const consent = await import("~/lib/analytics-consent");
    await act(async () => consent.startAnalyticsConsent("tenant", "7"));
    expect(
      await screen.findByRole("heading", { name: "Nutzungsanalyse" }),
    ).toBeInTheDocument();
  });

  it("stays hidden without analytics on this deployment", async () => {
    mocks.env.NEXT_PUBLIC_POSTHOG_KEY = undefined;
    const view = await renderCard(answer(true));
    expect(
      screen.queryByRole("heading", { name: "Nutzungsanalyse" }),
    ).not.toBeInTheDocument();

    // Counterpart: the same session on a deployment with analytics.
    mocks.env.NEXT_PUBLIC_POSTHOG_KEY = "phc_test_key_123";
    const { TenantAnalyticsConsentSection } =
      await import("./analytics-consent-section");
    view.rerender(<TenantAnalyticsConsentSection />);
    expect(
      screen.getByRole("heading", { name: "Nutzungsanalyse" }),
    ).toBeInTheDocument();
  });
});
