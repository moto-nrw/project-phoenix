import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { KpiCard, KpiCards, ViewToggle } from "./staff-time-views";
import type { ViewMode } from "./staff-time-views";
import type { StaffMetrics } from "~/lib/staff-metrics-helpers";

const meta = {
  title: "components/staff/StaffTimeViews",
} satisfies Meta;

export default meta;
type Story = StoryObj<typeof meta>;

export const Card: Story = {
  render: () => (
    <KpiCard
      label="Diese Woche"
      primary="32h 15min"
      secondary="von 38h 30min Soll"
      progressPct={72}
      color="green"
    />
  ),
};

export const CardOvertime: Story = {
  render: () => (
    <KpiCard
      label="Überstunden Monat"
      primary="+4h 20min"
      secondary="Überstunden"
      color="amber"
    />
  ),
};

const baseMetrics: StaffMetrics = {
  weekSoll: 2310,
  weekIst: 1935,
  weekDelta: -15,
  monthSoll: 9200,
  monthIst: 9450,
  monthDelta: 250,
  accountStart: new Date(2026, 4, 13),
  accountSoll: 30000,
  accountIst: 30120,
  accountBalance: 120,
};

export const Cards: Story = {
  render: () => <KpiCards metrics={baseMetrics} />,
};

export const CardsBalanced: Story = {
  render: () => (
    <KpiCards
      metrics={{
        ...baseMetrics,
        monthDelta: 0,
        accountBalance: 0,
      }}
    />
  ),
};

export const Toggle: Story = {
  render: function ToggleStory() {
    const [value, setValue] = useState<ViewMode>("month");
    return <ViewToggle value={value} onChange={setValue} />;
  },
};
