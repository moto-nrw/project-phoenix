import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { Role } from "@/lib/auth-helpers";
import { RolePermissionsTab } from "./role-permissions-tab";

const role: Role = {
  id: "2",
  name: "Vertretungslehrkraft",
  description: "Vertritt im Krankheitsfall.",
  isSystem: false,
  baseRole: "teacher",
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
  permissions: [],
};

const meta: Meta<typeof RolePermissionsTab> = {
  title: "components/roles/RolePermissionsTab",
  component: RolePermissionsTab,
  // Lädt über authService; im Storybook zeigt der Reiter den Ladezustand
  // bzw. die Fehlermeldung, die Anatomie der Zustände steht im Test.
  args: {
    role,
    editing: false,
    onSaved: () => undefined,
    onCancelEdit: () => undefined,
  },
};

export default meta;
type Story = StoryObj<typeof RolePermissionsTab>;

export const View: Story = {};

export const Editing: Story = {
  args: { editing: true },
};
