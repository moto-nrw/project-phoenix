"use client";

import { Modal } from "~/components/ui/modal";
import { DatabaseForm } from "~/components/ui/database/database-form";
import { devicesConfig } from "@/lib/database/configs/devices.config";
import type { Device } from "@/lib/iot-helpers";

interface Props {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreate: (data: Partial<Device>) => Promise<void>;
}

export function DeviceCreateModal({ isOpen, onClose, onCreate }: Props) {
  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={devicesConfig.labels?.createModalTitle ?? "Neues Gerät"}
    >
      <DatabaseForm
        theme={devicesConfig.theme}
        sections={devicesConfig.form.sections.map((section) => ({
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
        initialData={devicesConfig.form.defaultValues}
        onSubmit={onCreate}
        onCancel={onClose}
        submitLabel="Erstellen"
        stickyActions
      />
    </Modal>
  );
}
