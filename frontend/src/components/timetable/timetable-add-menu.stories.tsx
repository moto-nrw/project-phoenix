import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { TimetableAddMenu } from "./timetable-add-menu";

const meta = {
  title: "components/timetable/TimetableAddMenu",
  component: TimetableAddMenu,
  args: {
    onAddInstance: () => undefined,
    onAddSeries: () => undefined,
  },
} satisfies Meta<typeof TimetableAddMenu>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Disabled: Story = {
  args: {
    disabled: true,
  },
};
