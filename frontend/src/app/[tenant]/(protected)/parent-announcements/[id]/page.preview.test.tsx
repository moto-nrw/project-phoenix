import { render, renderHook, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { SWRConfig } from "~/lib/swr/testing";
import { useSWRAuth } from "~/lib/swr";
import { parentAnnouncementKeys } from "~/lib/swr/portal-keys";
import { ToastProvider } from "~/contexts/ToastContext";
import AnnouncementDetailPage from "./page";

vi.unmock("swr");
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
      expect.anything(),
    );
    if (response_type !== "none") {
      expect(fetch).toHaveBeenCalledWith(
        "/api/parent-announcements/4/poll-results",
        expect.anything(),
      );
      expect(fetch).toHaveBeenCalledWith(
        "/api/parent-announcements/4/poll-children",
        expect.anything(),
      );
    }
  },
);
