import { fireEvent, render, screen } from "@testing-library/react";
import { useSession } from "next-auth/react";
import Link from "next/link";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { staffService } from "~/lib/staff-api";
import StaffDetailContent from "./page";

const replaceMock = vi.fn();
const searchParams = vi.hoisted(() => new URLSearchParams());

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

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: vi.fn(), error: vi.fn() }),
}));

// Der Personal-Datensatz für den Reiter „Konto" (#3115).
const staffRecord = {
  id: "42",
  name: "Mila Muster",
  first_name: "Mila",
  last_name: "Muster",
  account_role: "teacher",
  role: "Betreuung",
  email: "mila@example.test",
  tag_id: "ABC123",
  account_id: 7,
};

vi.mock("~/lib/database/service-factory", () => ({
  createCrudService: () => ({
    getList: vi.fn(),
    getOne: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
  }),
}));

vi.mock("~/lib/swr", () => ({
  useTenantMutate: () => vi.fn(),
  useSWRAuth: (key: string | null, fetcher?: () => Promise<unknown>) => {
    if (key?.startsWith("staff-record-")) {
      return { data: staffRecord, isLoading: false, error: null };
    }
    if (key?.startsWith("staff-detail-")) {
      void fetcher?.();
      return {
        data: {
          id: "42",
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
    return { data: 0, isLoading: false, error: null };
  },
}));

vi.mock("~/lib/staff-api", () => ({
  staffService: {
    getStaffById: vi.fn(),
    getFinancialProfile: vi.fn(),
    getDocumentProfile: vi.fn(),
  },
  staffAbsenceService: { getAbsences: vi.fn() },
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
  beforeEach(() => {
    vi.clearAllMocks();
    searchParams.delete("tab");
    searchParams.delete("date");
    window.scrollTo = vi.fn();
  });

  it("shows overview and time tracking to managers without admin role", () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          token: "test-token",
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

  it("passes the selected date from a time-tracking deep link", () => {
    searchParams.set("tab", "zeiterfassung");
    searchParams.set("date", "2026-01-05");
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          token: "test-token",
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
          token: "test-token",
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
          token: "test-token",
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
            token: "test-token",
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
      expect(staffService.getStaffById).toHaveBeenCalledWith("42");
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
            token: "test-token",
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
          token: "test-token",
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
            token: "test-token",
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
          token: "test-token",
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
    expect(screen.getByText("Systemrolle")).toBeInTheDocument();
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
          token: "test-token",
          roles: ["teacher"],
          permissions: ["staff:manage", "users:delete", "users:manage"],
        },
        expires: "2099-01-01T00:00:00.000Z",
      },
      status: "authenticated",
      update: vi.fn(),
    });

    render(<StaffDetailContent />);

    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    expect(
      screen.getByRole("menuitem", { name: "Bearbeiten" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "Löschen" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Rolle verwalten" }),
    ).toBeInTheDocument();
  });

  it("redirects a role without any staff permission back to the referrer", () => {
    searchParams.set("from", "/database/personal");
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "7",
          token: "test-token",
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
});
