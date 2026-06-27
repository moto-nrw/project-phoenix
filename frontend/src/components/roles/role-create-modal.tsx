"use client";

import { rolesConfig } from "@/lib/database/configs/roles.config";
import type { Role } from "@/lib/auth-helpers";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";

interface Props {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreate: (data: Partial<Role>) => Promise<void>;
}

export function RoleCreateModal({ isOpen, onClose, onCreate }: Props) {
  return (
    <DatabaseFormModal<Role>
      isOpen={isOpen}
      onClose={onClose}
      title={rolesConfig.labels?.createModalTitle ?? "Neue Rolle"}
      config={rolesConfig}
      onSubmit={onCreate}
      submitLabel="Erstellen"
      stickyActions
    />
  );
}
