"use client";

import { Modal } from "~/components/ui/modal";
import { DatabaseForm } from "~/components/ui/database/database-form";
import { groupsConfig } from "@/lib/database/configs/groups.config";
import type { Group } from "@/lib/group-helpers";

interface Props {
  isOpen: boolean;
  onClose: () => void;
  group: Group | null;
  onSave: (data: Partial<Group>) => Promise<void>;
  loading?: boolean;
}

export function GroupEditModal({ isOpen, onClose, group, onSave, loading = false }: Props) {
  if (!group) return null;
  return (
    <Modal isOpen={isOpen} onClose={onClose} title={groupsConfig.labels?.editModalTitle ?? 'Gruppe bearbeiten'}>
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-[#83CD2D]" />
            <p className="text-gray-600">Daten werden geladen...</p>
          </div>
        </div>
      ) : (
        <DatabaseForm
          theme={groupsConfig.theme}
          sections={groupsConfig.form.sections.map(section => ({
            title: section.title,
            subtitle: section.subtitle,
            iconPath: section.iconPath,
            fields: section.fields.map(field => ({
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
          initialData={group}
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
