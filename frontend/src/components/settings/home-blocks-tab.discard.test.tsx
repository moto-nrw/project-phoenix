import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Session } from "next-auth";

// A policy save whose scope changes while it runs is discarded by the
// optimistic helper. The tab must then report neither success nor failure
// (#5012, Schluss-Review auf c99735a): real SWR, real useHomeLayout.
vi.unmock("swr");
const mocks = vi.hoisted(() => ({
  session: vi.fn(),
  layout: vi.fn(),
  policies: vi.fn(),
  // Stable references: the tab derives effects from them.
  schema: { data: { tabs: [] }, isLoading: false, mutate: () => undefined },
  modules: ["nfc_anwesenheit", "mittagessen", "zeiterfassung"],
}));
vi.mock("next-auth/react", () => ({ useSession: mocks.session }));
vi.mock("~/lib/home-layout-api", () => ({
  fetchHomeLayout: mocks.layout,
  saveHomeLayout: vi.fn(),
  resetHomeLayout: vi.fn(),
  saveHomeBlockPolicies: mocks.policies,
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "school-a",
  usePresenceMode: () => "detailed",
  useOpenCareGroupMode: () => false,
  useNFCEnabled: () => true,
  useBookedModules: () => mocks.modules,
}));
vi.mock("~/lib/hooks/use-settings-schema", () => ({
  useSettingsSchema: () => mocks.schema,
}));
vi.mock("~/lib/settings-api", () => ({ saveSettingValue: vi.fn() }));
vi.mock("~/lib/settings-broadcast", () => ({ notifySettingsChanged: vi.fn() }));

import { ToastProvider } from "~/contexts/ToastContext";
import { SWRConfig } from "~/lib/swr/testing";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { resetSWRScope, resumeSWRSession } from "~/lib/swr/scope";
import { HomeBlocksTab } from "./home-blocks-tab";

const layout = {
  blocks: [],
  overrides: {},
  policies: {},
  canManagePolicies: true,
};
function signedIn(id: string) {
  return {
    data: {
      user: { id, tenantId: 1, permissions: ["config:update"] },
      expires: "2099-01-01",
    } satisfies Session,
    status: "authenticated",
  };
}
function Wrapper({ children }: { children: ReactNode }) {
  return (
    <SWRConfig
      value={{
        provider: () => new Map(),
        dedupingInterval: 0,
        shouldRetryOnError: false,
      }}
    >
      <SWRScopeBoundary expectedTenantId={1} />
      <ToastProvider>{children}</ToastProvider>
    </SWRConfig>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  resumeSWRSession();
  resetSWRScope({ revalidate: false });
  mocks.session.mockReturnValue(signedIn("A"));
  mocks.layout.mockResolvedValue(layout);
});
afterEach(() => {
  resumeSWRSession();
  resetSWRScope({ revalidate: false });
});

describe("HomeBlocksTab, Vorgabe während eines Kontowechsels", () => {
  it.each(["Fehler", "Erfolg"])(
    "meldet nach dem Verwerfen keinen Erfolg: Antwort %s",
    async (end) => {
      let settle!: (failed: boolean) => void;
      mocks.policies.mockReturnValue(
        new Promise<void>((resolve, reject) => {
          settle = (failed) =>
            failed ? reject(new TypeError("offline")) : resolve();
        }),
      );
      const view = render(<HomeBlocksTab />, { wrapper: Wrapper });
      const group = await screen.findByRole("group", {
        name: "Vorgabe für Geburtstage",
      });
      fireEvent.click(within(group).getByRole("button", { name: "Aus" }));
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
      await waitFor(() => expect(mocks.policies).toHaveBeenCalledTimes(1));

      act(() => {
        mocks.session.mockReturnValue(signedIn("B"));
        view.rerender(<HomeBlocksTab />);
      });
      // Anker: der Reiter steht für das neue Konto wieder geladen da.
      expect(
        await screen.findByRole("group", { name: "Vorgabe für Geburtstage" }),
      ).toBeInTheDocument();

      await act(async () => {
        settle(end === "Fehler");
      });

      // Anker: „Speichern“ ist nach dem Ende der Anfrage wieder bedienbar.
      await waitFor(() =>
        expect(
          screen.getByRole("button", { name: "Speichern" }),
        ).not.toHaveAttribute("aria-busy", "true"),
      );
      expect(
        screen.queryByText("Die Startseite für alle ist gespeichert."),
      ).not.toBeInTheDocument();
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    },
  );
});
