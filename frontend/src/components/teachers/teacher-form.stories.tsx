import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";

import { TeacherForm } from "./teacher-form";
import type { Teacher } from "@/lib/teacher-api";

const mockTeacher: Teacher = {
  id: "1",
  name: "Erika Musterfrau",
  first_name: "Erika",
  last_name: "Musterfrau",
  email: "erika.musterfrau@example.com",
  role: "Erzieherin",
};

const meta = {
  title: "components/teachers/TeacherForm",
  component: TeacherForm,
  args: {
    initialData: {},
    onSubmitAction: fn(),
    onCancelAction: fn(),
    isLoading: false,
  },
} satisfies Meta<typeof TeacherForm>;

export default meta;
type Story = StoryObj<typeof meta>;

export const NewTeacher: Story = {};

export const EditExistingTeacher: Story = {
  args: {
    initialData: mockTeacher,
    formTitle: "Personal bearbeiten",
    submitLabel: "Aktualisieren",
  },
};

export const Loading: Story = {
  args: {
    isLoading: true,
  },
};

export const WithoutCardWrapper: Story = {
  args: {
    initialData: mockTeacher,
    wrapInCard: false,
  },
};

export const WithExistingPositions: Story = {
  args: {
    existingPositions: ["Erzieherin", "Gruppenleitung", "Sozialpädagogik"],
  },
};
