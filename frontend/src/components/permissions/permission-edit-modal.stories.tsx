import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { PermissionEditModal } from "./permission-edit-modal";
import type { Permission } from "@/lib/auth-helpers";

const samplePermission: Permission = {
  id: "1",
  name: "students.read",
  resource: "students",
  action: "read",
  description: "Kann Schülerdaten einsehen",
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

const meta = {
  title: "components/permissions/PermissionEditModal",
  component: PermissionEditModal,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    isOpen: true,
    onClose: fn(),
    permission: samplePermission,
    onSave: fn(),
    loading: false,
  },
} satisfies Meta<typeof PermissionEditModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Loading: Story = {
  args: {
    loading: true,
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
  },
};
