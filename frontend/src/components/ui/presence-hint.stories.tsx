import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { PresenceHint } from "./presence-hint";

const meta = {
  title: "components/ui/PresenceHint",
  component: PresenceHint,
} satisfies Meta<typeof PresenceHint>;

export default meta;

type Story = StoryObj<typeof meta>;

export const UnplannedPresent: Story = {
  args: { kind: "notArrival", title: "Arzttermin" },
};

export const Sick: Story = {
  args: { kind: "sick" },
};

export const ClassTrip: Story = {
  args: { kind: "classTrip" },
};

export const Excused: Story = {
  args: { kind: "excused" },
};

export const Small: Story = {
  args: { kind: "sick", size: "sm" },
};

export const Large: Story = {
  args: { kind: "sick", size: "lg" },
};
