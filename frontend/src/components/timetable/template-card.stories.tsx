import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";

import type { TimetableTemplate } from "~/lib/timetable-types";
import { TemplateCard } from "./template-card";

const baseTemplate: TimetableTemplate = {
  id: "1",
  name: "Hausaufgabenbetreuung",
  type: "care",
  categoryId: "10",
  categoryName: "Betreuung",
  roomId: "5",
  roomName: "Raum 101",
  isOpen: true,
  maxParticipants: 20,
  targetGroupType: "none",
  enrollmentCount: 12,
  supervisorCount: 2,
  requiredStaffCount: 1,
  assignedStaffCount: 2,
  studentIds: [],
  staffIds: [],
  weekdayAssignments: [],
  schedules: [
    {
      id: "s1",
      weekday: 1,
      startTime: "14:00",
      endTime: "15:30",
      weekPattern: 1,
    },
    {
      id: "s2",
      weekday: 3,
      startTime: "14:00",
      endTime: "15:30",
      weekPattern: 1,
    },
  ],
};

const meta: Meta<typeof TemplateCard> = {
  title: "timetable/TemplateCard",
  component: TemplateCard,
  args: {
    onEdit: fn(),
    onApply: fn(),
    onArchive: fn(),
  },
};

export default meta;
type Story = StoryObj<typeof TemplateCard>;

export const Default: Story = {
  args: {
    template: baseTemplate,
  },
};

export const Understaffed: Story = {
  args: {
    template: {
      ...baseTemplate,
      assignedStaffCount: 1,
      requiredStaffCount: 3,
    },
  },
};

export const ExternalActivityNoRoom: Story = {
  args: {
    template: {
      ...baseTemplate,
      id: "2",
      name: "Schach-AG",
      type: "activity",
      categoryName: "AG",
      roomId: undefined,
      roomName: undefined,
      enrollmentCount: 1,
      supervisorCount: 1,
      weekdayAssignments: [],
      schedules: [
        {
          id: "s3",
          weekday: 5,
          startTime: "13:00",
          endTime: "14:00",
          weekPattern: 1,
        },
      ],
    },
  },
};

export const NoSchedules: Story = {
  args: {
    template: {
      ...baseTemplate,
      id: "3",
      name: "Musik AG",
      type: "external",
      categoryName: "Extern",
      enrollmentCount: 0,
      supervisorCount: 0,
      weekdayAssignments: [],
      schedules: [],
    },
  },
};
