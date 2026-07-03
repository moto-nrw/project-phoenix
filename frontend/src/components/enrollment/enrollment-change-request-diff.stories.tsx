import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { EnrollmentChangeRequestDiff } from "./enrollment-change-request-diff";

const meta: Meta<typeof EnrollmentChangeRequestDiff> = {
  title: "enrollment/EnrollmentChangeRequestDiff",
  component: EnrollmentChangeRequestDiff,
};

export default meta;
type Story = StoryObj<typeof EnrollmentChangeRequestDiff>;

export const WithChanges: Story = {
  args: {
    baseSnapshot: {
      guardian_first_name: "Anna",
      guardian_last_name: "Muster",
      guardian_phone: "0170 1234567",
    },
    proposedSnapshot: {
      guardian_first_name: "Anna",
      guardian_last_name: "Musterfrau",
      guardian_phone: "0170 7654321",
    },
    diff: {
      guardian_last_name: true,
      guardian_phone: true,
    },
  },
};

export const Empty: Story = {
  args: {
    baseSnapshot: {},
    proposedSnapshot: {},
    diff: {},
  },
};
