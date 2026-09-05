import { fireEvent, render, screen } from "@testing-library/react";
import { useSession } from "next-auth/react";
import Link from "~/components/ui/navigation-link";
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
  useTenantRouter: () => ({ replace: replaceMock }),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null, fetcher?: () => Promise<unknown>) => {
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
  it("redirects a role with only users:read away from the detail page", () => {
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

    expect(replaceMock).toHaveBeenCalledWith("/staff");
    expect(
      screen.queryByRole("button", { name: "Stammdaten" }),
    ).not.toBeInTheDocument();
  });
});
