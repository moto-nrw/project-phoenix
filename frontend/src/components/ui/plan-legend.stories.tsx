import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { PlanLegend } from "./plan-legend";

const meta = {
  title: "components/ui/PlanLegend",
  component: PlanLegend,
  args: {
    entries: [
      { key: "care", label: "Arbeit am Kind", color: "#5A8E1F" },
      { key: "prep", label: "Vorbereitung", color: "#4A6FA5" },
      { key: "sub", label: "Vertretung", color: "#0F766E" },
      { key: "other", label: "Sonstiges", color: "#8E6C9E" },
      { key: "absent", label: "Abwesend", variant: "hatched" },
      { key: "cancelled", label: "Fällt aus", variant: "cancelled" },
    ],
  },
} satisfies Meta<typeof PlanLegend>;

export default meta;

type Story = StoryObj<typeof meta>;

export const CategoriesAndStates: Story = {};

export const CategoriesOnly: Story = {
  args: {
    entries: [
      { key: "care", label: "Arbeit am Kind", color: "#5A8E1F" },
      { key: "prep", label: "Vorbereitung", color: "#4A6FA5" },
      { key: "untyped", label: "Ohne Schichtart" },
    ],
  },
};
