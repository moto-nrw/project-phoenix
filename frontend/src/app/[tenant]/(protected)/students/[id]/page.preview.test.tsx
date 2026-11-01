import { render, renderHook, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, onTestFinished, vi } from "vitest";
import { SWRConfig } from "~/lib/swr/testing";
import { useSWRAuth } from "~/lib/swr";
import { studentKeys } from "~/lib/swr/portal-keys";
import { ToastProvider } from "~/contexts/ToastContext";
import StudentDetailPage from "./page";

vi.unmock("swr");
const state = vi.hoisted(() => ({
  permissions: ["users:read"],
  detail: vi.fn(),
  pickup: vi.fn(),
  arrival: vi.fn(),
  status: vi.fn(),
  partial: vi.fn(),
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "7", permissions: state.permissions },
      expires: "2099-01-01",
    },
    status: "authenticated",
  }),
}));
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "7" }),
  usePathname: () => "/students/7",
  useSearchParams: () => new URLSearchParams(),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));
vi.mock("~/lib/breadcrumb-context", () => ({ useSetBreadcrumb: vi.fn() }));
vi.mock("~/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api")>()),
  studentService: { getStudent: state.detail },
}));
vi.mock("~/lib/usercontext-api", () => ({
  userContextService: {
    getMyEducationalGroups: async () => [],
    getMySupervisedGroups: async () => [],
  },
}));
vi.mock("~/lib/pickup-schedule-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/pickup-schedule-api")>()),
  fetchStudentPickupData: state.pickup,
}));
vi.mock("~/lib/student-arrival-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/student-arrival-api")>()),
  fetchArrivalData: state.arrival,
}));
vi.mock("~/lib/student-status-days-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/student-status-days-api")>()),
  fetchStudentStatusDays: state.status,
}));
vi.mock("~/lib/student-partial-absences-api", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("~/lib/student-partial-absences-api")
  >()),
  fetchStudentPartialAbsences: state.partial,
}));

beforeEach(() => {
  state.permissions = ["users:read"];
  for (const fn of [
    state.detail,
    state.pickup,
    state.arrival,
    state.status,
    state.partial,
  ])
    fn.mockReset().mockImplementation(() => new Promise(() => {}));
});

async function renderFromList() {
  const cache = new Map();
  const wrapper = ({ children }: { children: React.ReactNode }) => (
    <SWRConfig value={{ provider: () => cache }}>
      <ToastProvider>{children}</ToastProvider>
    </SWRConfig>
  );
  const list = renderHook(
    () =>
      useSWRAuth("search-students-gall-preview", async () => ({
        students: [
          {
            id: "7",
            name: "Mila Muster",
            has_full_access: true,
            health_info: "Nicht aus der Liste zeigen",
          },
        ],
      })),
    { wrapper },
  );
  await waitFor(() => expect(list.result.current.data).toBeDefined());
  list.unmount();
  return render(<StudentDetailPage />, { wrapper });
}

describe("Kindakte mit echtem SWR und echtem useStudentData", () => {
  it("zeigt nur die Identität und startet alle vier Zusatzabrufe vor der Detailantwort", async () => {
    const project = vi.spyOn(studentKeys, "preview");
    onTestFinished(() => project.mockRestore());
    const view = await renderFromList();
    // Anker: der echte Seitenkopf ist sichtbar, die Akte wartet noch auf die Antwort.
    expect(
      screen.getByRole("heading", { name: "Mila Muster" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("status", { name: "Kind wird geladen" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Nicht aus der Liste zeigen"),
    ).not.toBeInTheDocument();
    expect(
      view.container.querySelector(".animate-pulse.h-4.w-56"),
    ).toBeInTheDocument();
    expect(state.detail).toHaveBeenCalledTimes(1);
    for (const fn of [state.pickup, state.arrival, state.status, state.partial])
      expect(fn).toHaveBeenCalledTimes(1);
    view.rerender(<StudentDetailPage />);
    expect(project).toHaveBeenCalledTimes(1);
  });

  it("startet ohne users:read keine Zusatzabrufe und zeigt auch dann die schmale Vorschau", async () => {
    state.permissions = ["students:read"];
    await renderFromList();
    // Anker: dieselbe Kindakte ist geladen, nicht etwa ein leeres Rendering.
    expect(
      screen.getByRole("heading", { name: "Mila Muster" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("status", { name: "Kind wird geladen" }),
    ).toBeInTheDocument();
    for (const fn of [state.pickup, state.arrival, state.status, state.partial])
      expect(fn).not.toHaveBeenCalled();
  });
});
