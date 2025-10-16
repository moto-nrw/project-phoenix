"use client";

import { Modal } from "~/components/ui/modal";
import { DatabaseForm } from "~/components/ui/database/database-form";
import { permissionsConfig } from "@/lib/database/configs/permissions.config";
import type { Permission } from "@/lib/auth-helpers";

interface Props {
  isOpen: boolean;
  onClose: () => void;
  onCreate: (data: Partial<Permission>) => Promise<void>;
  loading?: boolean;
}

export function PermissionCreateModal({ isOpen, onClose, onCreate, loading = false }: Props) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} title={permissionsConfig.labels?.createModalTitle ?? 'Neue Berechtigung'}>
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
          sections={permissionsConfig.form.sections.map(section => ({
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
          initialData={permissionsConfig.form.defaultValues}
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
