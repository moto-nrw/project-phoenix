import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { StudentGuardiansTab } from "./student-guardians-tab";
import { ToastProvider } from "~/contexts/ToastContext";
import type { Student } from "~/lib/api";

const baseStudent: Student = {
  id: "1",
  name: "Max Mustermann",
  school_class: "3a",
  current_location: "unknown",
};

const meta = {
  title: "students/StudentGuardiansTab",
  component: StudentGuardiansTab,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
} satisfies Meta<typeof StudentGuardiansTab>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Editable: Story = {
  args: {
    student: { ...baseStudent, has_full_access: true, has_write_access: true },
  },
};

export const ReadOnly: Story = {
  args: {
    student: {
      ...baseStudent,
      has_full_access: true,
      has_write_access: false,
    },
  },
};

export const NoFullAccess: Story = {
  args: {
    student: { ...baseStudent, has_full_access: false },
  },
};
