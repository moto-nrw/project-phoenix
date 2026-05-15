"use client";

import { Modal } from "~/components/ui/modal";
import { DatabaseForm } from "~/components/ui/database/database-form";
import { groupsConfig } from "@/lib/database/configs/groups.config";
import type { Group } from "@/lib/group-helpers";

interface Props {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreate: (data: Partial<Group>) => Promise<void>;
}

export function GroupCreateModal({ isOpen, onClose, onCreate }: Props) {
  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={groupsConfig.labels?.createModalTitle ?? "Neue Gruppe"}
    >
      <DatabaseForm
        theme={groupsConfig.theme}
        sections={groupsConfig.form.sections.map((section) => ({
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
        initialData={groupsConfig.form.defaultValues}
        onSubmit={onCreate}
        onCancel={onClose}
        submitLabel="Erstellen"
        stickyActions
      />
    </Modal>
  );
}
