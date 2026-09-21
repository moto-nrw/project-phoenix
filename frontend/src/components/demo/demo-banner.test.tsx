import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderToString } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { saveDemoVisit } from "~/lib/demo-access";
import { DemoBanner } from "./demo-banner";

const signIn = vi.fn();
vi.mock("next-auth/react", () => ({
  signIn: (...args: unknown[]) => signIn(...args) as unknown,
}));

const analytics = vi.hoisted(() => ({
  trackDemoEvent: vi.fn(),
  registerDemoVisit: vi.fn(),
}));
vi.mock("~/lib/analytics", () => analytics);

const tenant = vi.hoisted(() => ({
  useTenantSafe: vi.fn<() => unknown>(),
  useTenantSlugSafe: vi.fn(() => "ogs-beispiel-a1b2c3"),
  useTenantRoutingModeSafe: vi.fn(() => "subdomain"),
}));
vi.mock("~/lib/tenant-context", () => tenant);

// The app the banner sits in: the OGS app unless a test says otherwise.
const shell = vi.hoisted(() => ({ mode: "teacher" }));
vi.mock("~/lib/shell-auth-context", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/shell-auth-context")>()),
  useShellAuthSafe: () => ({ mode: shell.mode }),
}));

const fetchMock = vi.fn();
const assign = vi.fn();

function json(status: number, body: unknown = {}) {
  return Promise.resolve(
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

function renderBanner() {
  return render(
    <ToastProvider>
      <DemoBanner />
    </ToastProvider>,
  );
}

beforeEach(() => {
  vi.stubEnv("NEXT_PUBLIC_APP_ENV", "demo");
  localStorage.clear();
  signIn.mockReset().mockResolvedValue({ error: undefined });
  fetchMock.mockReset();
  assign.mockReset();
  analytics.trackDemoEvent.mockReset();
  analytics.registerDemoVisit.mockReset();
  vi.stubGlobal("fetch", fetchMock);
  vi.stubGlobal("location", { ...globalThis.location, assign });
  shell.mode = "teacher";
  tenant.useTenantSafe.mockReturnValue({
    tenantSlug: "ogs-beispiel-a1b2c3",
    tenant: { name: "OGS Beispiel" },
    routingMode: "subdomain",
  });
  saveDemoVisit({ accessId: "4711", role: "caregiver", src: "messe" });
});

afterEach(() => {
  vi.unstubAllEnvs();
});

describe("DemoBanner", () => {
  it("exists only in the demo build", () => {
    vi.stubEnv("NEXT_PUBLIC_APP_ENV", "production");

    renderBanner();

    expect(screen.queryByRole("region", { name: "Demo" })).toBeNull();
    expect(screen.queryByText("Kostenlos starten")).toBeNull();
  });

  // The server has no stored visit. A first render that read it would not
  // match the server's markup, and React would throw the page away.
  it("renders the same first markup as the server, whatever is stored", () => {
    const markup = renderToString(
      <ToastProvider>
        <DemoBanner />
      </ToastProvider>,
    );

    expect(markup).toContain("Kostenlos starten");
    expect(markup).not.toContain("Betreuungskraft");
    expect(markup).not.toContain("Rolle");
  });

  it("names the demo, the school and the current role", () => {
    renderBanner();

    expect(screen.getByText("Demo")).toBeInTheDocument();
    expect(screen.getByText("OGS Beispiel")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Rolle wechseln/ }),
    ).toHaveTextContent("Betreuungskraft");
  });

  it("reports the visit as the demo access, and an entry once", () => {
    saveDemoVisit({
      accessId: "4711",
      role: "lead",
      src: "messe",
      pending: "demo_entered",
    });

    const { unmount } = renderBanner();

    const visit = { accessId: "4711", role: "lead", src: "messe" };
    expect(analytics.registerDemoVisit).toHaveBeenCalledWith(visit);
    expect(analytics.trackDemoEvent).toHaveBeenCalledWith(
      "demo_entered",
      visit,
    );
    unmount();
    analytics.trackDemoEvent.mockReset();
    renderBanner();
    expect(analytics.trackDemoEvent).not.toHaveBeenCalled();
  });

  it("opens the website's get-to-know page in a new tab and reports the click", () => {
    renderBanner();

    const start = screen.getByRole("link", { name: "Kostenlos starten" });
    expect(start).toHaveAttribute(
      "href",
      "https://moto-ogs.de/start/?src=demo",
    );
    expect(start).toHaveAttribute("target", "_blank");
    fireEvent.click(start);

    expect(analytics.trackDemoEvent).toHaveBeenCalledWith(
      "demo_start_clicked",
      { accessId: "4711", role: "caregiver", src: "messe" },
    );
  });

  it("switches the role with the keyboard and opens the start page anew", async () => {
    const user = userEvent.setup();
    fetchMock.mockReturnValueOnce(
      json(200, {
        access_token: "access",
        refresh_token: "refresh",
        demo: { access_id: "4711", role: "lead", src: "messe" },
      }),
    );
    renderBanner();

    const trigger = screen.getByRole("button", { name: /Rolle wechseln/ });
    trigger.focus();
    await user.keyboard("{Enter}");
    const items = await screen.findAllByRole("menuitemradio");
    expect(items.map((item) => item.textContent)).toEqual([
      "Betreuungskraft",
      "OGS-Leitung",
      "Elternteil",
      "Alle Funktionen",
    ]);
    expect(items[0]).toHaveAttribute("aria-checked", "true");
    expect(items[0]).toHaveFocus();
    await user.keyboard("{ArrowDown}");
    expect(items[1]).toHaveFocus();
    await user.keyboard("{Enter}");

    await waitFor(() => expect(assign).toHaveBeenCalledWith("/"));
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/demo/access/sessions");
    expect(init.body).toBe(JSON.stringify({ role: "lead" }));
    expect(signIn).toHaveBeenCalledWith("credentials", {
      redirect: false,
      internalRefresh: true,
      token: "access",
      refreshToken: "refresh",
    });
    expect(JSON.parse(localStorage.getItem("moto-demo-visit") ?? "")).toEqual({
      accessId: "4711",
      role: "lead",
      src: "messe",
      pending: "demo_role_switched",
    });
  });

  it("keeps the current role when it is chosen again", async () => {
    const user = userEvent.setup();
    renderBanner();

    await user.click(screen.getByRole("button", { name: /Rolle wechseln/ }));
    await user.click(
      screen.getByRole("menuitemradio", { name: "Betreuungskraft" }),
    );

    expect(fetchMock).not.toHaveBeenCalled();
    expect(assign).not.toHaveBeenCalled();
  });

  // Without a stored visit the banner does not know the role: it claims
  // none, so every role can be chosen.
  it("asks for a role when it does not know the current one", async () => {
    localStorage.clear();
    const user = userEvent.setup();
    fetchMock.mockReturnValueOnce(
      json(200, {
        access_token: "access",
        refresh_token: "refresh",
        demo: { access_id: "4711", role: "all" },
      }),
    );
    renderBanner();

    const trigger = screen.getByRole("button", { name: "Rolle wählen" });
    expect(trigger).toHaveTextContent("Rolle wählen");
    expect(analytics.registerDemoVisit).not.toHaveBeenCalled();
    await user.click(trigger);
    for (const item of screen.getAllByRole("menuitemradio")) {
      expect(item).toHaveAttribute("aria-checked", "false");
    }
    await user.click(
      screen.getByRole("menuitemradio", { name: "Alle Funktionen" }),
    );

    await waitFor(() => expect(assign).toHaveBeenCalledWith("/"));
  });

  // The standing demo school is shared: its role cannot change, so the
  // banner names it without a menu that would do nothing.
  it("shows a fixed role without a menu", () => {
    saveDemoVisit({ accessId: "4711", role: "all", fixedRole: true });

    renderBanner();

    expect(screen.getByText("Alle Funktionen")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Rolle/ })).toBeNull();
    expect(
      screen.getByRole("link", { name: "Kostenlos starten" }),
    ).toBeInTheDocument();
  });

  it("sends the visitor back to the mailed link when the demo cannot tell who it is", async () => {
    const user = userEvent.setup();
    fetchMock.mockReturnValueOnce(json(404));
    renderBanner();

    await user.click(screen.getByRole("button", { name: /Rolle wechseln/ }));
    await user.click(
      screen.getByRole("menuitemradio", { name: "OGS-Leitung" }),
    );

    expect(
      await screen.findByText(
        "Bitte öffnen Sie die Demo noch einmal über den Link aus Ihrer E-Mail.",
      ),
    ).toBeInTheDocument();
    expect(signIn).not.toHaveBeenCalled();
  });

  it("says so when the switch fails and stays in the current role", async () => {
    const user = userEvent.setup();
    fetchMock.mockReturnValueOnce(json(500));
    renderBanner();

    await user.click(screen.getByRole("button", { name: /Rolle wechseln/ }));
    await user.click(
      screen.getByRole("menuitemradio", { name: "Alle Funktionen" }),
    );

    expect(
      await screen.findByText(
        "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
      ),
    ).toBeInTheDocument();
    expect(signIn).not.toHaveBeenCalled();
    expect(assign).not.toHaveBeenCalled();
    expect(
      screen.getByRole("button", { name: /Rolle wechseln/ }),
    ).toHaveTextContent("Betreuungskraft");
  });

  // The role parent lives in the parents app on its own host (#3468): the
  // banner hands over to it instead of switching this session.
  it("opens the parents app for the role parent", async () => {
    const user = userEvent.setup();
    renderBanner();

    await user.click(screen.getByRole("button", { name: /Rolle wechseln/ }));
    await user.click(screen.getByRole("menuitemradio", { name: "Elternteil" }));

    expect(assign).toHaveBeenCalledWith("/api/demo/access/handoff?role=parent");
    expect(fetchMock).not.toHaveBeenCalled();
    expect(signIn).not.toHaveBeenCalled();
  });

  it("shows the banner in the parents app and leads back to the OGS app", async () => {
    shell.mode = "parent";
    // The parents app knows no school; the entry noted its name.
    tenant.useTenantSafe.mockReturnValue(null);
    saveDemoVisit({
      accessId: "4711",
      role: "parent",
      src: "messe",
      schoolName: "OGS Sonnenschein",
    });
    const user = userEvent.setup();
    renderBanner();

    expect(await screen.findByText("OGS Sonnenschein")).toBeInTheDocument();
    const trigger = screen.getByRole("button", { name: /Rolle wechseln/ });
    expect(trigger).toHaveTextContent("Elternteil");
    await user.click(trigger);
    await user.click(
      screen.getByRole("menuitemradio", { name: "OGS-Leitung" }),
    );

    expect(assign).toHaveBeenCalledWith("/api/demo/access/handoff?role=lead");
    expect(fetchMock).not.toHaveBeenCalled();
    expect(analytics.registerDemoVisit).toHaveBeenCalledWith(
      expect.objectContaining({ role: "parent" }),
    );
  });
});
