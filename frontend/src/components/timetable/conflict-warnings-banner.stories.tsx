import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ConflictWarningsBanner } from "./conflict-warnings-banner";

const meta = {
  title: "timetable/ConflictWarningsBanner",
  component: ConflictWarningsBanner,
} satisfies Meta<typeof ConflictWarningsBanner>;

export default meta;
type Story = StoryObj<typeof meta>;

export const NoConflicts: Story = {
  args: {
    conflictCount: 0,
  },
};

export const SingleConflict: Story = {
  args: {
    conflictCount: 1,
  },
};

export const MultipleConflicts: Story = {
  args: {
    conflictCount: 5,
  },
};
