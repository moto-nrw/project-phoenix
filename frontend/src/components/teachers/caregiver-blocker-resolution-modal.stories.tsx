import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { ToastProvider } from "~/contexts/ToastContext";
import type { CaregiverCapabilityState } from "~/lib/caregiver-capability-api";
import { CaregiverBlockerResolutionModal } from "./caregiver-blocker-resolution-modal";

const baseState: CaregiverCapabilityState = {
  accountId: "1",
  email: "lehrkraft@example.de",
  firstName: "Max",
  lastName: "Mustermann",
  personId: "1",
  staffId: "1",
  teacherId: "1",
  hasAdminRole: false,
  hasUserRole: true,
  hasPerson: true,
  hasStaff: true,
  hasTeacher: true,
  hasCaregiverProfile: true,
  isActiveCaregiver: false,
  disableBlocked: false,
  disableBlockers: [],
  activeSupervisions: [],
  activeSubstitutions: [],
  activitySupervisions: [],
  groupAssignments: [],
};

const withBlockersState: CaregiverCapabilityState = {
  ...baseState,
  activeSupervisions: [
    { id: "sup-1", groupName: "Gruppe Sonne", startDate: "01.09.2026" },
  ],
  activeSubstitutions: [
    {
      id: "sub-1",
      groupName: "Gruppe Mond",
      role: "substitute",
      startDate: "01.09.2026",
      endDate: "15.09.2026",
    },
  ],
  activitySupervisions: [
    {
      id: "act-1",
      activityId: "10",
      activityName: "Fußball AG",
      isPrimary: true,
    },
  ],
  groupAssignments: [
    {
      id: "grp-1",
      groupId: "20",
      groupName: "Gruppe Sterne",
      teacherId: "1",
      teacherIds: ["1"],
    },
  ],
};

const meta: Meta<typeof CaregiverBlockerResolutionModal> = {
  title: "teachers/CaregiverBlockerResolutionModal",
  component: CaregiverBlockerResolutionModal,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
  args: {
    isOpen: true,
    onClose: fn(),
    onResolved: fn(),
    state: baseState,
  },
};

export default meta;
type Story = StoryObj<typeof CaregiverBlockerResolutionModal>;

export const Resolved: Story = {
  args: {
    state: baseState,
  },
};

export const WithBlockers: Story = {
  args: {
    state: withBlockersState,
  },
};
