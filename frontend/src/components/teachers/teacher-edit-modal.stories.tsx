import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";

import { TeacherEditModal } from "./teacher-edit-modal";
import type { Teacher } from "@/lib/teacher-api";

const mockTeacher: Teacher = {
  id: "1",
  name: "Erika Musterfrau",
  first_name: "Erika",
  last_name: "Musterfrau",
  email: "erika.musterfrau@example.com",
  specialization: "Sozialpädagogik",
  role: "Erzieherin",
};

const meta = {
  title: "components/teachers/TeacherEditModal",
  component: TeacherEditModal,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    isOpen: true,
    onClose: fn(),
    teacher: mockTeacher,
    onSave: fn(),
    loading: false,
    existingPositions: [],
  },
} satisfies Meta<typeof TeacherEditModal>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Loading: Story = {
  args: {
    loading: true,
  },
};

export const WithExistingPositions: Story = {
  args: {
    existingPositions: ["Erzieherin", "Gruppenleitung", "Sozialpädagogik"],
  },
};

export const NoTeacher: Story = {
  args: {
    teacher: null,
  },
};
