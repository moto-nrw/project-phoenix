import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import type { EnrichedInstance } from "~/lib/timetable-types";
import { WeeklyCalendarGrid } from "./weekly-calendar-grid";

const weekDays = [
  new Date(2026, 0, 5),
  new Date(2026, 0, 6),
  new Date(2026, 0, 7),
  new Date(2026, 0, 8),
  new Date(2026, 0, 9),
  new Date(2026, 0, 10),
  new Date(2026, 0, 11),
];

function makeInstance(
  overrides: Partial<EnrichedInstance> & Pick<EnrichedInstance, "id" | "date">,
): EnrichedInstance {
  return {
    startTime: "10:00",
    endTime: "11:00",
    title: "AG Schach",
    status: "planned",
    isSpontaneous: false,
    isLive: false,
    activityType: "activity",
    roomId: "room-1",
    roomName: "Raum 1",
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

const instances: EnrichedInstance[] = [
  makeInstance({
    id: "1",
    date: "2026-01-05",
    startTime: "09:00",
    endTime: "10:30",
    title: "Hausaufgabenbetreuung",
  }),
  makeInstance({
    id: "2",
    date: "2026-01-05",
    startTime: "10:00",
    endTime: "11:00",
    title: "AG Schach",
  }),
  makeInstance({
    id: "3",
    date: "2026-01-07",
    startTime: "13:00",
    endTime: "14:00",
    title: "Kochkurs",
    status: "cancelled",
  }),
];

const meta: Meta<typeof WeeklyCalendarGrid> = {
  title: "components/timetable/WeeklyCalendarGrid",
  component: WeeklyCalendarGrid,
  parameters: {
    layout: "padded",
  },
};

export default meta;

type Story = StoryObj<typeof WeeklyCalendarGrid>;

export const Empty: Story = {
  args: {
    weekDays,
    instances: [],
    selectedId: null,
    onInstanceClick: () => undefined,
    todayISO: "2026-01-05",
    dayStartHour: 9,
    dayEndHour: 17,
    hourHeightPx: 60,
  },
};

export const WithInstances: Story = {
  args: {
    weekDays,
    instances,
    selectedId: "2",
    onInstanceClick: () => undefined,
    todayISO: "2026-01-05",
    dayStartHour: 9,
    dayEndHour: 17,
    hourHeightPx: 60,
    onSlotClick: () => undefined,
  },
};

export const EmptyStateOverlay: Story = {
  args: {
    weekDays,
    instances: [],
    selectedId: null,
    onInstanceClick: () => undefined,
    todayISO: "2026-01-05",
    dayStartHour: 9,
    dayEndHour: 17,
    hourHeightPx: 60,
    emptyState: {
      title: "Noch keine Aktivitäten",
      description: "Lege für diese Woche eine erste Aktivität an.",
    },
  },
};
