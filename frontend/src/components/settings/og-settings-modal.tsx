"use client";

import { Modal } from "~/components/ui/modal";
import { OGSettingsPanel } from "./og-settings-panel";

interface OGSettingsModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly ogId: string;
  readonly ogName: string;
}

export function OGSettingsModal({
  isOpen,
  onClose,
  ogId,
  ogName,
}: OGSettingsModalProps) {
  return (
    <Modal isOpen={isOpen} onClose={onClose} title={`Einstellungen: ${ogName}`}>
      <div className="max-h-[70vh] overflow-y-auto">
        <OGSettingsPanel ogId={ogId} showHistory />
      </div>
    </Modal>
  );
}
