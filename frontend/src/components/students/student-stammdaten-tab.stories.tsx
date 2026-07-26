import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { StudentStammdatenTab } from "./student-stammdaten-tab";
import type { Student } from "~/lib/api";

const baseStudent: Student = {
  id: "1",
  name: "Max Mustermann",
  first_name: "Max",
  second_name: "Mustermann",
  school_class: "3a",
  group_id: "1",
  current_location: "group_room",
  has_full_access: true,
};

const groups = [
  { value: "1", label: "Gruppe Sonne" },
  { value: "2", label: "Gruppe Mond" },
];

const meta: Meta<typeof StudentStammdatenTab> = {
  title: "components/students/StudentStammdatenTab",
  component: StudentStammdatenTab,
  parameters: {
    layout: "padded",
  },
};

export default meta;
type Story = StoryObj<typeof StudentStammdatenTab>;

export const Default: Story = {
  args: {
    student: baseStudent,
    groups,
    onSave: async () => {},
  },
};

export const NoWriteAccess: Story = {
  args: {
    student: { ...baseStudent, has_full_access: false },
    groups,
    onSave: async () => {},
  },
};
