"use client";

import { Modal } from "~/components/ui/modal";
import { DatabaseForm } from "~/components/ui/database/database-form";
import type { Activity } from "@/lib/activity-helpers";
import { activitiesConfig } from "@/lib/database/configs/activities.config";

interface ActivityCreateModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreate: (data: Partial<Activity>) => Promise<void>;
  loading?: boolean;
}

export function ActivityCreateModal({
  isOpen,
  onClose,
  onCreate,
  loading = false,
}: ActivityCreateModalProps) {
  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={activitiesConfig.labels?.createModalTitle ?? "Neue Aktivität"}
    >
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-[#FF3130]" />
            <p className="text-gray-600">Daten werden geladen...</p>
          </div>
        </div>
      ) : (
        <DatabaseForm
          theme={activitiesConfig.theme}
          sections={activitiesConfig.form.sections.map((section) => ({
            title: section.title,
            subtitle: section.subtitle,
            iconPath: section.iconPath,
            fields: section.fields.map((field) => ({
              name: field.name,
              label: field.label,
              type: field.type,
              required: field.required,
              placeholder: field.placeholder,
              options: field.options,
              validation: field.validation,
              component: field.component,
              helperText: field.helperText,
              autoComplete: field.autoComplete,
              colSpan: field.colSpan,
              min: field.min,
              max: field.max,
            })),
            columns: section.columns,
            backgroundColor: section.backgroundColor,
          }))}
          initialData={activitiesConfig.form.defaultValues}
          onSubmit={onCreate}
          onCancel={onClose}
          isLoading={loading}
          submitLabel="Erstellen"
          stickyActions
        />
      )}
    </Modal>
  );
}
