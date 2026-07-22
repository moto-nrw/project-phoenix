import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { StaffPoolSlideOver } from "./staff-pool-slide-over";
import { useSWRAuth } from "~/lib/swr";
import { timetableService } from "~/lib/timetable-api";
import type {
  EnrichedInstance,
  StaffPoolEntry,
  StaffPoolResponse,
} from "~/lib/timetable-types";

const mockToastSuccess = vi.fn();
const mockToastError = vi.fn();
const mockToastWarning = vi.fn();
const mockToastInfo = vi.fn();

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
}));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: {
    getStaffPool: vi.fn(),
    moveStaff: vi.fn(),
  },
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
    warning: mockToastWarning,
    info: mockToastInfo,
  }),
}));

const mockUseSWRAuth = vi.mocked(useSWRAuth);
const mockMoveStaff = vi.mocked(timetableService.moveStaff);

const FUTURE_DATE = "2099-09-21";

function makeInstance(
  overrides: Partial<EnrichedInstance> = {},
): EnrichedInstance {
  return {
    id: "42",
    date: FUTURE_DATE,
    startTime: "12:30",
    endTime: "13:30",
    title: "Mensa",
    status: "planned",
    staff: [],
    ...overrides,
  } as unknown as EnrichedInstance;
}

function entry(overrides: Partial<StaffPoolEntry> = {}): StaffPoolEntry {
  return {
    staffId: "7",
    displayName: "Ina Umzieherin",
    category: "assigned_elsewhere",
    onShift: true,
    coversWindow: true,
    shiftWindows: ["08:00–16:00"],
    assignments: [
      {
        instanceId: "41",
        title: "Schulhof",
        startTime: "12:00",
        endTime: "14:00",
        isSubstitute: false,
      },
    ],
    ...overrides,
  };
}

function mockPool(pool: Partial<StaffPoolResponse>) {
  mockUseSWRAuth.mockReturnValue({
    data: {
      instanceId: "42",
      title: "Mensa",
      date: FUTURE_DATE,
      startTime: "12:30",
      endTime: "13:30",
      dienstplanInUse: true,
      entries: [],
      ...pool,
    },
    isLoading: false,
    error: undefined,
    mutate: vi.fn(),
    isValidating: false,
  } as unknown as ReturnType<typeof useSWRAuth>);
}

function renderPool(
  overrides: Partial<StaffPoolResponse> = {},
  canManage = true,
) {
  mockPool(overrides);
  const onMoved = vi.fn();
  const onClose = vi.fn();
  render(
    <StaffPoolSlideOver
      open
      instance={makeInstance()}
      canManage={canManage}
      onClose={onClose}
      onMoved={onMoved}
    />,
  );
  return { onMoved, onClose };
}

describe("StaffPoolSlideOver", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the categorized sections with counts", () => {
    renderPool({
      entries: [
        entry(),
        entry({
          staffId: "8",
          displayName: "Frido Verfuegbar",
          category: "on_shift_free",
          assignments: [],
        }),
        entry({
          staffId: "9",
          displayName: "Abbi Abwesend",
          category: "absent",
          absenceReason: "krank",
          assignments: [],
        }),
      ],
    });

    expect(screen.getByText("Personalpool")).toBeInTheDocument();
    expect(screen.getByText("Ina Umzieherin")).toBeInTheDocument();
    expect(screen.getByText("Frido Verfuegbar")).toBeInTheDocument();
    expect(screen.getByText("Abbi Abwesend")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Hierher verschieben/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Zuweisen/ }),
    ).toBeInTheDocument();
  });

  it("shows a hint when no Dienstplan exists for the week", () => {
    renderPool({ dienstplanInUse: false });
    expect(screen.getByText(/kein Dienstplan gepflegt/)).toBeInTheDocument();
  });

  it("keeps the pool readable but hides mutations without schedules:manage", () => {
    renderPool(
      {
        entries: [
          entry(),
          entry({
            staffId: "8",
            displayName: "Frido Verfuegbar",
            category: "on_shift_free",
            assignments: [],
          }),
        ],
      },
      false,
    );

    expect(screen.getByText("Ina Umzieherin")).toBeVisible();
    expect(screen.getByText("Frido Verfuegbar")).toBeVisible();
    expect(screen.getByText(/nur Leserechte/)).toBeVisible();
    expect(
      screen.queryByRole("button", { name: /Hierher verschieben/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Zuweisen/ }),
    ).not.toBeInTheDocument();
    expect(mockMoveStaff).not.toHaveBeenCalled();
  });

  it("moves a person after the confirm step and revalidates", async () => {
    mockMoveStaff.mockResolvedValueOnce({
      targetInstanceId: "42",
      sourceInstanceId: "41",
      action: "moved",
      timeConflicts: [],
      coverageWarnings: [],
    });
    const { onMoved } = renderPool({ entries: [entry()] });

    fireEvent.click(
      screen.getByRole("button", { name: /Hierher verschieben/ }),
    );
    expect(screen.getByText("Person verschieben?")).toBeInTheDocument();
    expect(
      screen.getByText(/kann dadurch unterbesetzt werden/),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Verschieben" }));

    await waitFor(() => {
      expect(mockMoveStaff).toHaveBeenCalledWith("42", {
        staffId: "7",
        sourceInstanceId: "41",
      });
    });
    await waitFor(() => expect(onMoved).toHaveBeenCalled());
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Ina Umzieherin wurde verschoben.",
    );
  });

  it("assigns a free person without a source block", async () => {
    mockMoveStaff.mockResolvedValueOnce({
      targetInstanceId: "42",
      action: "assigned",
      timeConflicts: [],
      coverageWarnings: [],
    });
    renderPool({
      entries: [
        entry({
          staffId: "8",
          displayName: "Frido Verfuegbar",
          category: "on_shift_free",
          assignments: [],
        }),
      ],
    });

    fireEvent.click(screen.getByRole("button", { name: /Zuweisen/ }));
    // Der Bestätigungsdialog trägt denselben Button-Text wie die Pool-Zeile;
    // das Kit-Modal portalt ans Body-Ende, also ist der letzte Treffer der
    // Bestätigen-Button.
    const confirmButtons = screen.getAllByRole("button", { name: "Zuweisen" });
    fireEvent.click(confirmButtons[confirmButtons.length - 1]!);

    await waitFor(() => {
      expect(mockMoveStaff).toHaveBeenCalledWith("42", {
        staffId: "8",
        sourceInstanceId: undefined,
      });
    });
  });

  it("surfaces advisory warnings as toasts after a move", async () => {
    mockMoveStaff.mockResolvedValueOnce({
      targetInstanceId: "42",
      sourceInstanceId: "41",
      action: "moved",
      timeConflicts: [
        {
          instanceId: "44",
          title: "AG",
          date: FUTURE_DATE,
          startTime: "13:00",
          endTime: "14:00",
        },
      ],
      coverageWarnings: [
        {
          staffId: "7",
          staffName: "Ina Umzieherin",
          date: FUTURE_DATE,
          startTime: "12:30",
          endTime: "13:30",
          uncoveredStartTime: "13:00",
          uncoveredEndTime: "13:30",
          message: "Keine Schicht deckt 13:00–13:30 ab",
        },
      ],
    });
    renderPool({ entries: [entry()] });

    fireEvent.click(
      screen.getByRole("button", { name: /Hierher verschieben/ }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Verschieben" }));

    await waitFor(() =>
      expect(mockToastWarning).toHaveBeenCalledWith(
        "Keine Schicht deckt 13:00–13:30 ab",
      ),
    );
    expect(mockToastWarning).toHaveBeenCalledWith(
      expect.stringContaining("zeitgleich auf „AG“"),
    );
  });

  it("shows a German error toast when the move fails", async () => {
    mockMoveStaff.mockRejectedValueOnce(new Error("HTTP 409: conflict"));
    renderPool({ entries: [entry()] });

    fireEvent.click(
      screen.getByRole("button", { name: /Hierher verschieben/ }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Verschieben" }));

    await waitFor(() => expect(mockToastError).toHaveBeenCalled());
  });
});
