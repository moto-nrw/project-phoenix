import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { GroupCreateModal } from "./group-create-modal";

const meta = {
  title: "components/groups/GroupCreateModal",
  component: GroupCreateModal,
  parameters: {
    layout: "fullscreen",
  },
} satisfies Meta<typeof GroupCreateModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Open: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
    onCreate: async () => {},
  },
};
