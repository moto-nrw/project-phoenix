"use client";

import { Modal } from "~/components/ui/modal";
import { DatabaseForm } from "~/components/ui/database/database-form";
import { devicesConfig } from "@/lib/database/configs/devices.config";
import type { Device } from "@/lib/iot-helpers";

interface Props {
  isOpen: boolean;
  onClose: () => void;
  device: Device | null;
  onSave: (data: Partial<Device>) => Promise<void>;
  loading?: boolean;
}

export function DeviceEditModal({ isOpen, onClose, device, onSave, loading = false }: Props) {
  if (!device) return null;
  return (
    <Modal isOpen={isOpen} onClose={onClose} title={devicesConfig.labels?.editModalTitle ?? 'Gerät bearbeiten'}>
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-amber-500" />
            <p className="text-gray-600">Daten werden geladen...</p>
          </div>
        </div>
      ) : (
        <DatabaseForm
          theme={devicesConfig.theme}
          sections={devicesConfig.form.sections.map(section => ({
            title: section.title,
            subtitle: section.subtitle,
            iconPath: (section as any).iconPath,
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
          initialData={device}
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
