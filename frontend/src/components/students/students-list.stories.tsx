import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import type { Student } from "~/lib/api";
import { StudentsList } from "./students-list";

const students = [
  {
    id: "1",
    name: "Mia Fischer",
    first_name: "Mia",
    second_name: "Fischer",
    school_class: "3a",
    group_id: "10",
    group_name: "Füchse",
    current_location: "class",
  },
  {
    id: "2",
    name: "Ben Weber",
    first_name: "Ben",
    second_name: "Weber",
    school_class: "3a",
    group_id: "10",
    group_name: "Füchse",
    current_location: "class",
    care_ends_on: "2026-09-30",
    care_exit_recorded: true,
  },
  {
    id: "3",
    name: "Lina Koch",
    first_name: "Lina",
    second_name: "Koch",
    school_class: "2b",
    group_id: "11",
    group_name: "Igel",
    current_location: "class",
  },
] as Student[];

const meta: Meta<typeof StudentsList> = {
  title: "components/students/StudentsList",
  component: StudentsList,
  args: {
    students,
    grouping: "class",
    studentsWithArrival: new Set(["1", "2"]),
    arrivalSummaryById: new Map([["1", "Ankunft 13:15 Uhr"]]),
    onArrivalDataChanged: () => undefined,
    objectHref: (student) =>
      `/students/${student.id}?from=%2Fdatabase%2Fstudents`,
  },
};

export default meta;

type Story = StoryObj<typeof StudentsList>;

export const GroupedByClass: Story = {};

export const GroupedByGroup: Story = {
  args: { grouping: "group" },
};

export const SelectionMode: Story = {
  args: {
    selectionMode: true,
    selectedStudentIds: new Set(["1", "3"]),
    onToggleStudentSelection: () => undefined,
    onClearSelection: () => undefined,
    onFinishSelection: () => undefined,
    onSelectAllVisible: () => undefined,
    onEndCare: () => undefined,
  },
};
