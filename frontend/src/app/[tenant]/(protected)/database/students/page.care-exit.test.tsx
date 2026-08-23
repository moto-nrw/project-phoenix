// Die Aktionen im Detailbereich für "Betreuung beenden" (#2487): beenden,
// ein geplantes Ende ändern oder stornieren, nach dem Austritt wieder
// aufnehmen — und dass ohne die Berechtigung "Benutzer löschen" nichts davon
// erscheint.
//
// Eigene Datei: page.test.tsx ist bereits sehr breit, und die Auswahl hier
// hängt an anderen Mocks (Berechtigung, care-exit-api).
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";

import StudentsPage from "./page";
import { useSession } from "next-auth/react";
import { useSWRAuth } from "~/lib/swr";

const { mockCancelCareExit } = vi.hoisted(() => ({
  mockCancelCareExit: vi.fn(),
}));

vi.mock("~/lib/care-exit-api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/care-exit-api")>();
  return { ...actual, cancelCareExit: mockCancelCareExit };
});

vi.mock("next-auth/react", () => ({ useSession: vi.fn() }));

let currentSearch = new URLSearchParams();
vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: vi.fn(() => ({ push: vi.fn(), replace: vi.fn() })),
  usePathname: vi.fn(() => "/tenant/database/students"),
  useSearchParams: () => currentSearch,
}));

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: () => false,
}));

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: toastSuccess, error: toastError }),
}));

vi.mock("~/components/database/database-page-layout", () => ({
  DatabasePageLayout: ({ children }: { children: ReactNode }) => (
    <div>{children}</div>
  ),
}));

vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
  PageHeaderWithSearch: ({
    actionButton,
    overflowMenu,
  }: {
    actionButton?: ReactNode;
    overflowMenu?: Array<{ label: string; href?: string }>;
  }) => (
    <div>
      <div data-testid="header-actions">{actionButton}</div>
      <ul data-testid="overflow-menu">
        {(overflowMenu ?? []).map((entry) => (
          <li key={entry.label}>{entry.label}</li>
        ))}
      </ul>
    </div>
  ),
}));

vi.mock("~/components/database/master-detail-skeleton", () => ({
  MasterDetailSkeleton: () => <div />,
}));

vi.mock("~/components/database/database-grouping-toggle", () => ({
  DatabaseGroupingToggle: () => <div />,
}));

vi.mock("@/components/students/student-create-modal", () => ({
  StudentCreateModal: () => null,
}));

vi.mock("~/components/students/student-deletion-modal", () => ({
  StudentDeletionModal: () => <div data-testid="deletion-modal" />,
}));

vi.mock("~/components/students/care-exit-modal", () => ({
  CareExitModal: ({ studentIds }: { studentIds: readonly string[] }) => (
    <div data-testid="care-exit-modal">{studentIds.join(",")}</div>
  ),
}));

vi.mock("~/components/students/care-resume-modal", () => ({
  CareResumeModal: ({ studentId }: { studentId: string }) => (
    <div data-testid="care-resume-modal">{studentId}</div>
  ),
}));

// The master-detail is replaced by a stub that simply renders whatever the
// page passes as detail actions for the selected child.
vi.mock("@/components/students/students-master-detail", () => ({
  StudentsMasterDetail: ({ detailActions }: { detailActions?: ReactNode }) => (
    <div data-testid="detail-actions">{detailActions}</div>
  ),
}));

vi.mock("@/lib/database/service-factory", () => ({
  createCrudService: vi.fn(() => ({
    getList: vi.fn(),
    getOne: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
  })),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  mutate: vi.fn(),
  useTenantMutate: vi.fn(() => vi.fn()),
}));

const RUNNING = {
  id: "1",
  first_name: "Max",
  second_name: "Mustermann",
  school_class: "1a",
};
const PLANNED = {
  ...RUNNING,
  care_ends_on: isoInDays(14),
  care_ended: false,
  care_exit_recorded: true,
};
// Ein Ende weit voraus, das die Schule selbst eingetragen hat. Es muss sich
// genauso ändern und stornieren lassen wie ein nahes (#2487).
const PLANNED_FAR_AHEAD = {
  ...RUNNING,
  care_ends_on: isoInDays(200),
  care_ended: false,
  care_exit_recorded: true,
};
// Kein eingetragener Austritt, nur das Ende der Anmeldephase. Das gehört der
// Anmeldung und wird hier nicht zum Stornieren angeboten.
const PHASE_END_ONLY = {
  ...RUNNING,
  care_ends_on: isoInDays(200),
  care_ended: false,
};
// Ein wirksam gewordener Austritt, den die Schule selbst eingetragen hat.
const ENDED = {
  ...RUNNING,
  care_ends_on: isoInDays(-3),
  care_ended: true,
  care_exit_recorded: true,
};
// Beendet, aber ohne eingetragenen Austritt: die Anmeldephase lief aus. Der
// Server nimmt so ein Kind nicht wieder auf, also darf der Knopf auch nicht
// angeboten werden (#2487).
const ENDED_WITHOUT_EXIT = {
  ...RUNNING,
  care_ends_on: isoInDays(-3),
  care_ended: true,
};

function isoInDays(days: number): string {
  const d = new Date();
  d.setDate(d.getDate() + days);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

function renderWith(
  student: Record<string, unknown>,
  permissions: string[] = ["users:delete", "users:update"],
) {
  currentSearch = new URLSearchParams({ student: "1" });
  vi.mocked(useSession).mockReturnValue({
    data: {
      user: { id: "1", token: "t", permissions },
      expires: "2099-01-01",
    },
    status: "authenticated",
  } as ReturnType<typeof useSession>);
  vi.mocked(useSWRAuth).mockImplementation((key: string | null) => {
    const data = key === "database-students-list" ? [student] : [];
    return {
      data,
      isLoading: false,
      error: null,
      isValidating: false,
      mutate: vi.fn(),
    } as ReturnType<typeof useSWRAuth>;
  });
  render(<StudentsPage />);
}

describe("Datenverwaltung Kinder — Betreuung beenden", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCancelCareExit.mockResolvedValue(1);
  });

  it("offers ending the care for a child without a planned exit", () => {
    renderWith(RUNNING);
    expect(
      screen.getByRole("button", { name: /Betreuung beenden/ }),
    ).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Ende stornieren" }),
    ).toBeNull();
  });

  it("switches to change and cancel once an exit is planned", () => {
    renderWith(PLANNED);
    expect(screen.getByRole("button", { name: /Ende ändern/ })).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Ende stornieren" }),
    ).toBeVisible();
  });

  it("keeps change and cancel for an exit planned far ahead", () => {
    renderWith(PLANNED_FAR_AHEAD);
    expect(screen.getByRole("button", { name: /Ende ändern/ })).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Ende stornieren" }),
    ).toBeVisible();
  });

  it("offers no cancellation for a mere end of the enrolment phase", () => {
    renderWith(PHASE_END_ONLY);
    expect(
      screen.getByRole("button", { name: /Betreuung beenden/ }),
    ).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Ende stornieren" }),
    ).toBeNull();
  });

  it("cancels the planned exit and says so", async () => {
    renderWith(PLANNED);
    fireEvent.click(screen.getByRole("button", { name: "Ende stornieren" }));

    await waitFor(() => expect(mockCancelCareExit).toHaveBeenCalledWith(["1"]));
    await waitFor(() =>
      expect(toastSuccess).toHaveBeenCalledWith(
        expect.stringContaining("storniert"),
      ),
    );
  });

  it("shows the server's reason when the cancellation is refused", async () => {
    mockCancelCareExit.mockRejectedValue(
      new Error("Die Betreuung ist bereits beendet."),
    );
    renderWith(PLANNED);
    fireEvent.click(screen.getByRole("button", { name: "Ende stornieren" }));

    await waitFor(() =>
      expect(toastError).toHaveBeenCalledWith(
        "Die Betreuung ist bereits beendet.",
      ),
    );
  });

  it("offers resuming the care once the exit took effect", () => {
    renderWith(ENDED);
    expect(
      screen.getByRole("button", { name: /Wieder aufnehmen/ }),
    ).toBeVisible();
    expect(
      screen.queryByRole("button", { name: /Betreuung beenden/ }),
    ).toBeNull();
  });

  it("offers no resumption when no exit is recorded", () => {
    renderWith(ENDED_WITHOUT_EXIT);
    expect(
      screen.queryByRole("button", { name: /Wieder aufnehmen/ }),
    ).toBeNull();
    expect(
      screen.queryByRole("button", { name: /Betreuung beenden/ }),
    ).toBeNull();
  });

  it("hides every care-exit action without the delete permission", () => {
    renderWith(PLANNED, ["users:update"]);
    expect(screen.queryByRole("button", { name: /Ende ändern/ })).toBeNull();
    expect(
      screen.queryByRole("button", { name: "Ende stornieren" }),
    ).toBeNull();
    expect(screen.queryByText("Beendete Betreuungen")).toBeNull();
  });

  it("links the archive from the overflow menu for a deleter", () => {
    renderWith(RUNNING);
    expect(screen.getByText("Beendete Betreuungen")).toBeVisible();
  });
});
