import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { KpiCard, KpiCards, ViewToggle } from "./staff-time-views";
import type { ViewMode } from "./staff-time-views";
import type { PeriodMetrics } from "~/lib/hooks/use-period-metrics";

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

// Every figure the real screens render here is date-valid and comes from the
// backend month model (usePeriodMetrics, #1842), never from the current
// schedule — so the stories hand KpiCards that same shape.
const baseMetrics: PeriodMetrics = {
  week: { soll: 2310, ist: 1935, delta: -15 },
  month: { soll: 9200, ist: 9450, delta: 250 },
  accountStart: new Date(2026, 4, 13),
  accountBalanceMinutes: 120,
};

export const Cards: Story = {
  render: () => <KpiCards metrics={baseMetrics} />,
};

export const CardsBalanced: Story = {
  render: () => (
    <KpiCards
      metrics={{
        ...baseMetrics,
        month: { ...baseMetrics.month!, delta: 0 },
        accountBalanceMinutes: 0,
      }}
    />
  ),
};

// Nothing has resolved yet: every card must degrade to "–" rather than render a
// stale or client-computed number.
export const CardsLoading: Story = {
  render: () => (
    <KpiCards
      metrics={{
        week: null,
        month: null,
        accountStart: new Date(2026, 4, 13),
        accountBalanceMinutes: null,
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
