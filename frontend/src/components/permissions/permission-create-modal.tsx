"use client";

import { Modal } from "~/components/ui/modal";
import { DatabaseForm } from "~/components/ui/database/database-form";
import { permissionsConfig } from "@/lib/database/configs/permissions.config";
import type { Permission } from "@/lib/auth-helpers";

interface Props {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreate: (data: Partial<Permission>) => Promise<void>;
}

export function PermissionCreateModal({ isOpen, onClose, onCreate }: Props) {
  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={permissionsConfig.labels?.createModalTitle ?? "Neue Berechtigung"}
    >
      <DatabaseForm
        theme={permissionsConfig.theme}
        sections={permissionsConfig.form.sections.map((section) => ({
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
        initialData={permissionsConfig.form.defaultValues}
        onSubmit={onCreate}
        onCancel={onClose}
        submitLabel="Erstellen"
        stickyActions
      />
    </Modal>
  );
}
