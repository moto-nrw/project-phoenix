import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import {
  StudentCard,
  SchoolClassIcon,
  GroupIcon,
  StudentInfoRow,
  PickupTimeIcon,
  ExceptionIcon,
  AbsenceIcon,
  PickupTimeRow,
  StudentAbsenceRow,
  ArrivalTimeRow,
} from "./student-card";
import { LocationBadge } from "~/components/ui/location-badge";

const meta = {
  title: "students/StudentCard",
  component: StudentCard,
  parameters: {
    layout: "centered",
  },
  args: {
    studentId: "1",
    firstName: "Mia",
    lastName: "Schmidt",
    onClick: () => {
      // no-op for story purposes
    },
    locationBadge: (
      <LocationBadge
        student={{ current_location: "Anwesend" }}
        displayMode="roomName"
      />
    ),
  },
  decorators: [
    (Story) => (
      <div className="w-80">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof StudentCard>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WithPhoto: Story = {
  args: {
    photoUrl: "https://placekitten.com/200/200",
  },
};

export const WithExtraContent: Story = {
  args: {
    extraContent: (
      <StudentInfoRow icon={<SchoolClassIcon />}>Klasse 3a</StudentInfoRow>
    ),
    trackingIndicators: (
      <span className="text-xs text-gray-400">Indikator</span>
    ),
  },
};

export const CheckinModePresent: Story = {
  args: {
    checkinMode: true,
    checkinState: "anwesend",
  },
};

export const CheckinModeAbsent: Story = {
  args: {
    checkinMode: true,
    checkinState: "abwesend",
  },
};

export const CheckinModePending: Story = {
  args: {
    checkinMode: true,
    checkinState: "anwesend",
    isCheckinPending: true,
  },
};

// Small internal sub-parts used to compose card content across pages
// (OGS groups, active supervisions, student search). Grouped in one gallery
// since each takes only trivial/no props.
export const SubPartsGallery: Story = {
  render: () => (
    <div className="flex flex-col gap-2 rounded-2xl border border-gray-200 bg-white p-4">
      <StudentInfoRow icon={<SchoolClassIcon />}>Klasse 3a</StudentInfoRow>
      <StudentInfoRow icon={<GroupIcon />}>Bärengruppe</StudentInfoRow>
      <StudentInfoRow icon={<PickupTimeIcon />}>
        Gehzeit: 15:00 Uhr
      </StudentInfoRow>
      <StudentInfoRow icon={<ExceptionIcon />}>Ausnahme</StudentInfoRow>
      <StudentInfoRow icon={<AbsenceIcon />}>Abwesend</StudentInfoRow>
      <PickupTimeRow
        pickupTime="15:00"
        isException={false}
        now={new Date("2026-07-01T14:00:00")}
      />
      <StudentAbsenceRow label="krank" />
      <ArrivalTimeRow
        arrivalTime="08:00"
        isException={false}
        isAbsent={false}
        now={new Date("2026-07-01T08:05:00")}
      />
    </div>
  ),
};
