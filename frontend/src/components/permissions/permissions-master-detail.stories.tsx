import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import type { Permission } from "@/lib/auth-helpers";
import { PermissionsMasterDetail } from "./permissions-master-detail";

const permissions: Permission[] = [
  {
    id: "1",
    name: "students:read",
    description: "Schüler ansehen",
    resource: "students",
    action: "read",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
  {
    id: "2",
    name: "students:write",
    description: "Schüler bearbeiten",
    resource: "students",
    action: "write",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
];

const meta: Meta<typeof PermissionsMasterDetail> = {
  title: "components/permissions/PermissionsMasterDetail",
  component: PermissionsMasterDetail,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof PermissionsMasterDetail>;

function InteractiveWrapper() {
  const [selectedId, setSelectedId] = useState<string | null>(
    permissions[0]?.id ?? null,
  );

  const selectedPermission =
    permissions.find((permission) => permission.id === selectedId) ?? null;

  return (
    <div style={{ height: "600px" }}>
      <PermissionsMasterDetail
        permissions={permissions}
        selectedId={selectedId}
        selectedPermission={selectedPermission}
        onSelect={setSelectedId}
      />
    </div>
  );
}

export const Default: Story = {
  render: () => <InteractiveWrapper />,
};

export const NoSelection: Story = {
  args: {
    permissions,
    selectedId: null,
    selectedPermission: null,
    onSelect: () => undefined,
  },
  decorators: [
    (StoryComponent) => (
      <div style={{ height: "600px" }}>
        <StoryComponent />
      </div>
    ),
  ],
};

export const Empty: Story = {
  args: {
    permissions: [],
    selectedId: null,
    selectedPermission: null,
    onSelect: () => undefined,
  },
  decorators: [
    (StoryComponent) => (
      <div style={{ height: "600px" }}>
        <StoryComponent />
      </div>
    ),
  ],
};
