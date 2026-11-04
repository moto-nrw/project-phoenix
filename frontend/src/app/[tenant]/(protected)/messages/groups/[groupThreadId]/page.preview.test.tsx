import { render, renderHook, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { SWRConfig } from "~/lib/swr/testing";
import { useSWRAuth } from "~/lib/swr";
import { messageKeys } from "~/lib/swr/portal-keys";
import { ToastProvider } from "~/contexts/ToastContext";
import GroupMessagePage from "./page";

vi.unmock("swr");
const owner = vi.hoisted(() => ({ account: "7", school: "schule" }));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: owner.account, tenantId: 1 } },
    status: "authenticated",
  }),
}));
vi.mock("next/navigation", () => ({
  useParams: () => ({ groupThreadId: "4" }),
  usePathname: () => "/messages/groups/4",
  useSearchParams: () => new URLSearchParams(),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenant: () => ({ tenant: { messagingEnabled: true } }),
  useTenantSlugSafe: () => owner.school,
  useTenantRoutingModeSafe: () => "path",
}));
vi.mock("~/lib/hooks/use-messages-activity", () => ({
  useMessagesActivity: vi.fn(),
}));
beforeEach(() => {
  Object.assign(owner, { account: "7", school: "schule" });
  vi.stubGlobal(
    "fetch",
    vi.fn(() => new Promise<Response>(() => {})),
  );
});

it.each(["same", "school", "account"])(
  "group loading header with %s scope",
  async (change) => {
    const cache = new Map();
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <SWRConfig value={{ provider: () => cache }}>
        <ToastProvider>{children}</ToastProvider>
      </SWRConfig>
    );
    const list = renderHook(
      () =>
        useSWRAuth(messageKeys.groupMessages(), async () => [
          {
            id: "4",
            title: "Sommerfest",
            last_message_body: "Privater Inhalt",
            family_count: 99,
          },
        ]),
      { wrapper },
    );
    await waitFor(() => expect(list.result.current.data).toBeDefined());
    list.unmount();
    if (change === "school") owner.school = "andere-schule";
    if (change === "account") owner.account = "8";
    const view = render(<GroupMessagePage />, { wrapper });
    expect(
      screen.getByRole("heading", {
        name: change === "same" ? "Sommerfest" : "Gruppennachricht",
      }),
    ).toBeInTheDocument();
    // Anker: der echte Ladekopf ist sichtbar, Detailinhalte bleiben verborgen.
    expect(view.container.querySelector(".animate-pulse")).toBeInTheDocument();
    expect(screen.queryByText("Privater Inhalt")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Schließen" }),
    ).not.toBeInTheDocument();
    expect(fetch).toHaveBeenCalled();
  },
);

it.each(["school", "account"])(
  "withdraws a mounted preview on %s change",
  async (change) => {
    const cache = new Map();
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <SWRConfig value={{ provider: () => cache }}>
        <ToastProvider>{children}</ToastProvider>
      </SWRConfig>
    );
    const list = renderHook(
      () =>
        useSWRAuth(messageKeys.groupMessages(), async () => [
          { id: "4", title: "Sommerfest" },
        ]),
      { wrapper },
    );
    await waitFor(() => expect(list.result.current.data).toBeDefined());
    list.unmount();
    const view = render(<GroupMessagePage />, { wrapper });
    expect(
      screen.getByRole("heading", { name: "Sommerfest" }),
    ).toBeInTheDocument();
    if (change === "school") owner.school = "andere-schule";
    else owner.account = "8";
    view.rerender(<GroupMessagePage />);
    // Anker: die neue Sitzung zeigt ihren eigenen Ladekopf statt der alten Identität.
    expect(
      screen.getByRole("heading", { name: "Gruppennachricht" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Sommerfest" }),
    ).not.toBeInTheDocument();
  },
);
