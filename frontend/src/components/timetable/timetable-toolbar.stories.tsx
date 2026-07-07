import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";

import { Button } from "~/components/ui/button";
import { TimetableToolbar } from "./timetable-toolbar";

const meta = {
  title: "timetable/TimetableToolbar",
  component: TimetableToolbar,
  parameters: {
    layout: "padded",
  },
  args: {
    view: "week",
    onViewChange: fn(),
    rangeLabel: "KW 18 · Mo 28.04 – Fr 02.05.2026",
    onPrev: fn(),
    onNext: fn(),
    onToday: fn(),
  },
} satisfies Meta<typeof TimetableToolbar>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const OnToday: Story = {
  args: {
    isOnToday: true,
  },
};

export const NavDisabled: Story = {
  args: {
    navDisabled: true,
  },
};

export const WithDensityPicker: Story = {
  args: {
    density: "normal",
    onDensityChange: fn(),
  },
};

export const MonthView: Story = {
  args: {
    view: "month",
    rangeLabel: "April 2026",
  },
};

export const SeriesView: Story = {
  args: {
    view: "series",
    rangeLabel: "Regeltermine",
  },
};

export const FullToolbar: Story = {
  args: {
    density: "comfortable",
    onDensityChange: fn(),
    onAddInstance: fn(),
    onManagePeriods: fn(),
    planWeekAction: (
      <Button type="button" variant="outline" size="compact">
        Woche planen
      </Button>
    ),
  },
};
