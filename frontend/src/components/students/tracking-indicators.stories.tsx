import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { TrackingIndicators } from "./tracking-indicators";

const meta = {
  title: "components/students/TrackingIndicators",
  component: TrackingIndicators,
} satisfies Meta<typeof TrackingIndicators>;

export default meta;
type Story = StoryObj<typeof meta>;

export const AllDone: Story = {
  args: {
    labels: ["Hausaufgaben", "Mensa"],
    results: [true, true],
  },
};

export const AllPending: Story = {
  args: {
    labels: ["Hausaufgaben", "Mensa"],
    results: [false, false],
  },
};

export const Mixed: Story = {
  args: {
    labels: ["Hausaufgaben", "Mensa", "Sport"],
    results: [true, false, true],
  },
};

export const Empty: Story = {
  args: {
    labels: [],
    results: [],
  },
};
