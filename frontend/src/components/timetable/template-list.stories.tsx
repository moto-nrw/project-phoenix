import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";

import type { TimetableTemplate } from "~/lib/timetable-types";
import { TemplateList } from "./template-list";

const templates: TimetableTemplate[] = [
  {
    id: "1",
    name: "Mensa",
    type: "care",
    categoryId: "cat-1",
    categoryName: "Verpflegung",
    roomId: "room-1",
    roomName: "Mensa",
    isOpen: true,
    maxParticipants: 30,
    targetGroupType: "none",
    enrollmentCount: 12,
    supervisorCount: 2,
    requiredStaffCount: 1,
    assignedStaffCount: 2,
    studentIds: [],
    staffIds: [],
    schedules: [
      {
        id: "s1",
        weekday: 1,
        startTime: "12:00",
        endTime: "13:00",
        weekPattern: 1,
      },
    ],
  },
  {
    id: "2",
    name: "Fußball AG",
    type: "activity",
    categoryId: "cat-2",
    categoryName: "Sport",
    isOpen: true,
    maxParticipants: 20,
    targetGroupType: "none",
    enrollmentCount: 8,
    supervisorCount: 1,
    requiredStaffCount: 1,
    assignedStaffCount: 1,
    studentIds: [],
    staffIds: [],
    schedules: [
      {
        id: "s2",
        weekday: 3,
        startTime: "14:00",
        endTime: "15:30",
        weekPattern: 1,
      },
    ],
  },
];

const meta = {
  title: "components/timetable/TemplateList",
  component: TemplateList,
  args: {
    templates: [],
    onCreate: fn(),
    onEdit: fn(),
    onApply: fn(),
    onArchive: fn(),
  },
} satisfies Meta<typeof TemplateList>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Empty: Story = {
  args: {
    templates: [],
  },
};

export const WithTemplates: Story = {
  args: {
    templates,
  },
};
