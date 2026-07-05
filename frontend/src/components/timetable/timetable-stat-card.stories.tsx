import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { CalendarCheck } from "lucide-react";

import { TimetableStatCard } from "./timetable-stat-card";

const meta = {
  title: "components/timetable/TimetableStatCard",
  component: TimetableStatCard,
  args: {
    icon: <CalendarCheck className="h-3.5 w-3.5" />,
    label: "Stunden",
    value: "24",
    tone: "neutral",
  },
} satisfies Meta<typeof TimetableStatCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Small: Story = {
  args: {
    size: "sm",
    tone: "success",
    label: "Geplant",
    value: "18",
  },
};

export const Large: Story = {
  args: {
    size: "lg",
    tone: "warning",
    label: "Auslastung",
    value: "82%",
    sublabel: "diese Woche",
  },
};

export const Danger: Story = {
  args: {
    size: "lg",
    tone: "danger",
    label: "Konflikte",
    value: "3",
    sublabel: "ungelöst",
  },
};
