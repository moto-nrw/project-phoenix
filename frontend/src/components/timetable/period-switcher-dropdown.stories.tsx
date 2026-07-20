import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";

import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import { PeriodSwitcherDropdown } from "./period-switcher-dropdown";

function makePeriod(overrides: Partial<CalendarPeriod>): CalendarPeriod {
  return {
    id: "1",
    tenantId: "1",
    name: "Schuljahr 2025/2026",
    periodType: "school_year",
    startDate: "2025-08-01",
    endDate: "2026-07-31",
    weekCycleLength: 1,
    weekCycleAnchor: null,
    isActive: true,
    createdAt: "2025-08-01T00:00:00Z",
    updatedAt: "2025-08-01T00:00:00Z",
    ...overrides,
  };
}

const weekDays = [
  new Date("2026-06-01T00:00:00Z"),
  new Date("2026-06-02T00:00:00Z"),
  new Date("2026-06-03T00:00:00Z"),
  new Date("2026-06-04T00:00:00Z"),
  new Date("2026-06-05T00:00:00Z"),
];

const periods: CalendarPeriod[] = [
  makePeriod({
    id: "1",
    name: "Schuljahr 2025/2026",
    startDate: "2025-08-01",
    endDate: "2026-07-31",
  }),
  makePeriod({
    id: "2",
    name: "Sommerferien 2026",
    periodType: "holiday",
    startDate: "2026-07-01",
    endDate: "2026-08-14",
    isActive: true,
  }),
  makePeriod({
    id: "3",
    name: "Schuljahr 2024/2025",
    startDate: "2024-08-01",
    endDate: "2025-07-31",
    isActive: false,
  }),
];

const meta = {
  title: "components/timetable/PeriodSwitcherDropdown",
  component: PeriodSwitcherDropdown,
  args: {
    periods,
    weekDays,
    onCreate: fn(),
    onEdit: fn(),
    onSelect: fn(),
  },
} satisfies Meta<typeof PeriodSwitcherDropdown>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Loading: Story = {
  args: {
    isLoading: true,
  },
};

export const Empty: Story = {
  args: {
    periods: [],
  },
};

export const WithSelectedPeriod: Story = {
  args: {
    selectedPeriodId: "1",
  },
};

export const MonthView: Story = {
  args: {
    view: "month",
  },
};
