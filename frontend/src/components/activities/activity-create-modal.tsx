"use client";

import { Modal } from "~/components/ui/modal";
import { DatabaseForm } from "~/components/ui/database/database-form";
import type { Activity } from "@/lib/activity-helpers";
import { activitiesConfig } from "@/lib/database/configs/activities.config";
import { configToFormSection } from "@/lib/database/types";

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
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={activitiesConfig.labels?.createModalTitle ?? "Neue Aktivität"}
    >
      <DatabaseForm
        theme={activitiesConfig.theme}
        sections={activitiesConfig.form.sections.map(configToFormSection)}
        initialData={activitiesConfig.form.defaultValues}
        onSubmit={onCreate}
        onCancel={onClose}
        submitLabel="Erstellen"
        stickyActions
      />
    </Modal>
  );
}
