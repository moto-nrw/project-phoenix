"use client";

import { permissionsConfig } from "@/lib/database/configs/permissions.config";
import type { Permission } from "@/lib/auth-helpers";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";

interface Props {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreate: (data: Partial<Permission>) => Promise<void>;
}

export function PermissionCreateModal({ isOpen, onClose, onCreate }: Props) {
  return (
    <DatabaseFormModal<Permission>
      isOpen={isOpen}
      onClose={onClose}
      title={permissionsConfig.labels?.createModalTitle ?? "Neue Berechtigung"}
      config={permissionsConfig}
      onSubmit={onCreate}
      submitLabel="Erstellen"
      stickyActions
    />
  );
}
