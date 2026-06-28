"use client";

import { devicesConfig } from "@/components/database/configs/devices.config";
import type { Device } from "@/lib/iot-helpers";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";

interface Props {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreate: (data: Partial<Device>) => Promise<void>;
}

export function DeviceCreateModal({ isOpen, onClose, onCreate }: Props) {
  return (
    <DatabaseFormModal<Device>
      isOpen={isOpen}
      onClose={onClose}
      title={devicesConfig.labels?.createModalTitle ?? "Neues Gerät"}
      config={devicesConfig}
      onSubmit={onCreate}
      submitLabel="Erstellen"
      stickyActions
    />
  );
}
