import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import type { StudentLocationContext } from "@/lib/location-helper";

import { PresenceBadge } from "./presence-badge";

const meta: Meta<typeof PresenceBadge> = {
  title: "components/ui/PresenceBadge",
  component: PresenceBadge,
};

export default meta;
type Story = StoryObj<typeof PresenceBadge>;

const presentStudent: StudentLocationContext = {
  current_location: "Gruppenraum 1",
};

const schoolyardStudent: StudentLocationContext = {
  current_location: "Schulhof",
};

const absentStudent: StudentLocationContext = {
  current_location: null,
};

const sickStudent: StudentLocationContext = {
  current_location: null,
  sick: true,
  sick_since: "2026-07-01T08:00:00Z",
};

export const Present: Story = {
  args: {
    student: presentStudent,
  },
};

export const Schoolyard: Story = {
  args: {
    student: schoolyardStudent,
  },
};

export const Absent: Story = {
  args: {
    student: absentStudent,
  },
};

export const Sick: Story = {
  args: {
    student: sickStudent,
  },
};

export const SimpleVariant: Story = {
  args: {
    student: presentStudent,
    variant: "simple",
  },
};

export const WithLocationSince: Story = {
  args: {
    student: {
      ...presentStudent,
      location_since: "2026-07-01T09:30:00Z",
    },
    showLocationSince: true,
  },
};

export const SmallSize: Story = {
  args: {
    student: presentStudent,
    size: "sm",
  },
};

export const LargeSize: Story = {
  args: {
    student: presentStudent,
    size: "lg",
  },
};
