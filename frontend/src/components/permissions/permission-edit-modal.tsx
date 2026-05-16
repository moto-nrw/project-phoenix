"use client";

import { useMemo } from "react";
import { Modal } from "~/components/ui/modal";
import { DatabaseForm } from "~/components/ui/database/database-form";
import { permissionsConfig } from "@/lib/database/configs/permissions.config";
import { configToFormSection } from "@/lib/database/types";
import type { Permission } from "@/lib/auth-helpers";

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
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={
        permissionsConfig.labels?.editModalTitle ?? "Berechtigung bearbeiten"
      }
    >
      <DatabaseForm
        theme={permissionsConfig.theme}
        sections={permissionsConfig.form.sections.map(configToFormSection)}
        initialData={formData}
        onSubmit={onSave}
        onCancel={onClose}
        isLoading={loading}
        submitLabel="Speichern"
        stickyActions
      />
    </Modal>
  );
}
