import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { Button } from "./button";
import { PlanningContextBar, PlanningDayChip } from "./planning-context-bar";
import { Tabs, TabsList, TabsTrigger } from "./tabs";

const meta = {
  title: "components/ui/PlanningContextBar",
  component: PlanningContextBar,
  args: {
    title: "Vertretung",
    dateLabel: "Mo 13.07.",
    onPrevious: () => {},
    onNext: () => {},
    onToday: () => {},
  },
} satisfies Meta<typeof PlanningContextBar>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    actions: (
      <Button type="button" variant="primary" size="md">
        Neu
      </Button>
    ),
  },
};

const WEEK_DAYS = [
  { weekdayLabel: "Mo", dateLabel: "13.07.", count: 2 },
  { weekdayLabel: "Di", dateLabel: "14.07.", count: 0 },
  { weekdayLabel: "Mi", dateLabel: "15.07.", count: 1 },
  { weekdayLabel: "Do", dateLabel: "16.07.", count: 0 },
  { weekdayLabel: "Fr", dateLabel: "17.07.", count: 0 },
  { weekdayLabel: "Sa", dateLabel: "18.07." },
  { weekdayLabel: "So", dateLabel: "19.07." },
];

export const WithWeekStrip: Story = {
  args: {
    dateLabel: undefined,
    navigationSlot: (
      <div className="flex items-center gap-1">
        {WEEK_DAYS.map((day, index) => (
          <PlanningDayChip
            key={day.weekdayLabel}
            {...day}
            selected={index === 0}
          />
        ))}
      </div>
    ),
  },
};

export const WithViewSwitcher: Story = {
  args: {
    viewSwitcher: (
      <Tabs defaultValue="stoerungen">
        <TabsList>
          <TabsTrigger value="stoerungen">Nur Störungen</TabsTrigger>
          <TabsTrigger value="ganzerTag">Ganzer Tag</TabsTrigger>
        </TabsList>
      </Tabs>
    ),
  },
};

export const WithContextRow: Story = {
  args: {
    children: (
      <>
        <Button type="button" variant="ghost" size="compact">
          2 Lücken
        </Button>
        <Button type="button" variant="ghost" size="compact">
          1 quittiert
        </Button>
      </>
    ),
  },
};
