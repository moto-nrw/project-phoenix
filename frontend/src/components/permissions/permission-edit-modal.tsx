"use client";

import { useMemo } from "react";
import { permissionsConfig } from "@/components/database/configs/permissions.config";
import type { Permission } from "@/lib/auth-helpers";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";

interface Props {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly permission: Permission | null;
  readonly onSave: (data: Partial<Permission>) => Promise<void>;
  readonly loading?: boolean;
}

export function PermissionEditModal({
  isOpen,
  onClose,
  permission,
  onSave,
  loading = false,
}: Props) {
  // Transform permission data to form data structure
  // The form expects permissionSelector: { resource, action } but permission has them as separate fields
  const formData = useMemo(() => {
    if (!permission) return null;
    return {
      ...permission,
      permissionSelector: {
        resource: permission.resource,
        action: permission.action,
      },
    };
  }, [permission]);

  if (!permission || !formData) return null;

  return (
    <DatabaseFormModal<Permission>
      isOpen={isOpen}
      onClose={onClose}
      mode="edit"
      config={permissionsConfig}
      initialData={formData}
      onSubmit={onSave}
      isLoading={loading}
    />
  );
}
