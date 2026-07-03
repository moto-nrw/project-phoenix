import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { Student } from "@/lib/api";
import { StudentEditModal } from "./student-edit-modal";

const mockStudent: Student = {
  id: "1",
  name: "Max Mustermann",
  first_name: "Max",
  second_name: "Mustermann",
  school_class: "3a",
  group_id: "1",
  current_location: "home",
};

const meta = {
  title: "components/students/StudentEditModal",
  component: StudentEditModal,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    isOpen: true,
    onClose: () => undefined,
    onSave: async () => undefined,
    student: mockStudent,
  },
} satisfies Meta<typeof StudentEditModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Open: Story = {};

export const Closed: Story = {
  args: {
    isOpen: false,
  },
};

export const Loading: Story = {
  args: {
    loading: true,
  },
};

export const WithGroups: Story = {
  args: {
    groups: [
      { value: "1", label: "Gruppe Sonne" },
      { value: "2", label: "Gruppe Mond" },
    ],
  },
};

export const NoStudent: Story = {
  args: {
    student: null,
  },
};
