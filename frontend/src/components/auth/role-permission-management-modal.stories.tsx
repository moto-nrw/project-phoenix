import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ToastProvider } from "~/contexts/ToastContext";
import type { Role } from "~/lib/auth-helpers";
import { RolePermissionManagementModal } from "./role-permission-management-modal";

const storybookRole: Role = {
  id: "1",
  name: "teacher",
  description: "Betreuungskraft",
  isSystem: true,
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

const meta = {
  title: "components/auth/RolePermissionManagementModal",
  component: RolePermissionManagementModal,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
  args: {
    isOpen: true,
    onClose: () => undefined,
    onUpdate: () => undefined,
    role: storybookRole,
  },
} satisfies Meta<typeof RolePermissionManagementModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
