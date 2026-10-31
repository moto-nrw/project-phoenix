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
vi.unmock("swr");
const mocks = vi.hoisted(() => ({
  session: vi.fn(),
  list: vi.fn(),
  update: vi.fn(),
  remove: vi.fn(),
}));
vi.mock("next-auth/react", () => ({ useSession: mocks.session }));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "school-a",
  useTenant: () => ({ tenantSlug: "school-a" }),
}));
vi.mock("~/lib/hooks/use-require-permission", () => ({
  useRequirePermission: () => ({ isLoading: false }),
}));
vi.mock("~/components/tenant/display-mode-guard", () => ({
  DisplayModeGuard: ({ children }: { children: ReactNode }) => children,
}));
// The menu itself is covered by OverflowMenu's tests. Keep its disabled contract.
vi.mock("~/components/ui/page-header/OverflowMenu", () => ({
  OverflowMenu: ({
    items,
    disabled,
  }: {
    items: { label: string; onClick: () => void }[];
    disabled?: boolean;
  }) => (
    <div>
      {items.map((item) => (
        <button key={item.label} disabled={disabled} onClick={item.onClick}>
          {item.label}
        </button>
      ))}
    </div>
  ),
}));
vi.mock("~/lib/display-api", () => ({
  listDisplays: mocks.list,
  updateDisplay: mocks.update,
  deleteDisplay: mocks.remove,
  createDisplay: vi.fn(),
  regenerateDisplayToken: vi.fn(),
}));

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { InfoDisplay } from "~/lib/display-helpers";
import { SWRConfig } from "~/lib/swr/testing";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { resetSWRScope, resumeSWRSession } from "~/lib/swr/scope";
import InfoDisplaysPage from "./page";

const display: InfoDisplay = {
  id: "1",
  name: "Eingang",
  isActive: true,
  createdAt: "2026-09-01T08:00:00Z",
};
const neighbour: InfoDisplay = { ...display, id: "2", name: "Mensa" };
function signedIn(id = "A") {
  return {
    data: {
      user: { id, tenantId: 1, roles: ["admin"], permissions: [] },
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
function deferred() {
  let resolve!: (value: InfoDisplay | undefined) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<InfoDisplay | undefined>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}
function rows() {
  return screen
    .getAllByRole("row")
    .map((row) => row.textContent)
    .join("|");
}
// A string name matches the whole accessible name.
function clickFirst(name: string) {
  fireEvent.click(screen.getAllByRole("button", { name })[0]!);
}
const scenarios = [
  {
    name: "Schalten",
    request: mocks.update,
    reply: { ...display, isActive: false },
  },
  {
    name: "Umbenennen",
    request: mocks.update,
    reply: { ...display, name: "Flur" },
  },
  { name: "Löschen", request: mocks.remove, reply: undefined },
] as const;
beforeEach(() => {
  vi.clearAllMocks();
  resumeSWRSession();
  resetSWRScope({ revalidate: false });
  mocks.session.mockReturnValue(signedIn());
  mocks.list.mockResolvedValue([display, neighbour]);
});
afterEach(() => {
  resumeSWRSession();
  resetSWRScope({ revalidate: false });
});
for (const scenario of scenarios) {
  describe(scenario.name, () => {
    async function setup() {
      const pending = deferred();
      scenario.request.mockReturnValue(pending.promise);
      const view = render(<InfoDisplaysPage />, { wrapper: Wrapper });
      await screen.findByText("Eingang");
      let submit: () => void;
      if (scenario.name === "Umbenennen") {
        clickFirst("Umbenennen");
        const dialog = screen.getByRole("dialog");
        fireEvent.change(within(dialog).getByLabelText("Name des Displays"), {
          target: { value: "Flur" },
        });
        submit = () =>
          fireEvent.click(
            within(dialog).getByRole("button", { name: "Speichern" }),
          );
      } else if (scenario.name === "Löschen") {
        clickFirst("Löschen");
        const dialog = screen.getByRole("dialog");
        fireEvent.click(
          within(dialog).getByRole("button", {
            name: "Ja, löschen",
          }),
        );
        const confirm = within(dialog).getByRole("button", {
          name: "Endgültig löschen",
        });
        submit = () => fireEvent.click(confirm);
      } else {
        submit = () =>
          fireEvent.click(
            screen.getAllByRole("button", {
              name: "Deaktivieren",
            })[0]!,
          );
      }
      return { ...view, pending, submit };
    }
    function expectOptimistic() {
      expect(rows()).toContain("Mensa"); // Anker: Nachbarzeile der geladenen Liste.
      if (scenario.name === "Löschen") expect(rows()).not.toContain("Eingang");
      else if (scenario.name === "Umbenennen") expect(rows()).toContain("Flur");
      else expect(rows()).toMatch(/Eingang.*Inaktiv/);
    }
    it("zeigt sofort den Zwischenstand und übernimmt die Antwort ohne Neuabruf", async () => {
      const { pending, submit } = await setup();
      const start = performance.now();
      submit();
      expect(performance.now() - start).toBeLessThan(100);
      expectOptimistic();
      await act(async () => {
        pending.resolve(scenario.reply);
      });
      expectOptimistic();
      expect(mocks.list).toHaveBeenCalledTimes(1);
    });
    it.each([
      "offline",
      "display.disabled",
      "identity.session_account_mismatch",
    ])("nimmt %s zurück, ohne die Liste zu verwerfen", async (code) => {
      const { pending, submit } = await setup();
      const before = rows();
      submit();
      expectOptimistic();
      await act(async () => {
        pending.reject(
          code === "offline"
            ? new TypeError("offline")
            : new ApiError("diag", 403, {
                code: code as
                  "display.disabled" | "identity.session_account_mismatch",
              }),
        );
      });
      expect(rows()).toBe(before);
      expect(await screen.findByRole("alert")).toBeInTheDocument();
    });
    // No code of these routes refuses the list (everyday-action-refusals.ts):
    // a 403 may leave it readable (display:read), a 404 names one display.
    // The action rolls back and the list loads again (#5423).
    it.each([
      [403, "general.permission"],
      [404, "general.input"],
    ] as const)(
      "nimmt %s %s zurück und lädt die Liste neu, statt sie zu verwerfen",
      async (status, code) => {
        const { pending, submit } = await setup();
        submit();
        expectOptimistic();
        // The reload shows what the server now has: the display is gone.
        mocks.list.mockResolvedValue([neighbour]);
        await act(async () => {
          pending.reject(new ApiError("diag", status, { code }));
        });
        expect(await screen.findByRole("alert")).toBeInTheDocument();
        await waitFor(() => expect(mocks.list).toHaveBeenCalledTimes(2));
        // Anker: die Liste steht nach dem Neuladen weiter da.
        expect(await screen.findByText("Mensa")).toBeInTheDocument();
        await waitFor(() => expect(rows()).not.toContain("Eingang"));
      },
    );
    it.each(["Erfolg", "Fehler"])(
      "zeigt nach Scope-Wechsel keine alte Antwort: %s",
      async (end) => {
        const { pending, submit, rerender } = await setup();
        submit();
        expectOptimistic();
        act(() => {
          mocks.session.mockReturnValue(signedIn("B"));
          rerender(<InfoDisplaysPage />);
        });
        await screen.findByText("Eingang");
        const next = rows();
        await act(async () => {
          if (end === "Erfolg") pending.resolve(scenario.reply);
          else pending.reject(new TypeError("offline"));
        });
        expect(rows()).toBe(next);
      },
    );
    it("schreibt bei doppeltem Absenden nur einmal", async () => {
      const { pending, submit } = await setup();
      submit();
      submit();
      expectOptimistic();
      expect(scenario.request).toHaveBeenCalledTimes(1);
      await act(async () => {
        pending.resolve(scenario.reply);
      });
    });
    it("hinterlässt nach Unmount und spätem Fehler keinen Zwischenstand", async () => {
      const { pending, submit, unmount } = await setup();
      submit();
      expectOptimistic();
      unmount();
      await act(async () => {
        pending.reject(new TypeError("offline"));
      });
      render(<InfoDisplaysPage />, { wrapper: Wrapper });
      await screen.findByText("Eingang");
      expect(rows()).toContain("Mensa");
    });
  });
}
