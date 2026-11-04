import {
  act,
  render,
  renderHook,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, onTestFinished, vi } from "vitest";
import { SWRConfig } from "~/lib/swr/testing";
import { useSWRAuth } from "~/lib/swr";
import { roomKeys } from "~/lib/swr/rooms-keys";
import { ToastProvider } from "~/contexts/ToastContext";
import RoomDetailPage from "./page";

vi.unmock("swr");
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "7", permissions: ["rooms:read"] },
      expires: "2099-01-01",
    },
    status: "authenticated",
  }),
}));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "7" }),
  usePathname: () => "/rooms/7",
  useSearchParams: () => new URLSearchParams(),
  useRouter: () => ({ replace: vi.fn() }),
}));
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));
vi.mock("~/lib/breadcrumb-context", () => ({ useSetBreadcrumb: vi.fn() }));
vi.mock("~/lib/supervision-context", () => ({
  useOptionalSupervision: () => ({ overviewEnabled: false }),
}));
vi.mock("~/lib/hooks/use-student-photos-enabled", () => ({
  useStudentPhotosEnabled: () => ({ enabled: false }),
}));

beforeEach(() =>
  vi.stubGlobal(
    "fetch",
    vi.fn(() => new Promise<Response>(() => {})),
  ),
);

describe("Raumseite mit echtem SWR und echtem useRoomDetail", () => {
  it("zeigt die Vorschau bis Raum und Historie geladen sind und hält den Platz ohne Kontext frei", async () => {
    const cache = new Map();
    const wrapper = ({ children }: { children: React.ReactNode }) => (
      <SWRConfig value={{ provider: () => cache }}>
        <ToastProvider>{children}</ToastProvider>
      </SWRConfig>
    );
    const list = renderHook(
      () =>
        useSWRAuth(roomKeys.key({ includeSystem: true }), async () => [
          { id: "7", name: "Aula" },
        ]),
      { wrapper },
    );
    await waitFor(() => expect(list.result.current.data).toBeDefined());
    list.unmount();
    const project = vi.spyOn(roomKeys, "preview");
    onTestFinished(() => project.mockRestore());
    const view = render(<RoomDetailPage />, { wrapper });
    expect(screen.getByRole("heading", { name: "Aula" })).toBeInTheDocument();
    expect(
      screen.getByRole("status", { name: "Raumdetails werden geladen" }),
    ).toBeInTheDocument();
    expect(
      view.container.querySelector(".animate-pulse.h-4.w-56"),
    ).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith("/api/rooms/7", expect.any(Object));
    expect(fetch).toHaveBeenCalledWith(
      "/api/rooms/7/history",
      expect.any(Object),
    );
    await act(async () => view.rerender(<RoomDetailPage />));
    expect(project).toHaveBeenCalledTimes(1);
  });
});
