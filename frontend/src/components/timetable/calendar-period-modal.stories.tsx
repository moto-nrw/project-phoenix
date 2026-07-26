import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { CalendarPeriodModal } from "./calendar-period-modal";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";

const meta: Meta<typeof CalendarPeriodModal> = {
  title: "timetable/CalendarPeriodModal",
  component: CalendarPeriodModal,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof CalendarPeriodModal>;

const exampleInitial: CalendarPeriod = {
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
};

export const Create: Story = {
  args: {
    isOpen: true,
    onClose: () => undefined,
    onSaved: () => undefined,
  },
};

export const Edit: Story = {
  args: {
    isOpen: true,
    onClose: () => undefined,
    onSaved: () => undefined,
    onDeleted: () => undefined,
    initial: exampleInitial,
  },
};
