import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  mutate: vi.fn(),
  create: vi.fn(),
  end: vi.fn(),
  success: vi.fn(),
  openCare: false,
  overview: {
    groups: [{ id: "12", name: "Robins Gruppe" }],
    targets: [{ id: "34", fullName: "Toni Test" }],
    groupHandovers: [
      {
        id: "5",
        type: "group_handover" as const,
        groupId: "12",
        groupName: "Robins Gruppe",
        substituteStaffId: "34",
        substituteStaffName: "Toni Test",
        startDate: "2026-08-31",
        endDate: "2026-08-31",
        canEnd: true,
      },
    ],
    runningSupervisions: [
      {
        id: "41",
        name: "Freispiel",
        roomName: "Atelier",
        supervisors: [{ id: "11", fullName: "Alex Alt" }],
        availableTargets: [{ id: "34", fullName: "Toni Test" }],
        isCurrentUserSupervising: true,
        canAssign: true,
      },
      {
        id: "42",
        name: "Mensa",
        roomName: "Speiseraum",
        supervisors: [{ id: "13", fullName: "Nora Neu" }],
        availableTargets: [],
        isCurrentUserSupervising: false,
        canAssign: false,
      },
    ],
  },
  schedule: {
    appointments: [
      {
        id: "77",
        date: "2026-08-31",
        startTime: "12:00",
        endTime: "13:00",
        title: "Lesezeit",
        status: "planned",
        staff: [
          {
            assignmentId: "7",
            id: "11",
            name: "Alex Alt",
            isAbsent: true,
            isSubstitute: false,
            canEnd: false,
          },
        ],
      },
    ],
    staff: [],
  },
}));

vi.mock("next-auth/react", () => ({ useSession: vi.fn() }));
vi.mock("~/lib/swr", () => ({ useSWRAuth: vi.fn() }));
vi.mock("~/lib/tenant-context", () => ({
  useOpenCareGroupMode: () => mocks.openCare,
  useTenantSlugSafe: () => "test",
  useTenantRoutingModeSafe: () => "path",
}));
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn() }),
}));
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: mocks.success }),
}));
vi.mock("~/lib/substitution-api", () => ({
  substitutionService: {
    fetchOverview: vi.fn(),
    fetchScheduleOverview: vi.fn(),
    createSubstitution: mocks.create,
    deleteSubstitution: mocks.end,
  },
}));
vi.mock("~/components/active-supervisions/add-supervisor-modal", () => ({
  AddSupervisorModal: ({ activeGroupId }: { activeGroupId: string }) => (
    <div role="dialog">Zusätzliche Aufsicht für {activeGroupId}</div>
  ),
}));

import { useSession } from "next-auth/react";
import { useSWRAuth } from "~/lib/swr";
import SubstitutionPage from "./page";

function adminSession() {
  return {
    data: {
      user: {
        roles: ["admin"],
        permissions: ["schedules:read", "schedules:manage"],
      },
    },
    status: "authenticated",
  } as never;
}

function staffSession() {
  return {
    data: {
      user: { roles: ["user"], permissions: ["schedules:read"] },
    },
    status: "authenticated",
  } as never;
}

describe("SubstitutionPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.openCare = false;
    mocks.mutate.mockResolvedValue(undefined);
    mocks.create.mockResolvedValue(undefined);
    mocks.end.mockResolvedValue(undefined);
    vi.mocked(useSession).mockReturnValue(adminSession());
    vi.mocked(useSWRAuth).mockImplementation(
      (key: string | null) =>
        ({
          data: key?.startsWith("substitution-schedule-")
            ? mocks.schedule
            : mocks.overview,
          isLoading: false,
          error: null,
          mutate: mocks.mutate,
        }) as never,
    );
  });

  it("shows all three workflows with running and today content first", () => {
    render(<SubstitutionPage />);

    const running = screen.getByRole("heading", {
      name: "Laufende Betreuungen",
    });
    const appointments = screen.getByRole("heading", { name: "Termine" });
    const groups = screen.getByRole("heading", { name: "Gruppen" });
    expect(running.compareDocumentPosition(appointments)).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    );
    expect(appointments.compareDocumentPosition(groups)).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING,
    );
    expect(screen.getByText("Freispiel")).toBeInTheDocument();
    expect(screen.getByText("Lesezeit")).toBeInTheDocument();
    expect(screen.getByText("Robins Gruppe")).toBeInTheDocument();
  });

  it("opens the additional-supervision flow only for an allowed session", () => {
    render(<SubstitutionPage />);

    expect(
      screen.getAllByRole("button", { name: "Betreuer hinzufügen" }),
    ).toHaveLength(1);
    expect(
      screen.getByText("Nur zuständige Personen können jemanden hinzufügen."),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Betreuer hinzufügen" }),
    );
    expect(screen.getByRole("dialog", { name: "" })).toHaveTextContent("41");
  });

  it("links an allowed appointment to the existing module flow", () => {
    render(<SubstitutionPage />);

    expect(
      screen.getByRole("link", { name: "Vertretung eintragen" }),
    ).toHaveAttribute(
      "href",
      expect.stringMatching(
        /^\/test\/vertretung\?d=\d{4}-\d{2}-\d{2}&block=77$/,
      ),
    );
  });

  it("keeps staff actions within their returned capabilities", () => {
    vi.mocked(useSession).mockReturnValue(staffSession());
    render(<SubstitutionPage />);

    expect(
      screen.queryByRole("link", { name: "Vertretung eintragen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText("Ein Admin kann die Vertretung eintragen."),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Gruppe übergeben" }));
    expect(
      screen.getByText("Sie können nur eigene Gruppen für heute übergeben."),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Startdatum")).not.toBeInTheDocument();
  });

  it("denies the overview to authenticated accounts without a staff or admin role", () => {
    vi.mocked(useSession).mockReturnValue({
      data: { user: { roles: ["guardian"], permissions: [] } },
      status: "authenticated",
    } as never);

    render(<SubstitutionPage />);

    expect(
      screen.getByText("Ihnen fehlt eine Berechtigung"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Laufende Betreuungen")).not.toBeInTheDocument();
  });

  it("gives wildcard admins the admin date controls", () => {
    vi.mocked(useSession).mockReturnValue({
      data: { user: { roles: ["user"], permissions: ["admin:*"] } },
      status: "authenticated",
    } as never);

    render(<SubstitutionPage />);
    fireEvent.click(screen.getByRole("button", { name: "Gruppe übergeben" }));

    expect(screen.getByLabelText("Startdatum")).toBeInTheDocument();
    expect(screen.getByLabelText("Enddatum")).toBeInTheDocument();
  });

  it("keeps the other workflows available for open-care schools", () => {
    mocks.openCare = true;
    render(<SubstitutionPage />);

    expect(screen.getByText("Keine Gruppenübergabe nötig")).toBeInTheDocument();
    expect(screen.getByText("Freispiel")).toBeInTheDocument();
    expect(screen.getByText("Lesezeit")).toBeInTheDocument();
  });

  it("ends a group handover through the module", async () => {
    render(<SubstitutionPage />);

    fireEvent.click(screen.getByRole("button", { name: "Beenden" }));
    const buttons = screen.getAllByRole("button", { name: "Beenden" });
    fireEvent.click(buttons[buttons.length - 1]!);

    await waitFor(() => expect(mocks.end).toHaveBeenCalledWith("5"));
    expect(mocks.mutate).toHaveBeenCalled();
    expect(mocks.success).toHaveBeenCalledWith("Die Übergabe wurde beendet.");
  });

  it("does not present load failures as empty results", () => {
    vi.mocked(useSWRAuth).mockImplementation(
      () =>
        ({
          data: undefined,
          isLoading: false,
          error: new Error("offline"),
          mutate: mocks.mutate,
        }) as never,
    );

    render(<SubstitutionPage />);

    expect(
      screen.getByText(/Vertretungen konnten nicht geladen werden/),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Keine laufenden Betreuungen"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("Keine Gruppenübergaben"),
    ).not.toBeInTheDocument();
  });

  it("shows the access state inside the existing page scaffold", () => {
    const error = new Error("Vertretungen konnten nicht geladen werden.");
    error.name = "SubstitutionAccessError";
    vi.mocked(useSWRAuth).mockImplementation(
      () =>
        ({
          data: undefined,
          isLoading: false,
          error,
          mutate: mocks.mutate,
        }) as never,
    );

    render(<SubstitutionPage />);

    expect(
      screen.getByText("Ihnen fehlt eine Berechtigung"),
    ).toBeInTheDocument();
    expect(
      screen.getAllByRole("heading", { name: "Vertretungen" }),
    ).toHaveLength(1);
    expect(screen.queryByText("Laufende Betreuungen")).not.toBeInTheDocument();
  });

  it("explains every empty section", () => {
    mocks.overview.groups = [];
    mocks.overview.targets = [];
    mocks.overview.groupHandovers = [];
    mocks.overview.runningSupervisions = [];
    mocks.schedule.appointments = [];
    render(<SubstitutionPage />);

    expect(screen.getByText("Keine laufenden Betreuungen")).toBeInTheDocument();
    expect(
      screen.getByText("Heute keine Terminvertretungen"),
    ).toBeInTheDocument();
    expect(screen.getByText("Keine Gruppenübergaben")).toBeInTheDocument();
    expect(screen.getByText(/Keine Gruppe verfügbar/)).toBeInTheDocument();
  });
});
