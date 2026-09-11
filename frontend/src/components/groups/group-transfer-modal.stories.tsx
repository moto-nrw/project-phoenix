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
    fullName: "Anna Schmidt",
  },
  {
    id: "user-2",
    fullName: "Max Mustermann",
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
