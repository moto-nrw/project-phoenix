import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import type { EnrichedInstance } from "~/lib/timetable-types";
import { YearPlannerGrid } from "./year-planner-grid";

const months = [
  new Date(2026, 0, 1),
  new Date(2026, 1, 1),
  new Date(2026, 2, 1),
];

const instances: EnrichedInstance[] = [
  {
    id: "1",
    date: "2026-01-15",
    conflictWarnings: [],
  } as unknown as EnrichedInstance,
  {
    id: "2",
    date: "2026-01-20",
    conflictWarnings: ["overlap"],
  } as unknown as EnrichedInstance,
];

const meta: Meta<typeof YearPlannerGrid> = {
  title: "components/timetable/YearPlannerGrid",
  component: YearPlannerGrid,
  parameters: {
    layout: "padded",
  },
};

export default meta;

type Story = StoryObj<typeof YearPlannerGrid>;

export const Empty: Story = {
  args: {
    months,
    instances: [],
    onMonthClick: () => undefined,
    onDayClick: () => undefined,
  },
};

export const WithInstancesAndConflicts: Story = {
  args: {
    months,
    instances,
    todayISO: "2026-01-15",
    onMonthClick: () => undefined,
    onDayClick: () => undefined,
  },
};
