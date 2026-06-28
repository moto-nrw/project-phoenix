"use client";

import { groupsConfig } from "@/components/database/configs/groups.config";
import type { Group } from "@/lib/group-helpers";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";

interface Props {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreate: (data: Partial<Group>) => Promise<void>;
}

export function GroupCreateModal({ isOpen, onClose, onCreate }: Props) {
  return (
    <DatabaseFormModal<Group>
      isOpen={isOpen}
      onClose={onClose}
      title={groupsConfig.labels?.createModalTitle ?? "Neue Gruppe"}
      config={groupsConfig}
      onSubmit={onCreate}
      submitLabel="Erstellen"
      stickyActions
    />
  );
}
