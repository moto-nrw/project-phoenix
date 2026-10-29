import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MODULE_KEYS } from "~/lib/modules";
import { useBookedModules } from "~/lib/tenant-context";
import { useSession } from "next-auth/react";
import Link from "next/link";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { staffService } from "~/lib/staff-api";
import { catalogText } from "~/test/error-catalog-text";
import StaffDetailContent from "./page";

const replaceMock = vi.fn();
const searchParams = vi.hoisted(() => new URLSearchParams());
const { mockRecordUpdate, mockUpdateTeacher, mockDeleteTeacher } = vi.hoisted(
  () => ({
    mockRecordUpdate: vi.fn(),
    mockUpdateTeacher: vi.fn(() => Promise.resolve()),
    mockDeleteTeacher: vi.fn(),
  }),
);
const detailRequest = vi.hoisted(() => ({
  error: null as unknown,
  endedOn: null as string | null,
  avatarUrl: undefined as string | undefined,
  plannedExitDate: null as string | null,
  mutate: vi.fn(),
}));
const roleAssignment = vi.hoisted(() => ({ currentIsLehrkraft: false }));

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useParams: () => ({ id: "42" }),
  useSearchParams: () => searchParams,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ replace: replaceMock, push: vi.fn() }),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: vi.fn(), error: vi.fn() }),
}));

vi.mock("~/lib/teacher-api", () => ({
  teacherService: {
    updateTeacher: mockUpdateTeacher,
    deleteTeacher: mockDeleteTeacher,
  },
}));

// Der Personal-Datensatz von GET /api/staff/{id}: Kopf der Personalakte und
// Reiter „Konto" (#3115) leiten sich daraus ab (#4993). Stabil je Zustand, wie
// die Daten aus dem SWR-Cache.
const staffRecords = new Map<string, Record<string, unknown>>();
function staffRecord(): Record<string, unknown> {
  const cacheKey = `${detailRequest.endedOn}|${detailRequest.avatarUrl}|${detailRequest.plannedExitDate}`;
  const cached = staffRecords.get(cacheKey);
  if (cached) return cached;
  const record = {
    id: "42",
    started_on: "2024-09-02",
    ended_on: detailRequest.endedOn,
    planned_exit_date: detailRequest.plannedExitDate,
    avatar: detailRequest.avatarUrl ?? null,
    name: "Mila Muster",
    firstName: "Mila",
    lastName: "Muster",
    first_name: "Mila",
    last_name: "Muster",
    account_role: "teacher",
    role: "Betreuung",
    qualifications: null,
    staff_notes: null,
    email: "mila@example.test",
    tag_id: "ABC123",
    account_id: "7",
    created_at: "2024-09-02T08:00:00Z",
    updated_at: "2024-09-02T08:00:00Z",
  };
  staffRecords.set(cacheKey, record);
  return record;
}

vi.mock("~/lib/database/service-factory", () => ({
  createCrudService: () => ({
    getList: vi.fn(),
    getOne: vi.fn(),
    update: mockRecordUpdate,
    delete: vi.fn(),
  }),
}));

vi.mock("~/lib/swr", () => ({
  useTenantMutate: () => vi.fn(),
  useTenantMutateMatching: () => vi.fn(),
  useInvalidateMatching: () => vi.fn(),
  useSWRAuth: (key: string | null, fetcher?: () => Promise<unknown>) => {
    if (key?.startsWith("staff-history-"))
      return { data: [], isLoading: false, error: null };
    if (key?.startsWith("staff-record-")) {
      void fetcher?.();
      if (detailRequest.error) {
        return {
          data: undefined,
          isLoading: false,
          error: detailRequest.error,
          mutate: detailRequest.mutate,
        };
      }
      return {
        data: staffRecord(),
        isLoading: false,
        error: null,
        mutate: detailRequest.mutate,
      };
    }
    if (key?.startsWith("staff-detail-")) {
      void fetcher?.();
      if (detailRequest.error) {
        return {
          data: undefined,
          isLoading: false,
          error: detailRequest.error,
          mutate: detailRequest.mutate,
        };
      }
      return {
        data: {
          id: "42",
          startedOn: "2024-09-02",
          endedOn: detailRequest.endedOn,
          avatarUrl: detailRequest.avatarUrl,
          name: "Mila Muster",
          firstName: "Mila",
          lastName: "Muster",
          hasRfid: false,
          isTeacher: false,
          isSupervising: false,
          supervisions: [],
        },
        isLoading: false,
        error: null,
      };
    }
    if (key?.startsWith("staff-role-assignment-")) {
      return {
        data: {
          options: [{ id: "1", name: "Administration", systemName: "admin" }],
          currentRoleIds: ["1"],
          currentIsLehrkraft: roleAssignment.currentIsLehrkraft,
        },
        isLoading: false,
        error: null,
      };
    }
    return { data: 0, isLoading: false, error: null };
  },
}));

vi.mock("~/lib/staff-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/staff-api")>()),
  staffService: {
    getStaffRecord: vi.fn(),
    getFinancialProfile: vi.fn(),
    getDocumentProfile: vi.fn(),
  },
  staffAbsenceService: { getAbsences: vi.fn() },
  staffHistoryService: { getHistory: vi.fn() },
}));

vi.mock("~/lib/staff-helpers", () => ({
  employmentTypeLabels: { full_time: "Vollzeit" },
  getStaffDisplayType: () => "",
  getStaffLocationStatus: () => ({
    badgeColor: "",
    customBgColor: "#83CD2D",
    customShadow: undefined,
    label: "Abwesend",
  }),
}));

vi.mock("~/lib/format-utils", () => ({
  getInitials: () => "MM",
}));

vi.mock("~/components/staff/uebersicht-tab", () => ({
  UebersichtTab: () => <div data-testid="uebersicht-tab" />,
}));

vi.mock("~/components/staff/zeiterfassung-tab", () => ({
  ZeiterfassungTab: ({ initialDate }: { initialDate?: string }) => (
    <div data-testid="zeiterfassung-tab" data-initial-date={initialDate} />
  ),
}));

vi.mock("~/components/staff/arbeitszeitmodell-tab", () => ({
  ArbeitszeitmodellTab: () => <div data-testid="arbeitszeitmodell-tab" />,
}));

vi.mock("~/components/staff/abwesenheiten-tab", () => ({
  AbwesenheitenTab: () => <div data-testid="abwesenheiten-tab" />,
}));

vi.mock("~/components/staff/dokumente-tab", () => ({
  DokumenteTab: () => <div data-testid="dokumente-tab" />,
}));

vi.mock("~/components/staff/klassen-tab", () => ({
  KlassenTab: () => <div data-testid="klassen-tab" />,
}));

vi.mock("~/components/staff/stammdaten-tab", () => ({
  StammdatenTab: ({
    canManagePayroll,
    canManagePayrollSettings,
  }: {
    canManagePayroll: boolean;
    canManagePayrollSettings: boolean;
  }) => (
    <div data-testid="stammdaten-tab">
      {canManagePayroll ? <button>Bearbeiten</button> : null}
      {canManagePayroll && canManagePayrollSettings ? (
        <Link href="/payroll">Abrechnung</Link>
      ) : null}
    </div>
  ),
}));

vi.mock("./page-skeleton", () => ({
  StaffDetailSkeleton: () => <div data-testid="staff-detail-skeleton" />,
}));

describe("StaffDetailContent permissions", () => {
  it.each(["konto", "klassen", "arbeitszeitmodell", "abwesenheiten"])(
    "keeps former employment read-only for a direct %s bookmark",
    (requestedTab) => {
      detailRequest.endedOn = "2026-07-31";
      searchParams.set("tab", requestedTab);
      searchParams.set("from", "/staff?status=ehemalige&search=Mila");
      vi.mocked(useSession).mockReturnValue({
        data: {
          user: {
            id: "7",
            roles: ["admin"],
            permissions: ["admin:*"],
          },
          expires: "2099-01-01",
        },
        status: "authenticated",
        update: vi.fn(),
      });
      render(<StaffDetailContent />);
      expect(screen.getByText("02.09.2024")).toBeInTheDocument();
      expect(screen.getByText("31.07.2026")).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: "Weitere Aktionen" }),
      ).not.toBeInTheDocument();
      for (const label of [
        "Konto",
        "Klassen",
        "Arbeitszeitmodell",
        "Abwesenheiten",
      ])
        expect(
          screen.queryByRole("tab", { name: label }),
        ).not.toBeInTheDocument();
      expect(
        screen.getByRole("link", { name: "Zurück zu den Mitarbeitenden" }),
      ).toHaveAttribute(
        "href",
        "/test-tenant/staff?status=ehemalige&search=Mila",
      );
      fireEvent.click(screen.getByRole("tab", { name: "Stammdaten" }));
      expect(
        screen.queryByRole("button", { name: "Bearbeiten" }),
      ).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole("tab", { name: "Zeiterfassung" }));
      expect(
        screen.queryByRole("button", {
          name: /nachtragen|öffnen|Buchen|Bearbeiten/,
        }),
      ).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole("tab", { name: "Dokumente" }));
      expect(
        screen.getByText(
          "Die Personaldokumente wurden beim Austritt endgültig gelöscht.",
        ),
      ).toBeInTheDocument();
    },
  );
  beforeEach(() => {
    vi.clearAllMocks();
    detailRequest.error = null;
    detailRequest.endedOn = null;
    detailRequest.avatarUrl = undefined;
    detailRequest.plannedExitDate = null;
    roleAssignment.currentIsLehrkraft = false;
    searchParams.delete("from");
    searchParams.delete("tab");
    searchParams.delete("date");
    window.scrollTo = vi.fn();
  });

  // #4432: the redesign in #1310 dropped the picture and left the initials.
  it("shows the uploaded profile picture in the header", () => {
    detailRequest.avatarUrl = "/api/staff/42/avatar";
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          roles: ["admin"],
          permissions: ["admin:*"],
        },
        expires: "2099-01-01T00:00:00.000Z",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    const { container } = render(<StaffDetailContent />);

    expect(container.querySelector("img")?.getAttribute("src")).toMatch(
      /\/api\/staff\/42\/avatar$/,
    );
  });

  it("shows overview and time tracking to managers without admin role", () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          roles: ["teacher"],
          permissions: ["time_tracking:manage"],
        },
        expires: "2099-01-01T00:00:00.000Z",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<StaffDetailContent />);

    // Seitenreiter tragen jetzt die Rolle „tab"; nur der aktive Reiter
    // rendert seinen Inhalt.
    expect(screen.getByRole("tab", { name: "Übersicht" })).toBeInTheDocument();
    expect(screen.getByTestId("uebersicht-tab")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("tab", { name: "Zeiterfassung" }));
    expect(screen.getByTestId("zeiterfassung-tab")).toBeInTheDocument();
    expect(
      screen.queryByRole("tab", { name: "Arbeitszeitmodell" }),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("tab", { name: "Stammdaten" }));
    expect(
      screen.getByRole("button", { name: "Bearbeiten" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Abrechnung" }),
    ).not.toBeInTheDocument();
    expect(replaceMock).not.toHaveBeenCalled();
  });

  // Modul-Register (#4894): Zeiterfassung, Arbeitszeitmodell und
  // Abwesenheiten gehören zur Zeiterfassung, Stammdaten und Dokumente zur
  // Personalakte. Ohne gebuchtes Modul fehlen ihre Reiter.
  it("blendet die Reiter nicht gebuchter Module aus", () => {
    vi.mocked(useBookedModules).mockReturnValue(
      MODULE_KEYS.filter(
        (key) => key !== "zeiterfassung" && key !== "personalakte",
      ),
    );
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          roles: ["admin"],
          permissions: ["admin:*"],
        },
        expires: "2099-01-01T00:00:00.000Z",
      },
      status: "authenticated",
      update: vi.fn(),
    });
    try {
      render(<StaffDetailContent />);

      // Anker: die Basis-Reiter bleiben.
      expect(screen.getByRole("tab", { name: "Konto" })).toBeInTheDocument();
      expect(screen.getByRole("tab", { name: "Klassen" })).toBeInTheDocument();
      for (const label of [
        "Übersicht",
        "Zeiterfassung",
        "Arbeitszeitmodell",
        "Abwesenheiten",
        "Stammdaten",
        "Dokumente",
      ]) {
        expect(
          screen.queryByRole("tab", { name: label }),
        ).not.toBeInTheDocument();
      }
    } finally {
      vi.mocked(useBookedModules).mockReturnValue([...MODULE_KEYS]);
    }
  });

  it("fällt nach einer entfernten Modulbuchung auf den ersten sichtbaren Reiter zurück", () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          roles: ["admin"],
          permissions: ["admin:*"],
        },
        expires: "2099-01-01T00:00:00.000Z",
      },
      status: "authenticated",
      update: vi.fn(),
    });
    try {
      const { rerender } = render(<StaffDetailContent />);
      fireEvent.click(screen.getByRole("tab", { name: "Zeiterfassung" }));
      expect(screen.getByTestId("zeiterfassung-tab")).toBeInTheDocument();

      vi.mocked(useBookedModules).mockReturnValue(
        MODULE_KEYS.filter((key) => key !== "zeiterfassung"),
      );
      rerender(<StaffDetailContent />);

      // Anker: Stammdaten bleibt nach der Buchungsänderung sichtbar.
      expect(
        screen.getByRole("tab", { name: "Stammdaten" }),
      ).toBeInTheDocument();
      expect(screen.getByTestId("stammdaten-tab")).toBeInTheDocument();
      expect(screen.queryByTestId("zeiterfassung-tab")).not.toBeInTheDocument();
    } finally {
      vi.mocked(useBookedModules).mockReturnValue([...MODULE_KEYS]);
    }
  });

  it("blendet Modul-Reiter bei früheren Beschäftigungen aus", () => {
    detailRequest.endedOn = "2026-07-31";
    vi.mocked(useBookedModules).mockReturnValue(
      MODULE_KEYS.filter(
        (key) => key !== "zeiterfassung" && key !== "personalakte",
      ),
    );
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          roles: ["admin"],
          permissions: ["admin:*"],
        },
        expires: "2099-01-01T00:00:00.000Z",
      },
      status: "authenticated",
      update: vi.fn(),
    });
    try {
      render(<StaffDetailContent />);

      // Anker: Der Bereich der früheren Beschäftigung bleibt erreichbar.
      expect(
        screen.getByRole("tab", { name: "Beschäftigung" }),
      ).toBeInTheDocument();
      for (const label of ["Stammdaten", "Zeiterfassung", "Dokumente"]) {
        expect(
          screen.queryByRole("tab", { name: label }),
        ).not.toBeInTheDocument();
      }
    } finally {
      vi.mocked(useBookedModules).mockReturnValue([...MODULE_KEYS]);
    }
  });

  it("passes the selected date from a time-tracking deep link", () => {
    searchParams.set("tab", "zeiterfassung");
    searchParams.set("date", "2026-01-05");
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          roles: ["teacher"],
          permissions: ["time_tracking:manage"],
        },
        expires: "2099-01-01T00:00:00.000Z",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<StaffDetailContent />);

    expect(screen.getByTestId("zeiterfassung-tab")).toHaveAttribute(
      "data-initial-date",
      "2026-01-05",
    );
  });

  it("ignores an invalid date from a time-tracking deep link", () => {
    searchParams.set("tab", "zeiterfassung");
    searchParams.set("date", "2026-02-31");
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          roles: ["teacher"],
          permissions: ["time_tracking:manage"],
        },
        expires: "2099-01-01T00:00:00.000Z",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<StaffDetailContent />);

    expect(screen.getByTestId("zeiterfassung-tab")).not.toHaveAttribute(
      "data-initial-date",
    );
  });

  it("links payroll settings for managers with config permission", () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          roles: ["teacher"],
          permissions: ["time_tracking:manage", "config:manage"],
        },
        expires: "2099-01-01T00:00:00.000Z",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<StaffDetailContent />);

    fireEvent.click(screen.getByRole("tab", { name: "Stammdaten" }));
    expect(screen.getByRole("link", { name: "Abrechnung" })).toHaveAttribute(
      "href",
      "/payroll",
    );
  });

  // The backend grants everything to admin:* / *:* holders regardless of the
  // role name, so a custom role carrying the wildcard must see the same
  // admin-gated UI as the literal admin role.
  it.each(["admin:*", "*:*"])(
    "treats a custom role holding %s as an admin",
    (permission) => {
      vi.mocked(useSession).mockReturnValue({
        data: {
          user: {
            id: "7",
            roles: ["lohnbuero"],
            permissions: [permission],
          },
          expires: "2099-01-01T00:00:00.000Z",
        },
        status: "authenticated",
        update: vi.fn(),
      });

      render(<StaffDetailContent />);

      fireEvent.click(screen.getByRole("tab", { name: "Arbeitszeitmodell" }));
      expect(screen.getByTestId("arbeitszeitmodell-tab")).toBeInTheDocument();
      fireEvent.click(screen.getByRole("tab", { name: "Abwesenheiten" }));
      expect(screen.getByTestId("abwesenheiten-tab")).toBeInTheDocument();
      expect(staffService.getStaffRecord).toHaveBeenCalledWith("42");
      expect(replaceMock).not.toHaveBeenCalled();
    },
  );

  it.each(["staff:stammdaten", "staff:financial"])(
    "shows Stammdaten to a role with %s",
    (permission) => {
      vi.mocked(useSession).mockReturnValue({
        data: {
          user: {
            id: "7",
            roles: ["teacher"],
            permissions: [permission],
          },
          expires: "2099-01-01T00:00:00.000Z",
        },
        status: "authenticated",
        update: vi.fn(),
      });

      render(<StaffDetailContent />);

      expect(
        screen.getByRole("tab", { name: "Stammdaten" }),
      ).toBeInTheDocument();
      expect(replaceMock).not.toHaveBeenCalled();
    },
  );

  it("shows only the documents tab to a health-document role", () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          roles: ["teacher"],
          permissions: ["staff_documents:health"],
        },
        expires: "2099-01-01T00:00:00.000Z",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<StaffDetailContent />);

    expect(screen.getByRole("tab", { name: "Dokumente" })).toBeInTheDocument();
    expect(staffService.getDocumentProfile).toHaveBeenCalledWith("42");
    expect(replaceMock).not.toHaveBeenCalled();
  });

  it.each(["staff:documents", "staff:financial"])(
    "opens the documents tab from a document-directory deep link for %s",
    (permission) => {
      searchParams.set("tab", "dokumente");
      vi.mocked(useSession).mockReturnValue({
        data: {
          user: {
            id: "7",
            roles: ["teacher"],
            permissions: [permission],
          },
          expires: "2099-01-01T00:00:00.000Z",
        },
        status: "authenticated",
        update: vi.fn(),
      });

      render(<StaffDetailContent />);

      // Der Deep-Link öffnet den Reiter direkt: er ist ausgewählt und sein
      // Inhalt steht auf der Seite.
      expect(screen.getByRole("tab", { name: "Dokumente" })).toHaveAttribute(
        "aria-selected",
        "true",
      );
      expect(screen.getByTestId("dokumente-tab")).toBeInTheDocument();
    },
  );

  // users:read is the list tier; the Stammdaten sections carry HR-file data
  // (birthday, private address, contract terms) and stay closed to it —
  // mirrors the backend route gate.
  // Seit #3115 ist die Personalakte die einzige Objektansicht: wer nur
  // users:read hat, sah vorher das Pane der Datenverwaltung und sieht jetzt
  // hier den Reiter „Konto" — ohne Personalakte, ohne Zeiterfassung.
  it("shows only the account tab to a role with users:read", () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          roles: ["teacher"],
          permissions: ["users:read"],
        },
        expires: "2099-01-01T00:00:00.000Z",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<StaffDetailContent />);

    expect(replaceMock).not.toHaveBeenCalled();
    expect(screen.getByRole("tab", { name: "Konto" })).toBeInTheDocument();
    expect(
      screen.queryByRole("tab", { name: "Stammdaten" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("tab", { name: "Zeiterfassung" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Rolle")).toBeInTheDocument();
    // Ohne staff:manage kein Bearbeiten, ohne users:delete kein Löschen: das
    // Kebab fehlt ganz statt leer zu bleiben.
    expect(
      screen.queryByRole("button", { name: "Weitere Aktionen" }),
    ).not.toBeInTheDocument();
  });

  it("offers editing and deleting the record to the account managers", () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          roles: ["teacher"],
          permissions: ["staff:manage", "users:delete", "users:manage"],
        },
        expires: "2099-01-01T00:00:00.000Z",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<StaffDetailContent />);

    // Bearbeitet wird im Reiter „Konto" (#3116), nicht aus dem Kebab heraus.
    expect(screen.getByRole("button", { name: "Bearbeiten" })).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    expect(
      screen.queryByRole("menuitem", { name: "Bearbeiten" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "Löschen" }),
    ).toBeInTheDocument();
    // Die Kontoaktionen mit eigenem Ablauf stehen im Kebab des Kopfes.
    expect(
      screen.getByRole("menuitem", {
        name: "Zwei-Faktor-Authentifizierung verwalten",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "Betreuung verwalten" }),
    ).toBeInTheDocument();
    // Die Systemrolle ist ein Feld des Bearbeiten-Zustands, kein Dialog.
    expect(
      screen.queryByRole("button", { name: "Rolle verwalten" }),
    ).not.toBeInTheDocument();
  });

  // Geplanter Austritt (#4809): mit dem Löschrecht planen, ändern und
  // zurücknehmen; der Kopf zeigt den Tag.
  it("plans, changes and withdraws an exit with the delete permission", () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          roles: ["teacher"],
          permissions: ["users:read", "users:delete"],
        },
        expires: "2099-01-01T00:00:00.000Z",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    const { unmount } = render(<StaffDetailContent />);
    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    expect(
      screen.getByRole("menuitem", { name: "Austritt planen" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: "Austritt zurücknehmen" }),
    ).not.toBeInTheDocument();
    unmount();

    detailRequest.plannedExitDate = "2026-12-31";
    render(<StaffDetailContent />);
    expect(screen.getByText("Austritt am 31.12.2026")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    expect(
      screen.getByRole("menuitem", { name: "Austritt ändern" }),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Austritt zurücknehmen" }),
    );
    expect(
      screen.getByRole("heading", { name: "Austritt zurücknehmen?" }),
    ).toBeInTheDocument();
  });

  it("offers no exit planning without the delete permission", () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          roles: ["teacher"],
          permissions: ["users:read", "users:manage"],
        },
        expires: "2099-01-01T00:00:00.000Z",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<StaffDetailContent />);
    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    expect(
      screen.getByRole("menuitem", { name: "Betreuung verwalten" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("menuitem", { name: "Austritt planen" }),
    ).not.toBeInTheDocument();
  });

  it("offers the edit state of the account tab without account actions to staff:manage alone", () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          roles: ["teacher"],
          permissions: ["staff:manage"],
        },
        expires: "2099-01-01T00:00:00.000Z",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<StaffDetailContent />);

    expect(screen.getByRole("button", { name: "Bearbeiten" })).toBeVisible();
    // Ohne users:manage und users:delete gibt es keinen Kebab.
    expect(
      screen.queryByRole("button", { name: "Weitere Aktionen" }),
    ).not.toBeInTheDocument();
  });

  it("saves account fields through the person-aware teacher update", async () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          roles: ["teacher"],
          permissions: ["staff:manage", "users:update", "users:manage"],
        },
        expires: "2099-01-01T00:00:00.000Z",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<StaffDetailContent />);

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.change(screen.getByLabelText("Vorname"), {
      target: { value: "Milena" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mockUpdateTeacher).toHaveBeenCalledWith(
        "42",
        expect.objectContaining({
          first_name: "Milena",
          last_name: "Muster",
        }),
        expect.objectContaining({
          id: "42",
          first_name: "Mila",
          last_name: "Muster",
        }),
      );
    });
    expect(mockRecordUpdate).not.toHaveBeenCalled();
  });

  it("offers account-role editing without staff:manage", () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          roles: ["teacher"],
          permissions: ["users:read", "users:manage"],
        },
        expires: "2099-01-01T00:00:00.000Z",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<StaffDetailContent />);

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    expect(screen.getByRole("combobox", { name: "Rolle" })).toBeVisible();
    expect(screen.queryByLabelText("Position")).not.toBeInTheDocument();
    expect(
      screen.queryByLabelText("Notizen der Leitung"),
    ).not.toBeInTheDocument();
  });

  // #4431: the Lehrkraft hint sends the reader to „Löschen" only when the
  // header menu offers it (users:delete); otherwise it names who can.
  it.each([
    {
      permissions: ["users:read", "users:manage", "users:delete"],
      canDelete: true,
    },
    { permissions: ["users:read", "users:manage"], canDelete: false },
  ])(
    "matches the Lehrkraft role hint to the delete permission ($permissions)",
    ({ permissions, canDelete }) => {
      roleAssignment.currentIsLehrkraft = true;
      vi.mocked(useSession).mockReturnValue({
        data: {
          user: {
            id: "7",
            roles: ["teacher"],
            permissions,
          },
          expires: "2099-01-01T00:00:00.000Z",
        },
        status: "authenticated",
        update: vi.fn(),
      });

      render(<StaffDetailContent />);

      fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
      expect(screen.queryByRole("menuitem", { name: "Löschen" }) !== null).toBe(
        canDelete,
      );
      fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
      expect(screen.queryByText("Wählen Sie „Löschen“.") !== null).toBe(
        canDelete,
      );
      expect(
        screen.queryByText(/Das kann nur, wer Personal löschen darf/) !== null,
      ).toBe(!canDelete);
    },
  );

  it("redirects a role without any staff permission back to the referrer", () => {
    searchParams.set("from", "/database/personal");
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          roles: ["teacher"],
          permissions: ["schedules:read"],
        },
        expires: "2099-01-01T00:00:00.000Z",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<StaffDetailContent />);

    expect(replaceMock).toHaveBeenCalledWith("/database/personal");
    searchParams.delete("from");
  });

  it("does not redirect a role without permission to an external referrer", () => {
    searchParams.set("from", "//attacker.example");
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          roles: ["teacher"],
          permissions: ["schedules:read"],
        },
        expires: "2099-01-01T00:00:00.000Z",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<StaffDetailContent />);

    expect(replaceMock).toHaveBeenCalledWith("/staff");
    searchParams.delete("from");
  });

  describe("Fehlerweg (#2514)", () => {
    function mockAdmin(permissions: string[]) {
      vi.mocked(useSession).mockReturnValue({
        data: {
          user: {
            id: "7",
            roles: ["teacher"],
            permissions,
          },
          expires: "2099-01-01T00:00:00.000Z",
        },
        status: "authenticated",
        update: vi.fn(),
      });
    }

    it("zeigt einen Ladefehler der Personalakte mit Wiederholen", async () => {
      detailRequest.error = new ApiError("down", 503, {
        code: "general.unavailable",
      });
      mockAdmin(["time_tracking:manage"]);

      render(<StaffDetailContent />);

      expect(
        await screen.findByText(
          catalogText("general.unavailable", "die Personalakte"),
        ),
      ).toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
      expect(detailRequest.mutate).toHaveBeenCalled();
    });

    it("nennt eine gelöschte Person statt eines Ladefehlers", () => {
      detailRequest.error = new ApiError("not found", 404, {
        code: "general.input",
      });
      mockAdmin(["time_tracking:manage"]);

      render(<StaffDetailContent />);

      expect(
        screen.getByText(/Diese Person gibt es nicht mehr\./),
      ).toBeInTheDocument();
    });

    it("zeigt einen Löschfehler im offenen Dialog", async () => {
      mockDeleteTeacher.mockRejectedValueOnce(
        new ApiError("forbidden", 403, { code: "general.permission" }),
      );
      mockAdmin(["staff:manage", "users:delete", "users:manage"]);

      render(<StaffDetailContent />);

      fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
      fireEvent.click(screen.getByRole("menuitem", { name: "Löschen" }));
      const dialog = await screen.findByRole("dialog");
      fireEvent.change(screen.getByPlaceholderText("Vorname Nachname"), {
        target: { value: "Mila Muster" },
      });
      fireEvent.click(
        screen.getByRole("button", { name: "Endgültig löschen" }),
      );

      const message = catalogText("general.permission", "die Person");
      expect(await screen.findByText(message)).toBeInTheDocument();
      expect(dialog).toContainElement(screen.getByText(message));
      expect(mockDeleteTeacher).toHaveBeenCalledWith("42");
    });
  });
});
