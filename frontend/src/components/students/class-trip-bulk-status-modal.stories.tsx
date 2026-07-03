import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { ClassTripBulkStatusModal } from "./class-trip-bulk-status-modal";
import type { Student } from "~/lib/api";

function makeStudent(id: string, name: string): Student {
  return {
    id,
    name,
    school_class: "3a",
    current_location: "GROUP_ROOM",
  };
}

const students: Student[] = [
  makeStudent("1", "Anna Muster"),
  makeStudent("2", "Ben Beispiel"),
  makeStudent("3", "Clara Schmidt"),
];

const meta: Meta<typeof ClassTripBulkStatusModal> = {
  title: "students/ClassTripBulkStatusModal",
  component: ClassTripBulkStatusModal,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    isOpen: true,
    onClose: fn(),
    targetLabel: "Klasse 3a",
    students,
    onSuccess: fn(),
  },
};

export default meta;

type Story = StoryObj<typeof ClassTripBulkStatusModal>;

export const Default: Story = {};

export const SingleStudent: Story = {
  args: {
    targetLabel: "Anna Muster",
    students: [makeStudent("1", "Anna Muster")],
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
  },
};
