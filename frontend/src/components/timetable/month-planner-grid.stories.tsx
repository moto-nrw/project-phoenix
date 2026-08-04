import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import type { EnrichedInstance } from "~/lib/timetable-types";
import { MonthPlannerGrid } from "./month-planner-grid";

const meta: Meta<typeof MonthPlannerGrid> = {
  title: "timetable/MonthPlannerGrid",
  component: MonthPlannerGrid,
};

export default meta;
type Story = StoryObj<typeof MonthPlannerGrid>;

function buildMonthDays(year: number, month: number): Date[] {
  const firstOfMonth = new Date(year, month, 1);
  const startOffset = (firstOfMonth.getDay() + 6) % 7; // Monday-first grid
  const gridStart = new Date(year, month, 1 - startOffset);

  return Array.from({ length: 42 }, (_, index) => {
    const day = new Date(gridStart);
    day.setDate(gridStart.getDate() + index);
    return day;
  });
}

function makeInstance(
  overrides: Partial<EnrichedInstance> & Pick<EnrichedInstance, "id" | "date">,
): EnrichedInstance {
  return {
    startTime: "14:00",
    endTime: "15:00",
    title: "Hausaufgabenbetreuung",
    status: "planned",
    isSpontaneous: false,
    isLive: false,
    activityType: "care",
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

const monthDate = new Date(2026, 5, 1);
const days = buildMonthDays(2026, 5);
const todayISO = "2026-06-15";

export const Empty: Story = {
  args: {
    days,
    monthDate,
    instances: [],
    todayISO,
    onDayClick: () => undefined,
  },
};

export const WithInstances: Story = {
  args: {
    days,
    monthDate,
    instances: [
      makeInstance({
        id: "instance-1",
        date: "2026-06-15",
        title: "Kunst AG",
        activityType: "activity",
        status: "active",
      }),
      makeInstance({
        id: "instance-2",
        date: "2026-06-15",
        title: "Ausflug Zoo",
        activityType: "external",
        isSpontaneous: true,
      }),
      makeInstance({
        id: "instance-3",
        date: "2026-06-16",
        title: "Musikunterricht",
        status: "cancelled",
        conflictWarnings: [
          {
            kind: "student",
            resourceId: "21",
            message: "Kind ist von 14:30–15:00 auch bei „Lernzeit“ eingeplant.",
            canOverride: true,
          },
        ],
      }),
      makeInstance({
        id: "instance-4",
        date: "2026-06-20",
        title: "Sportangebot",
      }),
      makeInstance({
        id: "instance-5",
        date: "2026-06-20",
        title: "Theater AG",
      }),
      makeInstance({
        id: "instance-6",
        date: "2026-06-20",
        title: "Schach Club",
      }),
      makeInstance({
        id: "instance-7",
        date: "2026-06-20",
        title: "Garten AG",
      }),
      makeInstance({
        id: "instance-8",
        date: "2026-06-20",
        title: "Kochkurs",
      }),
    ],
    todayISO,
    onDayClick: () => undefined,
  },
};
