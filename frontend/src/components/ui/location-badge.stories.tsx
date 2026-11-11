import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import type { PresenceStudent } from "@/lib/presence-status";
import { LocationBadge } from "./location-badge";

const meta = {
  title: "components/ui/LocationBadge",
  component: LocationBadge,
} satisfies Meta<typeof LocationBadge>;

export default meta;

type Story = StoryObj<typeof meta>;

const presentStudent: PresenceStudent = {
  current_location: "Raum 101",
  location_since: "2026-07-01T09:15:00.000Z",
  group_name: "Gruppe A",
};

const homeStudent: PresenceStudent = {
  current_location: "Zuhause",
  location_since: "2026-07-01T14:00:00.000Z",
};

const sickAtHomeStudent: PresenceStudent = {
  current_location: "Zuhause",
  sick: true,
  sick_since: "2026-07-01T07:30:00.000Z",
};

const sickButPresentStudent: PresenceStudent = {
  current_location: "Raum 101",
  sick: true,
};

const excusedStudent: PresenceStudent = {
  current_location: "Zuhause",
  excused: true,
};

const classTripStudent: PresenceStudent = {
  current_location: "Zuhause",
  class_trip: true,
};

const notArrivalStudent: PresenceStudent = {
  current_location: "Zuhause",
  arrival_is_exception: true,
  arrival_notes: "Arzttermin",
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

const SINCE = "2026-09-09T06:15:00.000Z";

export const AtSchool: Story = {
  args: {
    student: { current_location: "Schule", location_since: SINCE },
    displayMode: "roomName",
    showLocationSince: true,
  },
};

export const InClass: Story = {
  args: {
    student: { current_location: "Im Unterricht", location_since: SINCE },
    displayMode: "roomName",
    showLocationSince: true,
  },
};

export const Schoolyard: Story = {
  args: {
    student: { current_location: "Schulhof", location_since: SINCE },
    displayMode: "roomName",
    showLocationSince: true,
  },
};

export const SchoolyardRoomColor: Story = {
  args: {
    student: { current_location: "Schulhof", current_room_color: "#0EA5E9" },
    displayMode: "roomName",
  },
};

export const Transit: Story = {
  args: {
    student: { current_location: "Unterwegs", location_since: SINCE },
    displayMode: "roomName",
    showLocationSince: true,
  },
};

export const Unknown: Story = {
  args: {
    student: { current_location: null, location_since: SINCE },
    displayMode: "roomName",
    showLocationSince: true,
  },
};

export const RoomWithSince: Story = {
  args: {
    student: { current_location: "Anwesend - Raum 101", location_since: SINCE },
    displayMode: "roomName",
    showLocationSince: true,
  },
};

export const RoomColor: Story = {
  args: {
    student: {
      current_location: "Anwesend - Werkraum",
      current_room_color: "#D946EF",
    },
    displayMode: "roomName",
  },
};

export const OwnGroupRoom: Story = {
  args: {
    student: { current_location: "Anwesend - Gruppenraum" },
    displayMode: "roomName",
    groupRooms: ["Gruppenraum"],
  },
};

export const ContextAwareForeignChild: Story = {
  args: {
    student: { current_location: "Anwesend - Raum 101", group_id: "2" },
    displayMode: "contextAware",
    userGroups: ["1"],
  },
};

export const ContextAwareForeignTransit: Story = {
  args: {
    student: { current_location: "Unterwegs", group_id: "2" },
    displayMode: "contextAware",
    userGroups: ["1"],
  },
};

export const ContextAwareOwnChild: Story = {
  args: {
    student: { current_location: "Anwesend - Raum 101", group_id: "1" },
    displayMode: "contextAware",
    userGroups: ["1"],
  },
};

export const UnplannedPresent: Story = {
  args: {
    student: {
      current_location: "Anwesend - Raum 101",
      arrival_is_exception: true,
      arrival_notes: "Arzttermin",
    },
    displayMode: "roomName",
  },
};

export const UnplannedPresentDespiteSickNote: Story = {
  args: {
    student: {
      current_location: "Anwesend - Raum 101",
      sick: true,
      arrival_is_exception: true,
    },
    displayMode: "roomName",
  },
};

export const ClassTripButPresent: Story = {
  args: {
    student: { current_location: "Anwesend - Raum 101", class_trip: true },
    displayMode: "roomName",
  },
};

export const ExcusedButPresent: Story = {
  args: {
    student: { current_location: "Schulhof", excused: true },
    displayMode: "roomName",
  },
};

export const SickAndExcusedAtHome: Story = {
  args: {
    student: {
      current_location: "Zuhause",
      sick: true,
      excused: true,
      location_since: SINCE,
    },
    displayMode: "roomName",
    showLocationSince: true,
  },
};

export const ClassTripAtHomeSince: Story = {
  args: {
    student: {
      current_location: "Zuhause",
      class_trip: true,
      class_trip_since: SINCE,
    },
    displayMode: "roomName",
    showLocationSince: true,
  },
};

export const ExcusedAtSchoolSince: Story = {
  args: {
    student: {
      current_location: "Schule",
      excused: true,
      location_since: SINCE,
    },
    displayMode: "roomName",
    showLocationSince: true,
  },
};

export const NotArrivalAtHomeSince: Story = {
  args: {
    student: {
      current_location: "Zuhause",
      location_since: SINCE,
      arrival_is_exception: true,
      arrival_notes: "Arzttermin",
    },
    displayMode: "roomName",
    showLocationSince: true,
  },
};

export const NotArrivalAtSchool: Story = {
  args: {
    student: {
      current_location: "Schule",
      location_since: SINCE,
      arrival_is_exception: true,
    },
    displayMode: "roomName",
    showLocationSince: true,
  },
};

export const SmallSizeWithHint: Story = {
  args: {
    student: { current_location: "Anwesend - Raum 101", sick: true },
    displayMode: "roomName",
    size: "sm",
  },
};

export const LargeSizeWithHint: Story = {
  args: {
    student: { current_location: "Anwesend - Raum 101", sick: true },
    displayMode: "roomName",
    size: "lg",
  },
};

export const LongRoomNameWraps: Story = {
  args: {
    student: {
      current_location:
        "Anwesend - Mehrzweckraum im Erdgeschoss neben der Mensa",
    },
    displayMode: "roomName",
  },
  decorators: [
    (StoryFn) => (
      <div style={{ width: 160 }}>
        <StoryFn />
      </div>
    ),
  ],
};

// Rooms with their own colour (#1324): a light room hex and #FF3130, the
// retired Zuhause red that older rooms can still carry.
export const RoomColors: Story = {
  args: { student: presentStudent, displayMode: "roomName" },
  render: () => (
    <div className="flex flex-col items-start gap-2">
      <LocationBadge
        student={{
          current_location: "Anwesend - Raum 101",
          current_room_color: "#A3D977",
        }}
        displayMode="roomName"
      />
      <LocationBadge
        student={{
          current_location: "Anwesend - Raum 101",
          current_room_color: "#FF3130",
        }}
        displayMode="roomName"
      />
      <LocationBadge
        student={{
          current_location: "Schulhof",
          current_room_color: "#FF3130",
        }}
        displayMode="roomName"
      />
    </div>
  ),
};
