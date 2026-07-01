import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import type { StudentLocationContext } from "@/lib/location-helper";
import { LocationBadge } from "./location-badge";

const meta = {
  title: "components/ui/LocationBadge",
  component: LocationBadge,
} satisfies Meta<typeof LocationBadge>;

export default meta;

type Story = StoryObj<typeof meta>;

const presentStudent: StudentLocationContext = {
  current_location: "Raum 101",
  location_since: "2026-07-01T09:15:00.000Z",
  group_name: "Gruppe A",
};

const homeStudent: StudentLocationContext = {
  current_location: "Zuhause",
  location_since: "2026-07-01T14:00:00.000Z",
};

const sickAtHomeStudent: StudentLocationContext = {
  current_location: "Zuhause",
  sick: true,
  sick_since: "2026-07-01T07:30:00.000Z",
};

const sickButPresentStudent: StudentLocationContext = {
  current_location: "Raum 101",
  sick: true,
};

const excusedStudent: StudentLocationContext = {
  current_location: "Zuhause",
  excused: true,
};

const classTripStudent: StudentLocationContext = {
  current_location: "Zuhause",
  class_trip: true,
};

const notArrivalStudent: StudentLocationContext = {
  current_location: "Zuhause",
  not_arrival_today: true,
  not_arrival_reason: "Arzttermin",
};

export const Present: Story = {
  args: {
    student: presentStudent,
    displayMode: "roomName",
  },
};

export const AtHome: Story = {
  args: {
    student: homeStudent,
    displayMode: "roomName",
    showLocationSince: true,
  },
};

export const SickAtHome: Story = {
  args: {
    student: sickAtHomeStudent,
    displayMode: "roomName",
    showLocationSince: true,
  },
};

export const SickButPresent: Story = {
  args: {
    student: sickButPresentStudent,
    displayMode: "roomName",
  },
};

export const Excused: Story = {
  args: {
    student: excusedStudent,
    displayMode: "roomName",
  },
};

export const ClassTrip: Story = {
  args: {
    student: classTripStudent,
    displayMode: "roomName",
  },
};

export const NotArrivalToday: Story = {
  args: {
    student: notArrivalStudent,
    displayMode: "roomName",
  },
};

export const SimpleVariant: Story = {
  args: {
    student: presentStudent,
    displayMode: "roomName",
    variant: "simple",
  },
};

export const GroupNameMode: Story = {
  args: {
    student: presentStudent,
    displayMode: "groupName",
  },
};

export const SmallSize: Story = {
  args: {
    student: presentStudent,
    displayMode: "roomName",
    size: "sm",
  },
};

export const LargeSize: Story = {
  args: {
    student: presentStudent,
    displayMode: "roomName",
    size: "lg",
  },
};
