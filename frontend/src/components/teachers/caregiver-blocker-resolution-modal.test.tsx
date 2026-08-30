import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CaregiverBlockerResolutionModal } from "./caregiver-blocker-resolution-modal";

const { mockToastSuccess, mockGetAllAvailableStaff, mockFetch } = vi.hoisted(
  () => ({
    mockToastSuccess: vi.fn(),
    mockGetAllAvailableStaff: vi.fn(),
    mockFetch: vi.fn(),
  }),
);

vi.mock("~/components/ui/form-modal", async () => {
  const { createElement } = await import("react");

  return {
    FormModal: ({
      isOpen,
      title,
      footer,
      children,
    }: {
      isOpen: boolean;
      title: string;
      footer: ReactNode;
      children: ReactNode;
    }) =>
      isOpen
        ? createElement(
            "div",
            { "data-testid": "form-modal" },
            createElement("h1", null, title),
            createElement("div", null, children),
            createElement("div", null, footer),
          )
        : null,
  };
});

vi.mock("~/components/ui/alert", async () => {
  const { createElement } = await import("react");

  return {
    Alert: ({ message }: { message?: string }) =>
      message
        ? createElement("div", { "data-testid": "alert" }, message)
        : null,
  };
});

vi.mock("~/components/ui/detail-modal-components", async () => {
  const { createElement } = await import("react");

  return {
    InfoSection: ({
      title,
      children,
    }: {
      title: string;
      children: ReactNode;
    }) =>
      createElement(
        "section",
        null,
        createElement("h2", null, title),
        children,
      ),
    DetailIcons: {
      group: createElement("span", null, "group"),
      bus: createElement("span", null, "bus"),
      heart: createElement("span", null, "heart"),
      home: createElement("span", null, "home"),
    },
  };
});

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
  }),
}));

vi.mock("~/lib/group-transfer-api", () => ({
  groupTransferService: {
    getStaffByRole: mockGetAllAvailableStaff,
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
      { id: "20", fullName: "Current Caregiver", teacherId: "30" },
      { id: "21", fullName: "Other Caregiver", teacherId: "32" },
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

    fireEvent.click(screen.getByRole("combobox"));

    expect(
      screen.queryByRole("option", { name: "Current Caregiver" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: "Other Caregiver" }),
    ).toBeInTheDocument();
  });

  it("shows when replacement staff cannot be loaded", async () => {
    mockGetAllAvailableStaff.mockRejectedValueOnce(new Error("invalid data"));

    render(
      <CaregiverBlockerResolutionModal
        isOpen={true}
        onClose={vi.fn()}
        onResolved={vi.fn()}
        state={createState()}
      />,
    );

    expect(
      await screen.findByText(
        "Ersatzkräfte konnten nicht geladen werden. Bitte versuchen Sie es noch einmal.",
      ),
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

  it("ends group handovers and reports success", async () => {
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
        "/api/substitutions/end",
        expect.objectContaining({
          method: "POST",
          credentials: "include",
          body: JSON.stringify({ type: "group_handover", id: 9 }),
        }),
      );
    });
    expect(mockToastSuccess).toHaveBeenCalledWith(
      'Gruppenübergabe für "Gruppe Rot" beendet.',
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
      expect(screen.getByRole("combobox")).not.toBeDisabled();
    });

    fireEvent.click(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: "Other Caregiver" }));
    fireEvent.click(screen.getByText("Übertragen"));

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/activities/12/supervisors/8?replacement_staff_id=21",
        expect.objectContaining({ method: "DELETE" }),
      );
    });
    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockToastSuccess).toHaveBeenCalledWith(
      'Aktivitätsleitung für "Theater" übertragen.',
    );
  });

  it("shows the dedicated only-supervisor error when activity removal is rejected", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 409,
      json: async () => ({
        error: "cannot remove the only supervisor",
        code: "ONLY_SUPERVISOR_REPLACEMENT_REQUIRED",
      }),
    });

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

  it("falls back to the generic activity error message for unstructured failures", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      json: async () => {
        throw new Error("invalid json");
      },
    });

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
        "Aktivitätsleitung konnte nicht entfernt werden (500)",
      );
    });
  });

  it("preserves other group leaders when removing a Stammgruppe assignment", async () => {
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
              teacherIds: ["30", "31"],
            },
          ],
        })}
      />,
    );

    fireEvent.click(screen.getByText("Entfernen"));

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/groups/14",
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify({ name: "Gruppe Gelb", teacher_ids: [31] }),
        }),
      );
    });
    expect(mockToastSuccess).toHaveBeenCalledWith(
      'Gruppenleitung für "Gruppe Gelb" entfernt.',
    );
  });

  it("preserves other group leaders when reassigning a Stammgruppe", async () => {
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
              teacherIds: ["30", "31"],
            },
          ],
        })}
      />,
    );

    await waitFor(() => {
      expect(screen.getByRole("combobox")).not.toBeDisabled();
    });

    fireEvent.click(screen.getByRole("combobox"));
    fireEvent.click(screen.getByRole("option", { name: "Other Caregiver" }));
    fireEvent.click(screen.getByText("Übertragen"));

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/groups/14",
        expect.objectContaining({
          method: "PUT",
          body: JSON.stringify({ name: "Gruppe Gelb", teacher_ids: [31, 32] }),
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
