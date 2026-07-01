import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { activeService } from "~/lib/active-service";
import type { ActiveGroup } from "~/lib/active-helpers";
import type { Student } from "~/lib/api";
import {
  useSWRAuth,
  useTenantMutate,
  useTenantMutateMatching,
} from "~/lib/swr";
import { TransitStudentsSection } from "./transit-students-section";

const { mockPush, mockSearchParamsToString } = vi.hoisted(() => ({
  mockPush: vi.fn(),
  mockSearchParamsToString: vi.fn(() => "room=__transit__"),
}));
const { mockUseSession } = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => ({
    toString: mockSearchParamsToString,
  }),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => mockUseSession(),
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({
    push: mockPush,
  }),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  useTenantMutate: vi.fn(),
  useTenantMutateMatching: vi.fn(),
}));

vi.mock("~/lib/active-service", () => ({
  activeService: {
    getActiveGroups: vi.fn(),
    getStaffActiveSupervisions: vi.fn(),
    assignTransitStudents: vi.fn(),
  },
}));

vi.mock("~/lib/api", () => ({
  studentService: {
    getStudents: vi.fn(),
  },
}));

const mockToastSuccess = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
  }),
}));

const mockStudents: Student[] = [
  {
    id: "11",
    name: "Mila Sommer",
    first_name: "Mila",
    second_name: "Sommer",
    school_class: "2a",
    group_name: "Gruppe A",
    current_location: "transit",
  },
  {
    id: "12",
    name: "Noah Winter",
    first_name: "Noah",
    second_name: "Winter",
    school_class: "3b",
    group_name: "Gruppe B",
    current_location: "transit",
  },
];

const mockGroups: ActiveGroup[] = [
  {
    id: "101",
    groupId: "201",
    roomId: "301",
    startTime: new Date("2026-05-14T08:00:00.000Z"),
    isActive: true,
    room: { id: 301, name: "Aula" },
    actualGroup: { id: 201, name: "Gruppe A" },
    createdAt: new Date("2026-05-14T08:00:00.000Z"),
    updatedAt: new Date("2026-05-14T08:00:00.000Z"),
  },
  {
    id: "102",
    groupId: "202",
    roomId: "302",
    startTime: new Date("2026-05-14T08:00:00.000Z"),
    isActive: false,
    room: { id: 302, name: "Musikraum" },
    actualGroup: { id: 202, name: "Gruppe B" },
    createdAt: new Date("2026-05-14T08:00:00.000Z"),
    updatedAt: new Date("2026-05-14T08:00:00.000Z"),
  },
];

const mockSupervisions = [
  {
    id: "1",
    staffId: "20",
    activeGroupId: "101",
    startTime: new Date("2026-05-14T08:00:00.000Z"),
    isActive: true,
    createdAt: new Date("2026-05-14T08:00:00.000Z"),
    updatedAt: new Date("2026-05-14T08:00:00.000Z"),
  },
];

const mutateStudents = vi.fn();
const mutateKey = vi.fn();
const mutateMatching = vi.fn();

function mockTransitData({
  students = mockStudents,
  activeGroups = mockGroups,
  studentsError = null,
  groupsError = null,
  staffError = null,
  supervisionsError = null,
  studentsLoading = false,
  groupsLoading = false,
  staffLoading = false,
  supervisionsLoading = false,
  currentStaff = { id: "20" },
  activeSupervisions = mockSupervisions,
}: {
  students?: Student[];
  activeGroups?: ActiveGroup[];
  activeSupervisions?: typeof mockSupervisions;
  currentStaff?: { id: string } | undefined;
  studentsError?: Error | null;
  groupsError?: Error | null;
  staffError?: Error | null;
  supervisionsError?: Error | null;
  studentsLoading?: boolean;
  groupsLoading?: boolean;
  staffLoading?: boolean;
  supervisionsLoading?: boolean;
} = {}) {
  vi.mocked(useSWRAuth).mockImplementation((key: unknown) => {
    if (key === null) {
      return { data: undefined, error: null, isLoading: false } as never;
    }

    if (key === "transit-students") {
      return {
        data: { students, pagination: { total_records: students.length } },
        error: studentsError,
        isLoading: studentsLoading,
        mutate: mutateStudents,
      } as never;
    }

    if (key === "active-groups-for-transit") {
      return {
        data: activeGroups,
        error: groupsError,
        isLoading: groupsLoading,
      } as never;
    }

    if (key === "transit-target-current-staff") {
      return {
        data: currentStaff,
        error: staffError,
        isLoading: staffLoading,
      } as never;
    }

    if (key === "transit-target-supervisions-20") {
      return {
        data: activeSupervisions,
        error: supervisionsError,
        isLoading: supervisionsLoading,
      } as never;
    }

    throw new Error(`Unexpected SWR key: ${String(key)}`);
  });
}

describe("TransitStudentsSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSearchParamsToString.mockReturnValue("room=__transit__");
    mockUseSession.mockReturnValue({
      data: {
        user: {
          token: "staff-token",
          roles: ["user"],
          permissions: ["visits:update"],
          isAdmin: false,
        },
      },
      status: "authenticated",
      update: vi.fn(),
    });
    mutateStudents.mockResolvedValue(undefined);
    mutateKey.mockResolvedValue(undefined);
    mutateMatching.mockResolvedValue(undefined);
    mockToastSuccess.mockReset();
    vi.mocked(useTenantMutate).mockReturnValue(mutateKey as never);
    vi.mocked(useTenantMutateMatching).mockReturnValue(mutateMatching as never);
    vi.mocked(activeService.assignTransitStudents).mockResolvedValue({
      assigned: [11],
      skipped: [],
      active_group_id: 101,
      room_id: 301,
    });
    mockTransitData();
  });

  it("renders transit students and active target rooms", () => {
    render(<TransitStudentsSection />);

    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getByText("Kinder ohne Raum")).toBeInTheDocument();
    expect(screen.getByText("Mila Sommer")).toBeInTheDocument();
    expect(screen.getByText("Noah Winter")).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Aula · Gruppe A" })).toHaveValue(
      "101",
    );
    expect(
      screen.queryByRole("option", { name: "Musikraum · Gruppe B" }),
    ).not.toBeInTheDocument();
  });

  it("filters active target rooms to the current staff member's supervisions", () => {
    mockTransitData({
      activeGroups: [
        mockGroups[0]!,
        {
          id: "103",
          groupId: "203",
          roomId: "303",
          startTime: new Date("2026-05-14T08:00:00.000Z"),
          isActive: true,
          room: { id: 303, name: "Werkraum" },
          actualGroup: { id: 203, name: "Gruppe C" },
          createdAt: new Date("2026-05-14T08:00:00.000Z"),
          updatedAt: new Date("2026-05-14T08:00:00.000Z"),
        },
      ],
      activeSupervisions: [
        {
          ...mockSupervisions[0]!,
          activeGroupId: "103",
        },
      ],
    });

    render(<TransitStudentsSection />);

    expect(
      screen.queryByRole("option", { name: "Aula · Gruppe A" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: "Werkraum · Gruppe C" }),
    ).toHaveValue("103");
  });

  it("keeps all active target rooms visible for admins", () => {
    mockUseSession.mockReturnValue({
      data: {
        user: {
          token: "admin-token",
          roles: ["admin"],
          permissions: ["admin:*"],
          isAdmin: true,
        },
      },
      status: "authenticated",
      update: vi.fn(),
    });
    mockTransitData({
      activeGroups: [
        mockGroups[0]!,
        {
          id: "103",
          groupId: "203",
          roomId: "303",
          startTime: new Date("2026-05-14T08:00:00.000Z"),
          isActive: true,
          room: { id: 303, name: "Werkraum" },
          actualGroup: { id: 203, name: "Gruppe C" },
          createdAt: new Date("2026-05-14T08:00:00.000Z"),
          updatedAt: new Date("2026-05-14T08:00:00.000Z"),
        },
      ],
      activeSupervisions: [],
    });

    render(<TransitStudentsSection />);

    expect(screen.getByRole("option", { name: "Aula · Gruppe A" })).toHaveValue(
      "101",
    );
    expect(
      screen.getByRole("option", { name: "Werkraum · Gruppe C" }),
    ).toHaveValue("103");
  });

  it("assigns selected transit students to the selected active room", async () => {
    render(<TransitStudentsSection />);

    fireEvent.click(screen.getByLabelText("Mila Sommer auswählen"));
    fireEvent.change(screen.getByLabelText("Zielraum"), {
      target: { value: "101" },
    });
    fireEvent.click(screen.getByRole("button", { name: "In Raum setzen" }));

    await waitFor(() => {
      expect(activeService.assignTransitStudents).toHaveBeenCalledWith(
        ["11"],
        "101",
      );
    });
    expect(mockToastSuccess).toHaveBeenCalledWith("1 Kind zugewiesen.");
    expect(mutateStudents).toHaveBeenCalledTimes(1);
    expect(mutateKey).toHaveBeenCalledWith("rooms-list");
    expect(mutateMatching).toHaveBeenCalledTimes(1);
  });

  it("opens a transit child profile from the dedicated profile button", () => {
    render(<TransitStudentsSection />);

    fireEvent.click(
      screen.getByRole("button", { name: "Mila Sommer Profil öffnen" }),
    );

    expect(mockPush).toHaveBeenCalledWith(
      `/students/11?from=${encodeURIComponent("/rooms?room=__transit__")}`,
    );
  });

  it("uses an explicit referrer when embedded outside rooms", () => {
    render(<TransitStudentsSection fromReferrer="/active-supervisions" />);

    fireEvent.click(
      screen.getByRole("button", { name: "Mila Sommer Profil öffnen" }),
    );

    expect(mockPush).toHaveBeenCalledWith(
      `/students/11?from=${encodeURIComponent("/active-supervisions")}`,
    );
  });

  it("reports active selection state to the drawer wrapper", async () => {
    const onSelectionActiveChange = vi.fn();

    render(
      <TransitStudentsSection
        onSelectionActiveChange={onSelectionActiveChange}
      />,
    );

    fireEvent.click(screen.getByLabelText("Mila Sommer auswählen"));

    await waitFor(() => {
      expect(onSelectionActiveChange).toHaveBeenLastCalledWith(true);
    });
  });

  it("shows empty and error states", () => {
    mockTransitData({
      students: [],
      activeGroups: [],
      studentsError: new Error("failed"),
    });

    render(<TransitStudentsSection />);

    expect(
      screen.getByText("Die Unterwegs-Daten konnten nicht geladen werden."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Aktuell keine Kinder unterwegs."),
    ).toBeInTheDocument();
    expect(screen.getByRole("combobox")).toBeDisabled();
    expect(
      screen.getByRole("option", { name: "Keine aktiven Räume" }),
    ).toBeInTheDocument();
  });
});
