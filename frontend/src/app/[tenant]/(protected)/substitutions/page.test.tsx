import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  mutate: vi.fn(),
  create: vi.fn(),
  end: vi.fn(),
  success: vi.fn(),
  toastError: vi.fn(),
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
// Nur die Toasts ersetzen; der Fehlerweg (Katalogtexte) bleibt echt.
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: mocks.success, error: mocks.toastError }),
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
import { ApiError } from "~/lib/api-error";
import { useSWRAuth } from "~/lib/swr";
import { catalogText } from "~/test/error-catalog-text";
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

  it("does not present load failures as empty results", async () => {
    vi.mocked(useSWRAuth).mockImplementation(
      () =>
        ({
          data: undefined,
          isLoading: false,
          error: new ApiError("Failed to fetch", 503, {
            code: "general.unavailable",
          }),
          mutate: mocks.mutate,
        }) as never,
    );

    render(<SubstitutionPage />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Übersicht der Vertretungen"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Failed to fetch/)).not.toBeInTheDocument();
    expect(mocks.toastError).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(mocks.mutate).toHaveBeenCalledTimes(1);
    expect(
      screen.queryByText("Keine laufenden Betreuungen"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("Keine Gruppenübergaben"),
    ).not.toBeInTheDocument();
  });

  it("shows the access state inside the existing page scaffold", () => {
    const error = new ApiError("forbidden", 403, {
      code: "substitutions.forbidden",
    });
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

  it("shows a failed appointment load inside its card with a retry", async () => {
    const scheduleMutate = vi.fn();
    vi.mocked(useSWRAuth).mockImplementation(
      (key: string | null) =>
        (key?.startsWith("substitution-schedule-")
          ? {
              data: undefined,
              isLoading: false,
              error: new ApiError("boom", 500, { code: "general.server" }),
              mutate: scheduleMutate,
            }
          : {
              data: mocks.overview,
              isLoading: false,
              error: null,
              mutate: mocks.mutate,
            }) as never,
    );

    render(<SubstitutionPage />);

    expect(
      await screen.findByText(
        catalogText("general.server", "die Liste der Terminvertretungen"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Heute keine Terminvertretungen"),
    ).not.toBeInTheDocument();
    // Die übrigen Karten bleiben bedienbar.
    expect(screen.getByText("Freispiel")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(scheduleMutate).toHaveBeenCalledTimes(1);
  });

  it("keeps a failed group handover in the dialog and marks the named field", async () => {
    mocks.create.mockRejectedValueOnce(
      new ApiError("target staff not eligible", 400, {
        code: "substitutions.invalid_target",
        errors: [{ field: "target_staff_id", reason: "invalid" }],
      }),
    );
    render(<SubstitutionPage />);

    fireEvent.click(screen.getByRole("button", { name: "Gruppe übergeben" }));
    fireEvent.click(screen.getByRole("combobox", { name: "Gruppe" }));
    fireEvent.click(screen.getByRole("option", { name: "Robins Gruppe" }));
    fireEvent.click(screen.getByRole("combobox", { name: "Betreuungskraft" }));
    fireEvent.click(screen.getByRole("option", { name: "Toni Test" }));
    fireEvent.click(screen.getByRole("button", { name: "Zuweisen" }));

    expect(
      await screen.findByText(
        catalogText("substitutions.invalid_target", "die Gruppenübergabe"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("target staff not eligible"),
    ).not.toBeInTheDocument();
    expect(mocks.toastError).not.toHaveBeenCalled();
    expect(mocks.success).not.toHaveBeenCalled();
    expect(
      screen.getByRole("combobox", { name: "Betreuungskraft" }),
    ).toHaveAttribute("aria-invalid", "true");
  });

  it("retries a failed group handover with the current form", async () => {
    mocks.create
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce(undefined);
    render(<SubstitutionPage />);

    fireEvent.click(screen.getByRole("button", { name: "Gruppe übergeben" }));
    fireEvent.click(screen.getByRole("combobox", { name: "Gruppe" }));
    fireEvent.click(screen.getByRole("option", { name: "Robins Gruppe" }));
    fireEvent.click(screen.getByRole("combobox", { name: "Betreuungskraft" }));
    fireEvent.click(screen.getByRole("option", { name: "Toni Test" }));
    fireEvent.click(screen.getByRole("button", { name: "Zuweisen" }));

    fireEvent.click(await screen.findByRole("button", { name: "Wiederholen" }));

    await waitFor(() =>
      expect(mocks.success).toHaveBeenCalledWith("Die Gruppe wurde übergeben."),
    );
    expect(mocks.create).toHaveBeenCalledTimes(2);
    expect(mocks.create.mock.calls[1]?.slice(0, 2)).toEqual(["12", "34"]);
  });

  it("keeps a failed end of a handover in the confirmation dialog", async () => {
    mocks.end.mockRejectedValueOnce(
      new ApiError("conflict", 409, { code: "substitutions.conflict" }),
    );
    render(<SubstitutionPage />);

    fireEvent.click(screen.getByRole("button", { name: "Beenden" }));
    const buttons = screen.getAllByRole("button", { name: "Beenden" });
    fireEvent.click(buttons[buttons.length - 1]!);

    expect(
      await screen.findByText(
        catalogText("substitutions.conflict", "die Übergabe"),
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Übergabe beenden?")).toBeInTheDocument();
    expect(mocks.success).not.toHaveBeenCalled();
    expect(mocks.toastError).not.toHaveBeenCalled();
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
