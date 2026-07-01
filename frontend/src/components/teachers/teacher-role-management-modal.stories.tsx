import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { ToastProvider } from "~/contexts/ToastContext";
import type { Teacher } from "~/lib/teacher-api";
import { TeacherRoleManagementModal } from "./teacher-role-management-modal";

const baseTeacher: Teacher = {
  id: "1",
  name: "Anna Musterfrau",
  first_name: "Anna",
  last_name: "Musterfrau",
  account_id: 42,
};

const teacherWithoutAccount: Teacher = {
  id: "2",
  name: "Bernd Beispiel",
  first_name: "Bernd",
  last_name: "Beispiel",
};

const meta = {
  title: "components/teachers/TeacherRoleManagementModal",
  component: TeacherRoleManagementModal,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
  args: {
    isOpen: true,
    onClose: fn(),
    onUpdate: fn(),
    teacher: baseTeacher,
  },
} satisfies Meta<typeof TeacherRoleManagementModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WithoutLinkedAccount: Story = {
  args: {
    teacher: teacherWithoutAccount,
  },
};
