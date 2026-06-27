"use client";

import { devicesConfig } from "@/lib/database/configs/devices.config";
import type { Device } from "@/lib/iot-helpers";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";

interface Props {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly device: Device | null;
  readonly onSave: (data: Partial<Device>) => Promise<void>;
  readonly loading?: boolean;
}

export function DeviceEditModal({
  isOpen,
  onClose,
  device,
  onSave,
  loading = false,
}: Props) {
  if (!device) return null;
  return (
    <DatabaseFormModal<Device>
      isOpen={isOpen}
      onClose={onClose}
      title={devicesConfig.labels?.editModalTitle ?? "Gerät bearbeiten"}
      config={devicesConfig}
      initialData={device}
      onSubmit={onSave}
      isLoading={loading}
      submitLabel="Speichern"
      stickyActions
    />
  );
}
