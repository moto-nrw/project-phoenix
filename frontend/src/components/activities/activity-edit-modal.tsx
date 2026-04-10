"use client";

import { useMemo } from "react";
import { Modal } from "~/components/ui/modal";
import { DatabaseForm } from "~/components/ui/database/database-form";
import { isSystemActivity, type Activity } from "@/lib/activity-helpers";
import { activitiesConfig } from "@/lib/database/configs/activities.config";
import { configToFormSection } from "@/lib/database/types";

interface ActivityEditModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly activity: Activity | null;
  readonly onSave: (data: Partial<Activity>) => Promise<void>;
  readonly loading?: boolean;
}

export function ActivityEditModal({
  isOpen,
  onClose,
  activity,
  onSave,
  loading = false,
}: ActivityEditModalProps) {
  const isSystem = isSystemActivity(activity);

  // Disable name field for system activities (Schulhof Freispiel, WC)
  const sections = useMemo(() => {
    const baseSections =
      activitiesConfig.form.sections.map(configToFormSection);
    if (!isSystem) return baseSections;

    return baseSections.map((section) => ({
      ...section,
      fields: section.fields.map((field) =>
        field.name === "name"
          ? {
              ...field,
              disabled: true,
              helperText: "Systemaktivität — Name kann nicht geändert werden",
            }
          : field,
      ),
    }));
  }, [isSystem]);

  if (!activity) return null;

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={activitiesConfig.labels?.editModalTitle ?? "Aktivität bearbeiten"}
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
          sections={sections}
          initialData={activity}
          onSubmit={onSave}
          onCancel={onClose}
          isLoading={loading}
          submitLabel="Speichern"
          stickyActions
        />
      )}
    </Modal>
  );
}
