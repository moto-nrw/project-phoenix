import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";

import { TeacherCreateModal } from "./teacher-create-modal";

const meta = {
  title: "components/teachers/TeacherCreateModal",
  component: TeacherCreateModal,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    isOpen: true,
    onClose: fn(),
    onCreate: fn(),
    loading: false,
  },
} satisfies Meta<typeof TeacherCreateModal>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Loading: Story = {
  args: {
    loading: true,
  },
};
