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
  readonly error?: string | null;
}

export function PermissionEditModal({
  isOpen,
  onClose,
  permission,
  onSave,
  loading = false,
  error = null,
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
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-pink-600" />
            <p className="text-gray-600">Daten werden geladen...</p>
          </div>
        </div>
      ) : (
        <DatabaseForm
          theme={permissionsConfig.theme}
          sections={permissionsConfig.form.sections.map(configToFormSection)}
          initialData={formData}
          onSubmit={onSave}
          onCancel={onClose}
          isLoading={loading}
          error={error}
          submitLabel="Speichern"
          stickyActions
        />
      )}
    </Modal>
  );
}
