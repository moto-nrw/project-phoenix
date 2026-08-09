import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { ToastProvider } from "~/contexts/ToastContext";
import type { Student } from "~/lib/api";
import { FilteredBulkArrivalModal } from "./class-bulk-arrival-modal";

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
  title: "students/FilteredBulkArrivalModal",
  component: FilteredBulkArrivalModal,
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
    filter: { type: "school_class", schoolClass: "3a" },
    filterLabel: "3a",
    studentsInFilter: students,
    onSuccess: fn(),
  },
} satisfies Meta<typeof FilteredBulkArrivalModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const NoStudents: Story = {
  args: {
    studentsInFilter: [],
  },
};

export const Group: Story = {
  args: {
    filter: { type: "group", groupId: "12" },
    filterLabel: "Füchse",
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
  },
};
