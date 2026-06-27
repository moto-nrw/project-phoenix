"use client";

import { rolesConfig } from "@/lib/database/configs/roles.config";
import type { Role } from "@/lib/auth-helpers";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";

interface Props {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly role: Role | null;
  readonly onSave: (data: Partial<Role>) => Promise<void>;
}

export function RoleEditModal({ isOpen, onClose, role, onSave }: Props) {
  if (!role) return null;
  return (
    <DatabaseFormModal<Role>
      isOpen={isOpen}
      onClose={onClose}
      title={rolesConfig.labels?.editModalTitle ?? "Rolle bearbeiten"}
      config={rolesConfig}
      initialData={role}
      onSubmit={onSave}
      submitLabel="Speichern"
      stickyActions
    />
  );
}
