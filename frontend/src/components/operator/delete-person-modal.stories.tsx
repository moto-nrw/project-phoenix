import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import type { OperatorPerson } from "~/lib/operator/provisioning-helpers";
import { DeletePersonModal } from "./delete-person-modal";

const person: OperatorPerson = {
  id: "1",
  firstName: "Anna",
  lastName: "Beispiel",
  fullName: "Anna Beispiel",
  hasAccount: true,
  accountEmail: "anna.beispiel@example.com",
  hasRfidCard: true,
  isStaff: true,
  isStudent: false,
  schoolId: "1",
  schoolName: "OGS am Berg",
  organizationId: "1",
  organizationName: "Träger am Berg",
  createdAt: "2026-01-01T00:00:00Z",
};

const meta = {
  title: "components/operator/DeletePersonModal",
  component: DeletePersonModal,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    person,
    onClose: () => {},
    onDeleted: async () => {},
  },
} satisfies Meta<typeof DeletePersonModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Closed: Story = {
  args: {
    person: null,
  },
};
