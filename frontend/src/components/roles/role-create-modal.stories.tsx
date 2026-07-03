import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { RoleCreateModal } from "./role-create-modal";

const meta = {
  title: "components/roles/RoleCreateModal",
  component: RoleCreateModal,
  args: {
    isOpen: true,
    onClose: () => undefined,
    onCreate: async () => undefined,
  },
} satisfies Meta<typeof RoleCreateModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
