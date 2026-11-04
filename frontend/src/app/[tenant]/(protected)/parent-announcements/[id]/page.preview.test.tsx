import { render, renderHook, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, onTestFinished, vi } from "vitest";
import { SWRConfig } from "~/lib/swr/testing";
import { useSWRAuth } from "~/lib/swr";
import { parentAnnouncementKeys } from "~/lib/swr/portal-keys";
import { ToastProvider } from "~/contexts/ToastContext";
import AnnouncementDetailPage from "./page";

vi.unmock("swr");
vi.mock("~/lib/activity-api", () => ({
  fetchActivities: () => new Promise(() => {}),
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "7", tenantId: 1 } },
    status: "authenticated",
  }),
}));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "4" }),
  usePathname: () => "/parent-announcements/4",
  useSearchParams: () => new URLSearchParams(),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
}));
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn() }),
}));
vi.mock("~/lib/breadcrumb-context", () => ({ useSetBreadcrumb: vi.fn() }));
beforeEach(() =>
  vi.stubGlobal(
    "fetch",
    vi.fn(() => new Promise<Response>(() => {})),
  ),
);

it.each(["none", "single_choice"])(
  "shows list title for %s without revealing detail and starts poll evaluation in parallel",
  async (response_type) => {
    const cache = new Map();
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <SWRConfig value={{ provider: () => cache }}>
        <ToastProvider>{children}</ToastProvider>
      </SWRConfig>
    );
    const list = renderHook(
      () =>
        useSWRAuth(parentAnnouncementKeys.list(), async () => [
          {
            id: "4",
            title: "Sommerfest",
            status: "published",
            response_type,
            body: "Nur im Detail",
            options: [{ label: "Geheime Antwort" }],
          },
        ]),
      { wrapper },
    );
    await waitFor(() => expect(list.result.current.data).toBeDefined());
    list.unmount();
    const view = render(<AnnouncementDetailPage />, { wrapper });
    expect(
      screen.getByRole("heading", { name: "Sommerfest" }),
    ).toBeInTheDocument();
    expect(view.container.querySelector(".animate-pulse")).toBeInTheDocument();
    // Anker: der Vorschaukopf ist sichtbar, kein vollständiger Datensatz.
    expect(screen.queryByText("Nur im Detail")).not.toBeInTheDocument();
    expect(screen.queryByText("Geheime Antwort")).not.toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith(
      "/api/parent-announcements/4",
      undefined,
    );
    if (response_type !== "none") {
      expect(fetch).toHaveBeenCalledWith(
        "/api/parent-announcements/4/poll-results",
        undefined,
      );
      expect(fetch).toHaveBeenCalledWith(
        "/api/parent-announcements/4/poll-children",
        undefined,
      );
    }
  },
);

it("loads evaluation before detail and reuses it only after detail confirms the poll", async () => {
  const cache = new Map();
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <SWRConfig value={{ provider: () => cache }}>
      <ToastProvider>{children}</ToastProvider>
    </SWRConfig>
  );
  const list = renderHook(
    () =>
      useSWRAuth(parentAnnouncementKeys.list(), async () => [
        {
          id: "4",
          title: "Sommerfest",
          status: "published",
          response_type: "single_choice",
        },
      ]),
    { wrapper },
  );
  await waitFor(() => expect(list.result.current.data).toBeDefined());
  list.unmount();
  let finish!: (response: Response) => void;
  const pending = new Promise<Response>((resolve) => {
    finish = resolve;
  });
  vi.mocked(fetch).mockImplementation(async (input) => {
    const url = String(input);
    if (url.endsWith("/poll-results"))
      return Response.json({
        data: {
          answered_count: 1,
          child_count: 1,
          target_child_count: 1,
          options: [
            { option_id: "1", label: "Auswertung bestätigt", count: 1 },
          ],
        },
      });
    if (url.endsWith("/poll-children")) return Response.json({ data: [] });
    return pending;
  });
  render(<AnnouncementDetailPage />, { wrapper });
  expect(fetch).toHaveBeenCalledWith(
    "/api/parent-announcements/4/poll-results",
    undefined,
  );
  expect(fetch).toHaveBeenCalledWith(
    "/api/parent-announcements/4/poll-children",
    undefined,
  );
  // Anker: die bekannte Identität steht im Kopf, die bereits geladenen Ergebnisse noch nicht.
  expect(
    screen.getByRole("heading", { name: "Sommerfest" }),
  ).toBeInTheDocument();
  expect(screen.queryByText("Auswertung bestätigt")).not.toBeInTheDocument();
  finish(
    Response.json({
      data: {
        id: "4",
        title: "Sommerfest",
        body: "Bestätigter Detailtext",
        status: "published",
        priority: "info",
        response_type: "single_choice",
        delivery_mode: "standard",
        targets: [],
        options: [],
      },
    }),
  );
  expect(await screen.findByText("Bestätigter Detailtext")).toBeInTheDocument();
  expect(await screen.findByText("Auswertung bestätigt")).toBeInTheDocument();
  expect(
    vi
      .mocked(fetch)
      .mock.calls.filter(([url]) => String(url).endsWith("/poll-results")),
  ).toHaveLength(1);
});

it.each([true, false])(
  "logs poll failures through the reader (early list gate: %s)",
  async (seeded) => {
    const errors = vi.spyOn(console, "error").mockImplementation(() => {});
    onTestFinished(() => errors.mockRestore());
    const cache = new Map();
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <SWRConfig value={{ provider: () => cache, shouldRetryOnError: false }}>
        <ToastProvider>{children}</ToastProvider>
      </SWRConfig>
    );
    if (seeded) {
      const list = renderHook(
        () =>
          useSWRAuth(parentAnnouncementKeys.list(), async () => [
            {
              id: "4",
              title: "Sommerfest",
              status: "published",
              response_type: "single_choice",
            },
          ]),
        { wrapper },
      );
      await waitFor(() => expect(list.result.current.data).toBeDefined());
      list.unmount();
    }
    let finish!: (response: Response) => void;
    const pending = new Promise<Response>((resolve) => {
      finish = resolve;
    });
    vi.mocked(fetch).mockImplementation((input) => {
      const url = String(input);
      if (url.endsWith("/poll-results"))
        return Promise.reject(new Error("Auswertung offline"));
      if (url.endsWith("/poll-children"))
        return Promise.resolve(Response.json({ data: [] }));
      return pending;
    });
    render(<AnnouncementDetailPage />, { wrapper });
    if (seeded) {
      expect(
        screen.getByRole("heading", { name: "Sommerfest" }),
      ).toBeInTheDocument();
      await waitFor(() =>
        expect(errors).toHaveBeenCalledWith(
          "announcement_poll_results_failed",
          { error: "Auswertung offline" },
        ),
      );
    }
    finish(
      Response.json({
        data: {
          id: "4",
          title: "Sommerfest",
          body: "Bestätigter Detailtext",
          status: "published",
          priority: "info",
          response_type: "single_choice",
          delivery_mode: "standard",
          targets: [],
          options: [],
        },
      }),
    );
    expect(
      await screen.findByText("Bestätigter Detailtext"),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(errors).toHaveBeenCalledWith("announcement_poll_results_failed", {
        error: "Auswertung offline",
      }),
    );
    expect(
      errors.mock.calls.filter(
        ([event]) => event === "announcement_poll_results_failed",
      ),
    ).toHaveLength(1);
  },
);
