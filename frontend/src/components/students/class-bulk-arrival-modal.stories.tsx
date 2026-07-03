import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { ToastProvider } from "~/contexts/ToastContext";
import type { Student } from "~/lib/api";
import { ClassBulkArrivalModal } from "./class-bulk-arrival-modal";

const students: Student[] = [
  {
    id: "1",
    name: "Anna Beispiel",
    school_class: "3a",
    current_location: "unknown",
  },
  {
    id: "2",
    name: "Ben Muster",
    school_class: "3a",
    current_location: "unknown",
  },
];

const meta = {
  title: "students/ClassBulkArrivalModal",
  component: ClassBulkArrivalModal,
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
    schoolClass: "3a",
    studentsInClass: students,
    onSuccess: fn(),
  },
} satisfies Meta<typeof ClassBulkArrivalModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const NoStudents: Story = {
  args: {
    studentsInClass: [],
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
  },
};
