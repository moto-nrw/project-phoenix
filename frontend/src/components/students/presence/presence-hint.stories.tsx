import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { PresenceHint } from "./presence-hint";

const meta = {
  title: "components/students/presence/PresenceHint",
  component: PresenceHint,
} satisfies Meta<typeof PresenceHint>;

export default meta;

type Story = StoryObj<typeof meta>;

export const UnplannedPresent: Story = {
  args: { status: "not_arrival", title: "Arzttermin" },
};

export const Sick: Story = {
  args: { status: "sick" },
};

export const ClassTrip: Story = {
  args: { status: "class_trip" },
};

export const Excused: Story = {
  args: { status: "excused" },
};

export const Small: Story = {
  args: { status: "sick", size: "sm" },
};

export const Large: Story = {
  args: { status: "sick", size: "lg" },
};
