import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { GroupTransferModal } from "./group-transfer-modal";

const meta = {
  title: "components/groups/GroupTransferModal",
  component: GroupTransferModal,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    isOpen: true,
    onClose: fn(),
    onTransfer: fn(),
  },
} satisfies Meta<typeof GroupTransferModal>;

export default meta;

type Story = StoryObj<typeof meta>;

const group = {
  id: "group-1",
  name: "Die Sonnenblumen",
  studentCount: 18,
};

const availableUsers = [
  {
    id: "user-1",
    personId: "person-1",
    firstName: "Anna",
    lastName: "Schmidt",
    fullName: "Anna Schmidt",
    email: "anna.schmidt@example.com",
  },
  {
    id: "user-2",
    personId: "person-2",
    firstName: "Max",
    lastName: "Mustermann",
    fullName: "Max Mustermann",
    email: "max.mustermann@example.com",
  },
];

export const Default: Story = {
  args: {
    group,
    availableUsers,
  },
};

export const NoAvailableUsers: Story = {
  args: {
    group,
    availableUsers: [],
  },
};

export const WithExistingTransfer: Story = {
  args: {
    group,
    availableUsers,
    existingTransfers: [
      {
        targetName: "Anna Schmidt",
        substitutionId: "sub-1",
        targetStaffId: "person-1",
      },
    ],
    onCancelTransfer: fn(),
  },
};

export const Closed: Story = {
  args: {
    group,
    availableUsers,
    isOpen: false,
  },
};
