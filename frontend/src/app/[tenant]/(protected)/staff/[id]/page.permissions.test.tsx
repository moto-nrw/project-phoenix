import { render, screen } from "@testing-library/react";
import { useSession } from "next-auth/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import StaffDetailContent from "./page";

const replaceMock = vi.fn();

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useParams: () => ({ id: "42" }),
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ replace: replaceMock }),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) =>
    key?.startsWith("staff-detail-")
      ? {
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
        }
      : { data: 0, isLoading: false, error: null },
}));

vi.mock("~/lib/staff-api", () => ({
  staffService: { getStaffById: vi.fn() },
  staffAbsenceService: { getAbsences: vi.fn() },
}));

vi.mock("~/lib/staff-helpers", () => ({
  employmentTypeLabels: { full_time: "Vollzeit" },
  getStaffDisplayType: () => "",
  getStaffLocationStatus: () => ({
    badgeColor: "",
    customBgColor: undefined,
    customShadow: undefined,
    label: "Abwesend",
  }),
}));

vi.mock("~/lib/format-utils", () => ({
  getInitials: () => "MM",
}));

vi.mock("@radix-ui/react-tabs", () => ({
  Content: ({
    children,
    value,
  }: {
    children: React.ReactNode;
    value: string;
  }) => <div data-tab-content={value}>{children}</div>,
}));

vi.mock("~/components/ui/tabs", () => ({
  Tabs: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  TabsList: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  TabsTrigger: ({
    children,
    disabled,
  }: {
    children: React.ReactNode;
    disabled?: boolean;
  }) => <button disabled={disabled}>{children}</button>,
}));

vi.mock("~/components/staff/uebersicht-tab", () => ({
  UebersichtTab: () => <div data-testid="uebersicht-tab" />,
}));

vi.mock("~/components/staff/zeiterfassung-tab", () => ({
  ZeiterfassungTab: () => <div data-testid="zeiterfassung-tab" />,
}));

vi.mock("~/components/staff/arbeitszeitmodell-tab", () => ({
  ArbeitszeitmodellTab: () => <div data-testid="arbeitszeitmodell-tab" />,
}));

vi.mock("~/components/staff/abwesenheiten-tab", () => ({
  AbwesenheitenTab: () => <div data-testid="abwesenheiten-tab" />,
}));

vi.mock("./page-skeleton", () => ({
  StaffDetailSkeleton: () => <div data-testid="staff-detail-skeleton" />,
}));

describe("StaffDetailContent permissions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
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

    expect(
      screen.getByRole("button", { name: "Übersicht" }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("uebersicht-tab")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Zeiterfassung" }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("zeiterfassung-tab")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Arbeitszeitmodell" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Stammdaten" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Bearbeiten" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Abrechnung" }),
    ).not.toBeInTheDocument();
    expect(replaceMock).not.toHaveBeenCalled();
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

    expect(screen.getByRole("link", { name: "Abrechnung" })).toHaveAttribute(
      "href",
      "/payroll",
    );
  });

  it.each(["users:read", "users:update", "staff:financial"])(
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
        screen.getByRole("button", { name: "Stammdaten" }),
      ).toBeInTheDocument();
      expect(replaceMock).not.toHaveBeenCalled();
    },
  );
});
