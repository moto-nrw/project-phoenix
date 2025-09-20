"use client";

import { useState } from "react";
import { DatabasePage } from "@/components/ui/database";
import { rolesConfig } from "@/lib/database/configs/roles.config";
import { RolePermissionManagementModal } from "@/components/auth";
import type { Role } from "@/lib/auth-helpers";

export default function RolesPage() {
  const [permissionModalOpen, setPermissionModalOpen] = useState(false);
  const [selectedRole, setSelectedRole] = useState<Role | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);

  // Create a modified config with our custom action handler
  const modifiedConfig = {
    ...rolesConfig,
    detail: {
      ...rolesConfig.detail,
      actions: {
        ...rolesConfig.detail.actions,
        custom: [
          {
            label: 'Berechtigungen verwalten',
            onClick: (role: Role) => {
              setSelectedRole(role);
              setPermissionModalOpen(true);
            },
            color: 'bg-blue-600 text-white hover:bg-blue-700',
          },
        ],
      },
    },
  };

  const handlePermissionUpdate = () => {
    // Trigger a refresh of the roles list
    setRefreshKey(prev => prev + 1);
  };

  return (
    <>
      <DatabasePage key={refreshKey} config={modifiedConfig} />
      
      {selectedRole && (
        <RolePermissionManagementModal
          isOpen={permissionModalOpen}
          onClose={() => {
            setPermissionModalOpen(false);
            setSelectedRole(null);
          }}
          role={selectedRole}
          onUpdate={handlePermissionUpdate}
        />
      )}
    </>
  );
}