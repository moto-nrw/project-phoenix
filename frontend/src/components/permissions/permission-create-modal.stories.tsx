import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { PermissionCreateModal } from "./permission-create-modal";

const meta = {
  title: "components/permissions/PermissionCreateModal",
  component: PermissionCreateModal,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    isOpen: true,
    onClose: fn(),
    onCreate: fn(),
  },
} satisfies Meta<typeof PermissionCreateModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Closed: Story = {
  args: {
    isOpen: false,
  },
};
