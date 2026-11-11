// #6593: Nach einer Absage schließt sich das Vertretungs-Panel selbst, nur
// solange es denselben Block zeigt. Die View schließt aus der Antwort heraus
// nichts. Echte View mit echtem SubstitutionSlideOver und vaul-Mock.
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { EnrichedInstance } from "~/lib/timetable-types";
import { setTestClock } from "~/test/clock";
import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

const {
  mockSearch,
  mockToastSuccess,
  mockTenantMutate,
  mockUseSWRAuth,
  mockApplyDeviations,
} = vi.hoisted(() => ({
  mockSearch: { value: "" },
  mockToastSuccess: vi.fn(),
  mockTenantMutate: vi.fn(),
  mockUseSWRAuth: vi.fn(),
  mockApplyDeviations: vi.fn(),
}));

vi.mock("vaul", () => import("~/test/mocks/vaul"));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(mockSearch.value),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    status: "authenticated",
    data: { user: { permissions: ["schedules:manage"] } },
  }),
}));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({
    success: mockToastSuccess,
    error: vi.fn(),
    warning: vi.fn(),
  }),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: mockUseSWRAuth,
  useTenantMutate: () => mockTenantMutate,
  useTenantMutateMatching: () => () => Promise.resolve(),
}));

vi.mock("~/lib/hooks/use-timetable-day-hours", () => ({
  useTimetableDayHours: () => ({ dayStartHour: 9, dayEndHour: 17 }),
}));

vi.mock("~/lib/tenant-context", () => ({
  useWeekendFollowsFriday: () => false,
  useTimetableEnabled: () => true,
}));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: {
    getWeek: vi.fn(),
    getGaps: vi.fn(),
    getDeviationHistory: vi.fn(),
    // Keine gebuchten Kinder: die Absage braucht keinen Elternhinweis.
    getGuardianNoticeReach: () =>
      Promise.resolve({ childCount: 0, familyCount: 0 }),
    applyDeviations: mockApplyDeviations,
  },
}));

vi.mock("~/lib/substitution-api", () => ({
  substitutionService: {
    fetchScheduleOverview: vi.fn(),
    applyScheduleSubstitution: vi.fn(),
    applyBulkSubstitution: vi.fn(),
  },
}));

vi.mock("~/lib/shift-api", () => ({
  staffShiftService: { getOverview: vi.fn() },
}));

vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => `/acme${path}`,
}));

// Liste und Raster sind hier Kulisse; der Block kommt über die URL.
vi.mock(
  "~/components/timetable/vertretung-day-list",
  async (importOriginal) => ({
    ...(await importOriginal<
      typeof import("~/components/timetable/vertretung-day-list")
    >()),
    VertretungDayList: () => <div data-testid="day-list" />,
  }),
);

vi.mock("~/components/timetable/weekly-calendar-grid", () => ({
  WeeklyCalendarGrid: () => <div data-testid="calendar-grid" />,
}));

import { VertretungView } from "./vertretung-view";

function makeInstance(overrides: Partial<EnrichedInstance>): EnrichedInstance {
  return {
    id: "42",
    date: "2026-07-15",
    startTime: "14:00",
    endTime: "15:00",
    title: "Mensa",
    status: "planned",
    isSpontaneous: false,
    isLive: false,
    activityType: "care",
    roomId: "3",
    roomName: "Mensa",
    staff: [],
    studentIds: [],
    students: [],
    staffCount: 0,
    absentStaffCount: 0,
    expectedStudentsCount: 0,
    notScheduledStudentsCount: 0,
    presentStudentsCount: 0,
    requiredStaffCount: 0,
    assignedStaffCount: 0,
    conflictWarnings: [],
    ...overrides,
  };
}

const WEEK_INSTANCES: EnrichedInstance[] = [
  makeInstance({ id: "42", date: "2026-07-15", title: "Mensa" }),
  makeInstance({ id: "43", date: "2026-07-16", title: "Fussball" }),
];

function openBlock(search: string) {
  mockSearch.value = search;
  window.history.replaceState(null, "", `/acme/vertretung?${search}`);
}

function urlParams() {
  return new URLSearchParams(window.location.search);
}

/** Opens block 43, chooses „Block absagen“ and saves; the answer waits. */
async function startCancel() {
  const answer = deferred<{
    instanceId: string;
    cancelled: boolean;
    understaffedAck: boolean;
    affectedInstances: [];
    warnings: [];
  }>();
  mockApplyDeviations.mockReturnValue(answer.promise);
  openBlock("d=2026-07-16&block=43");
  const view = render(<VertretungView />);

  fireEvent.click(screen.getByRole("radio", { name: /Block absagen/ }));
  const save = screen.getByRole("button", { name: "Speichern" });
  // Die Reichweite des Elternhinweises lädt erst; bis dahin ist Speichern zu.
  await waitFor(() => expect(save).toBeEnabled());
  fireEvent.click(save);
  expect(mockApplyDeviations).toHaveBeenCalledWith(
    "43",
    expect.objectContaining({ cancel: true }),
  );
  expectSlideOverSaveLocked("Fussball", ["Abbrechen", "Speichere …"]);

  const resolveCancelled = () =>
    answer.resolve({
      instanceId: "43",
      cancelled: true,
      understaffedAck: false,
      affectedInstances: [],
      warnings: [],
    });
  return { ...view, resolveCancelled };
}

describe("VertretungView: Absage aus dem Panel (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Mittwoch 2026-07-15, Berlin. Woche Mo 13. – So 19.07.
    setTestClock("2026-07-15T12:00:00+02:00");
    mockTenantMutate.mockResolvedValue(undefined);
    mockUseSWRAuth.mockImplementation((key: string | null) => {
      if (key?.startsWith("vertretung-week")) {
        return {
          data: {
            from: "2026-07-13",
            to: "2026-07-19",
            instances: WEEK_INSTANCES,
          },
          isLoading: false,
        };
      }
      if (key?.startsWith("vertretung-gaps")) {
        return {
          data: {
            from: "2026-07-15",
            to: "2026-07-19",
            gaps: [],
            acknowledged: [],
          },
          isLoading: false,
        };
      }
      if (key?.startsWith("vertretung-staff-list-")) {
        return { data: { appointments: [], staff: [] } };
      }
      return {};
    });
  });

  it("schließt das Panel nach der Absage", async () => {
    const { resolveCancelled } = await startCancel();

    await act(async () => resolveCancelled());

    expect(mockToastSuccess).toHaveBeenCalledWith("Der Block ist abgesagt.");
    expect([...urlParams().keys()]).toEqual(["d"]);
  });

  it("schließt nach einer späten Absage nicht das Panel eines anderen Blocks", async () => {
    const { resolveCancelled, rerender } = await startCancel();

    openBlock("d=2026-07-16&block=42");
    rerender(<VertretungView />);
    expectSlideOverUnlocked("Mensa");

    await act(async () => resolveCancelled());

    expect(mockToastSuccess).toHaveBeenCalledWith("Der Block ist abgesagt.");
    expect(urlParams().get("block")).toBe("42");
    expectSlideOverUnlocked("Mensa");
  });
});
