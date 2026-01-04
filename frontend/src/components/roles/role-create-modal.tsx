"use client";

import { Modal } from "~/components/ui/modal";
import { DatabaseForm } from "~/components/ui/database/database-form";
import { rolesConfig } from "@/lib/database/configs/roles.config";
import type { Role } from "@/lib/auth-helpers";

interface Props {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreate: (data: Partial<Role>) => Promise<void>;
  readonly loading?: boolean;
}

export function RoleCreateModal({
  isOpen,
  onClose,
  onCreate,
  loading = false,
}: Props) {
  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={rolesConfig.labels?.createModalTitle ?? "Neue Rolle"}
    >
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-purple-600" />
            <p className="text-gray-600">Daten werden geladen...</p>
          </div>
        </div>
      ) : (
        <DatabaseForm
          theme={rolesConfig.theme}
          sections={rolesConfig.form.sections.map((section) => ({
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
          initialData={rolesConfig.form.defaultValues}
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
