"use client";

import type { Activity } from "@/lib/activity-helpers";
import { activitiesConfig } from "@/lib/database/configs/activities.config";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";

interface ActivityCreateModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreate: (data: Partial<Activity>) => Promise<void>;
}

export function ActivityCreateModal({
  isOpen,
  onClose,
  onCreate,
}: ActivityCreateModalProps) {
  return (
    <DatabaseFormModal<Activity>
      isOpen={isOpen}
      onClose={onClose}
      title={activitiesConfig.labels?.createModalTitle ?? "Neue Aktivität"}
      config={activitiesConfig}
      onSubmit={onCreate}
      submitLabel="Erstellen"
      stickyActions
    />
  );
}
