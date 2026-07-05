import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ExtendedStudent } from "~/lib/hooks/use-student-data";
import type { SupervisorContact } from "~/lib/student-helpers";
import {
  StudentDetailHeader,
  SupervisorsCard,
  PersonalInfoReadOnly,
  StudentHistorySection,
} from "./student-detail-components";

const mockStudent: ExtendedStudent = {
  id: "1",
  first_name: "Max",
  second_name: "Mustermann",
  name: "Max Mustermann",
  birthday: "2015-05-15",
  school_class: "3a",
  group_id: "g1",
  group_name: "Gruppe 1",
  buskind: false,
  pickup_status: "selbst",
  sick: false,
  current_location: "Klassenraum",
  location_since: "2024-01-15T08:00:00Z",
  bus: false,
};

const mockSupervisors: SupervisorContact[] = [
  {
    id: 1,
    first_name: "Anna",
    last_name: "Schmidt",
    email: "anna.schmidt@example.com",
    role: "Gruppenleitung",
  },
  {
    id: 2,
    first_name: "Tom",
    last_name: "Weber",
    role: "Gruppenleitung",
  },
];

// =============================================================================
// StudentDetailHeader
// =============================================================================

const headerMeta = {
  title: "students/StudentDetailHeader",
  component: StudentDetailHeader,
  args: {
    student: mockStudent,
    myGroups: [],
    myGroupRooms: [],
    mySupervisedRooms: [],
  },
} satisfies Meta<typeof StudentDetailHeader>;

export default headerMeta;
type HeaderStory = StoryObj<typeof headerMeta>;

export const Default: HeaderStory = {};

export const WithTodayTimes: HeaderStory = {
  args: {
    todayArrivalPlannedTime: "08:00:00",
    todayArrivalActualTime: "08:05:00",
    todayPickupPlannedTime: "16:00:00",
  },
};

export const AbsentToday: HeaderStory = {
  args: {
    student: { ...mockStudent, sick: true },
    sickReason: "Erkältung",
  },
};

// =============================================================================
// SupervisorsCard
// =============================================================================

export const SupervisorsCardStory: StoryObj<typeof SupervisorsCard> = {
  render: () => (
    <SupervisorsCard
      supervisors={mockSupervisors}
      studentName="Max Mustermann"
    />
  ),
};

// =============================================================================
// PersonalInfoReadOnly
// =============================================================================

export const PersonalInfoReadOnlyStory: StoryObj<typeof PersonalInfoReadOnly> =
  {
    render: () => <PersonalInfoReadOnly student={mockStudent} />,
  };

export const PersonalInfoReadOnlyEditable: StoryObj<
  typeof PersonalInfoReadOnly
> = {
  render: () => (
    <PersonalInfoReadOnly
      student={mockStudent}
      showEditButton
      onEditClick={() => {
        // no-op for storybook
      }}
    />
  ),
};

// =============================================================================
// StudentHistorySection
// =============================================================================

export const StudentHistorySectionStory: StoryObj<
  typeof StudentHistorySection
> = {
  render: () => (
    <StudentHistorySection
      studentId="1"
      attendanceLogEnabled
      feedbackEnabled
      onNavigate={() => {
        // no-op for storybook
      }}
    />
  ),
};

export const StudentHistorySectionReadOnly: StoryObj<
  typeof StudentHistorySection
> = {
  render: () => (
    <StudentHistorySection
      studentId="1"
      attendanceLogEnabled={false}
      feedbackEnabled={false}
      readOnly
      onNavigate={() => {
        // no-op for storybook
      }}
    />
  ),
};
