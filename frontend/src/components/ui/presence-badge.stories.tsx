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

const SINCE = "2026-09-09T06:15:00.000Z";

export const AtSchool: Story = {
  args: {
    student: { current_location: "Schule", location_since: SINCE },
    showLocationSince: true,
  },
};

export const InClass: Story = {
  args: {
    student: { current_location: "Im Unterricht", location_since: SINCE },
    showLocationSince: true,
  },
};

export const AtHomeSince: Story = {
  args: {
    student: { current_location: "Zuhause", location_since: SINCE },
    showLocationSince: true,
  },
};

export const SchoolyardSince: Story = {
  args: {
    student: { current_location: "Schulhof", location_since: SINCE },
    showLocationSince: true,
  },
};

export const SchoolyardRoomColor: Story = {
  args: {
    student: { current_location: "Schulhof", current_room_color: "#0EA5E9" },
  },
};

export const TransitSince: Story = {
  args: {
    student: { current_location: "Unterwegs", location_since: SINCE },
    showLocationSince: true,
  },
};

export const RoomNameSince: Story = {
  args: {
    student: { current_location: "Anwesend - Raum 101", location_since: SINCE },
    showLocationSince: true,
  },
};

export const UnknownSince: Story = {
  args: {
    student: { current_location: "Irgendwo", location_since: SINCE },
    showLocationSince: true,
  },
};

export const SickAtHomeSince: Story = {
  args: {
    student: { current_location: "Zuhause", sick: true, sick_since: SINCE },
    showLocationSince: true,
  },
};

export const ClassTripAtHome: Story = {
  args: {
    student: { current_location: "Zuhause", class_trip: true },
  },
};

export const ExcusedAtSchoolSince: Story = {
  args: {
    student: {
      current_location: "Schule",
      excused: true,
      location_since: SINCE,
    },
    showLocationSince: true,
  },
};

export const NotArrivalAtHome: Story = {
  args: {
    student: {
      current_location: "Zuhause",
      location_since: SINCE,
      arrival_is_exception: true,
      arrival_notes: "Arzttermin",
    },
    showLocationSince: true,
  },
};

export const SickButPresent: Story = {
  args: {
    student: { current_location: "Anwesend - Raum 101", sick: true },
  },
};

export const ClassTripButPresent: Story = {
  args: {
    student: { current_location: "Schulhof", class_trip: true },
  },
};

export const ExcusedButPresent: Story = {
  args: {
    student: { current_location: "Unterwegs", excused: true },
  },
};

export const UnplannedPresent: Story = {
  args: {
    student: {
      current_location: "Anwesend - Raum 101",
      sick: true,
      arrival_is_exception: true,
      arrival_notes: "Arzttermin",
    },
  },
};

export const SmallSizeWithHint: Story = {
  args: {
    student: { current_location: "Anwesend", excused: true },
    size: "sm",
  },
};

export const LargeSizeWithHint: Story = {
  args: {
    student: { current_location: "Anwesend", excused: true },
    size: "lg",
  },
};
