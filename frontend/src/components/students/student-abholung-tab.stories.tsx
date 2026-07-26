import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { StudentAbholungTab } from "./student-abholung-tab";

const meta: Meta<typeof StudentAbholungTab> = {
  title: "components/students/StudentAbholungTab",
  component: StudentAbholungTab,
  parameters: {
    layout: "padded",
  },
};

export default meta;
type Story = StoryObj<typeof StudentAbholungTab>;

export const Default: Story = {
  args: {
    studentId: "1",
  },
};

export const Sick: Story = {
  args: {
    studentId: "1",
    isSick: true,
  },
};

export const Excused: Story = {
  args: {
    studentId: "1",
    isExcused: true,
  },
};
