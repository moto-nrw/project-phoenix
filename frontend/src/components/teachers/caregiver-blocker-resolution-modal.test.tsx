import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CaregiverBlockerResolutionModal } from "./caregiver-blocker-resolution-modal";

const { mockToastSuccess, mockGetAllAvailableStaff, mockFetch } = vi.hoisted(
  () => ({
    mockToastSuccess: vi.fn(),
    mockGetAllAvailableStaff: vi.fn(),
    mockFetch: vi.fn(),
  }),
);

vi.mock("~/components/ui", () => ({
  FormModal: ({
    isOpen,
    title,
    footer,
    children,
  }: {
    isOpen: boolean;
    title: string;
    footer: React.ReactNode;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="form-modal">
        <h1>{title}</h1>
        <div>{children}</div>
        <div>{footer}</div>
      </div>
    ) : null,
}));

vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message }: { message?: string }) =>
    message ? <div data-testid="alert">{message}</div> : null,
}));

vi.mock("~/components/ui/detail-modal-components", () => ({
  InfoSection: ({
    title,
    children,
  }: {
    title: string;
    children: React.ReactNode;
  }) => (
    <section>
      <h2>{title}</h2>
      {children}
    </section>
  ),
  DetailIcons: {
    group: <span>group</span>,
    bus: <span>bus</span>,
    heart: <span>heart</span>,
    home: <span>home</span>,
  },
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
  }),
}));

vi.mock("~/lib/group-transfer-api", () => ({
  groupTransferService: {
    getAllAvailableStaff: mockGetAllAvailableStaff,
  },
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    error: vi.fn(),
  }),
}));

global.fetch = mockFetch as unknown as typeof fetch;

function createState(overrides: Record<string, unknown> = {}) {
  return {
    accountId: "42",
    email: "teacher@example.com",
    firstName: "Ada",
    lastName: "Lovelace",
    personId: "10",
    staffId: "20",
    teacherId: "30",
    hasAdminRole: true,
    hasUserRole: true,
    hasPerson: true,
    hasStaff: true,
    hasTeacher: true,
    hasCaregiverProfile: true,
    isActiveCaregiver: true,
    disableBlocked: true,
    disableBlockersCount: 1,
    disableBlockers: ["Aktive Gruppenaufsicht"],
    activeSupervisions: [],
    activeSubstitutions: [],
    activitySupervisions: [],
    groupAssignments: [],
    ...overrides,
  };
}

describe("CaregiverBlockerResolutionModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetAllAvailableStaff.mockResolvedValue([
      { id: "20", fullName: "Current Caregiver" },
      { id: "21", fullName: "Other Caregiver" },
    ]);
  });

  it("loads available staff and excludes the current caregiver from replacements", async () => {
    render(
      <CaregiverBlockerResolutionModal
        isOpen={true}
        onClose={vi.fn()}
        onResolved={vi.fn()}
        state={createState({
          activitySupervisions: [
            {
              id: "1",
              activityId: "11",
              activityName: "Theater",
              isPrimary: true,
            },
          ],
        })}
      />,
    );

    await waitFor(() => {
      expect(mockGetAllAvailableStaff).toHaveBeenCalled();
    });

    expect(
      screen.queryByRole("option", { name: "Current Caregiver" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: "Other Caregiver" }),
    ).toBeInTheDocument();
  });

  it("ends active supervisions and removes them from the list", async () => {
    mockFetch.mockResolvedValue({ ok: true });

    render(
      <CaregiverBlockerResolutionModal
        isOpen={true}
        onClose={vi.fn()}
        onResolved={vi.fn()}
        state={createState({
          activeSupervisions: [
            { id: "7", groupName: "Gruppe Blau", startDate: "2026-04-05" },
          ],
        })}
      />,
    );

    fireEvent.click(screen.getByText("Beenden"));

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/active/supervisors/7/end",
        expect.objectContaining({ method: "POST", credentials: "include" }),
      );
    });
    await waitFor(() => {
      expect(screen.queryByText("Gruppe Blau")).not.toBeInTheDocument();
    });
    expect(mockToastSuccess).toHaveBeenCalledWith(
      'Gruppenaufsicht für "Gruppe Blau" beendet.',
    );
  });

  it("removes substitutions and reports success", async () => {
    mockFetch.mockResolvedValue({ ok: true });

    render(
      <CaregiverBlockerResolutionModal
        isOpen={true}
        onClose={vi.fn()}
        onResolved={vi.fn()}
        state={createState({
          activeSubstitutions: [
            {
              id: "9",
              groupName: "Gruppe Rot",
              role: "substitute",
              startDate: "2026-04-05",
              endDate: "2026-04-06",
            },
          ],
        })}
      />,
    );

    fireEvent.click(screen.getByText("Entfernen"));

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/substitutions/9",
        expect.objectContaining({ method: "DELETE", credentials: "include" }),
      );
    });
    expect(mockToastSuccess).toHaveBeenCalledWith(
      'Vertretung für "Gruppe Rot" entfernt.',
    );
  });

  it("transfers activity supervision to a replacement caregiver", async () => {
    mockFetch.mockResolvedValue({ ok: true });

    render(
      <CaregiverBlockerResolutionModal
        isOpen={true}
        onClose={vi.fn()}
        onResolved={vi.fn()}
        state={createState({
          activitySupervisions: [
            {
              id: "8",
              activityId: "12",
              activityName: "Theater",
              isPrimary: true,
            },
          ],
        })}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByRole("option", { name: "Other Caregiver" }),
      ).toBeInTheDocument();
    });

    const select = screen.getByRole("combobox");
    fireEvent.change(select, { target: { value: "21" } });
    fireEvent.click(screen.getByText("Übertragen"));

    await waitFor(() => {
      expect(mockFetch).toHaveBeenNthCalledWith(
        1,
        "/api/activities/12/supervisors",
        expect.objectContaining({
          method: "POST",
          body: JSON.stringify({ staff_id: 21 }),
        }),
      );
    });
    expect(mockFetch).toHaveBeenNthCalledWith(
      2,
      "/api/activities/12/supervisors/8",
      expect.objectContaining({ method: "DELETE" }),
    );
    expect(mockToastSuccess).toHaveBeenCalledWith(
      'Aktivitätsleitung für "Theater" übertragen.',
    );
  });

  it("shows the dedicated only-supervisor error when activity removal is rejected", async () => {
    mockFetch.mockRejectedValueOnce(new Error("only supervisor"));

    render(
      <CaregiverBlockerResolutionModal
        isOpen={true}
        onClose={vi.fn()}
        onResolved={vi.fn()}
        state={createState({
          activitySupervisions: [
            {
              id: "8",
              activityId: "12",
              activityName: "Theater",
              isPrimary: true,
            },
          ],
        })}
      />,
    );

    fireEvent.click(screen.getByText("Entfernen"));

    await waitFor(() => {
      expect(screen.getByTestId("alert")).toHaveTextContent(
        '"Theater": Einzige Leitung',
      );
    });
  });

  it("updates group teachers when reassigning a Stammgruppe", async () => {
    mockFetch.mockResolvedValue({ ok: true });

    render(
      <CaregiverBlockerResolutionModal
        isOpen={true}
        onClose={vi.fn()}
        onResolved={vi.fn()}
        state={createState({
          groupAssignments: [
            {
              id: "10",
              groupId: "14",
              groupName: "Gruppe Gelb",
              teacherId: "30",
            },
          ],
        })}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByRole("option", { name: "Other Caregiver" }),
      ).toBeInTheDocument();
    });

    fireEvent.change(screen.getByRole("combobox"), { target: { value: "21" } });
    fireEvent.click(screen.getByText("Übertragen"));

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/groups/14",
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify({ name: "Gruppe Gelb", teacher_ids: [21] }),
        }),
      );
    });
    expect(mockToastSuccess).toHaveBeenCalledWith(
      'Gruppenleitung für "Gruppe Gelb" übertragen.',
    );
  });

  it("calls onResolved and onClose once everything is cleared", async () => {
    const onResolved = vi.fn();
    const onClose = vi.fn();

    render(
      <CaregiverBlockerResolutionModal
        isOpen={true}
        onClose={onClose}
        onResolved={onResolved}
        state={createState()}
      />,
    );

    fireEvent.click(screen.getByText("Fertig"));

    expect(onResolved).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
