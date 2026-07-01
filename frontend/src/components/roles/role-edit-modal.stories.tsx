import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { RoleEditModal } from "./role-edit-modal";
import type { Role } from "@/lib/auth-helpers";

const role: Role = {
  id: "1",
  name: "Erzieher",
  description: "Standardrolle für pädagogisches Personal",
  isSystem: false,
  baseRole: "staff",
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
  permissions: [],
};

const meta = {
  title: "components/roles/RoleEditModal",
  component: RoleEditModal,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    isOpen: true,
    onClose: () => undefined,
    role,
    onSave: async () => undefined,
  },
} satisfies Meta<typeof RoleEditModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Open: Story = {};

export const Closed: Story = {
  args: {
    isOpen: false,
  },
};

export const NoRole: Story = {
  args: {
    role: null,
  },
};
