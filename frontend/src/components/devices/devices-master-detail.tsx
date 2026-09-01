"use client";

import { Pencil } from "lucide-react";
import { useState } from "react";
import { DatabaseDetailHeader } from "~/components/database/database-detail-header";
import { DatabaseListItem } from "~/components/database/database-list-item";
import { DetailDeleteButton } from "~/components/database/detail-delete-button";
import {
  DetailPanel,
  type DetailTab,
} from "~/components/database/detail-panel";
import { EmptyDetailState } from "~/components/database/empty-detail-state";
import {
  GroupedList,
  type GroupDefinition,
} from "~/components/database/grouped-list";
import { MasterDetailLayout } from "~/components/database/master-detail-layout";
import {
  DataField,
  DataGrid,
  InfoSection,
} from "~/components/ui/detail-modal-components";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import {
  formatRelativeLastSeen,
  getDeviceStatusColor,
  getDeviceStatusDisplayName,
  getDeviceTypeDisplayName,
  type Device,
} from "@/lib/iot-helpers";
import { useClipboardCopy } from "~/lib/use-clipboard-copy";

interface DevicesMasterDetailProps {
  groupDefinitions: GroupDefinition<Device>[];
  selectedId: string | null;
  selectedDevice: Device | null;
  onSelect: (id: string | null) => void;
  onEditClick: () => void;
  onDeleteClick: () => void;
}

function keyForDevice(device: Device): string {
  return device.id;
}

function getDeviceTitle(device: Device): string {
  return device.name?.trim() || device.device_id;
}

function buildDeviceSubtitle(device: Device): string {
  const parts: string[] = [getDeviceTypeDisplayName(device.device_type)];
  if (device.room_name) parts.push(device.room_name);
  return parts.join(" · ");
}

export function DevicesMasterDetail({
  groupDefinitions,
  selectedId,
  selectedDevice,
  onSelect,
  onEditClick,
  onDeleteClick,
}: DevicesMasterDetailProps) {
  const renderItem = (device: Device) => (
    <DatabaseListItem
      title={getDeviceTitle(device)}
      subtitle={buildDeviceSubtitle(device)}
      isSelected={selectedId === device.id}
      onSelect={() => onSelect(device.id)}
      trailingAccessory={
        <span
          aria-label={device.is_online ? "Online" : "Offline"}
          className={`mr-1 h-2 w-2 shrink-0 rounded-full ${device.is_online ? "bg-moto-green" : "bg-gray-400"}`}
        />
      }
    />
  );

  const listNode = (
    <GroupedList
      groups={groupDefinitions}
      renderItem={renderItem}
      keyFor={keyForDevice}
      emptyState={
        <div className="text-center text-sm text-gray-500">
          Keine Geräte gefunden.
        </div>
      }
    />
  );

  const detailNode = selectedDevice ? (
    <DeviceDetailContent
      device={selectedDevice}
      onEditClick={onEditClick}
      onDeleteClick={onDeleteClick}
    />
  ) : (
    <EmptyDetailState
      title="Kein Gerät ausgewählt"
      description="Wähle links ein Gerät, um die Details zu sehen."
    />
  );

  return (
    <MasterDetailLayout
      list={listNode}
      detail={detailNode}
      selectedId={selectedId}
      onDeselect={() => onSelect(null)}
      unselectedBehavior="expand"
      mobileDrawerTitle={
        selectedDevice ? getDeviceTitle(selectedDevice) : "Gerät"
      }
    />
  );
}

interface DeviceDetailContentProps {
  device: Device;
  onEditClick: () => void;
  onDeleteClick: () => void;
}

function DeviceDetailContent({
  device,
  onEditClick,
  onDeleteClick,
}: DeviceDetailContentProps) {
  const [activeTab, setActiveTab] = useState<string>("master-data");

  const headerActions = (
    <>
      <button
        type="button"
        onClick={onEditClick}
        className="flex items-center gap-1.5 rounded-md border border-gray-200 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50"
      >
        <Pencil className="h-3.5 w-3.5" aria-hidden />
        Bearbeiten
      </button>
      <DetailDeleteButton onClick={onDeleteClick} />
    </>
  );

  const tabs: DetailTab[] = [
    {
      id: "master-data",
      label: "Stammdaten",
      content: <DeviceStammdatenTab device={device} />,
    },
  ];

  return (
    <DetailPanel
      header={
        <DatabaseDetailHeader
          icon={
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.devices.icon}
              tone={MOTO_CONCEPTS.devices.tone}
              size={36}
            />
          }
          title={getDeviceTitle(device)}
          subtitle={buildDeviceSubtitle(device)}
          actions={headerActions}
        />
      }
      tabs={tabs}
      activeTab={activeTab}
      onTabChange={setActiveTab}
    />
  );
}

function DeviceStammdatenTab({ device }: { device: Device }) {
  return (
    <div className="space-y-4">
      <InfoSection
        title="Gerätedetails"
        icon={
          <MotoDuotoneIcon
            icon={MOTO_CONCEPTS.devices.icon}
            tone={MOTO_CONCEPTS.devices.tone}
            size={18}
          />
        }
        accentColor="gray"
      >
        <DataGrid>
          <DataField label="Geräte-ID" mono>
            {device.device_id}
          </DataField>
          <DataField label="Verbindung">
            <span className="inline-flex items-center gap-1.5">
              <span
                className={`inline-block h-2 w-2 rounded-full ${device.is_online ? "bg-moto-green" : "bg-gray-400"}`}
              />
              {device.is_online ? "Online" : "Offline"}
              {!device.is_online ? (
                <span className="text-xs font-normal text-gray-500">
                  · {formatRelativeLastSeen(device.last_seen)}
                </span>
              ) : null}
            </span>
          </DataField>
          <DataField label="Standort">{device.room_name ?? "—"}</DataField>
          <DataField label="Status">
            <span
              className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${getDeviceStatusColor(device.status)}`}
            >
              {getDeviceStatusDisplayName(device.status)}
            </span>
          </DataField>
        </DataGrid>
      </InfoSection>

      {device.api_key ? <ApiKeySection apiKey={device.api_key} /> : null}
    </div>
  );
}

function ApiKeySection({ apiKey }: { apiKey: string }) {
  const [revealed, setRevealed] = useState(false);
  const { copied, copy } = useClipboardCopy("DeviceApiKeySection");

  return (
    <InfoSection
      title="API-Schlüssel (nur einmal sichtbar)"
      icon={
        <MotoDuotoneIcon
          icon={MOTO_CONCEPTS.permissions.icon}
          tone={MOTO_CONCEPTS.permissions.tone}
          size={18}
        />
      }
      accentColor="purple"
    >
      <div className="space-y-2">
        <div className="flex items-center gap-2">
          <input
            aria-label="API-Schlüssel"
            type={revealed ? "text" : "password"}
            value={apiKey}
            readOnly
            // Opt out of password manager / browser autosave prompts; this is
            // a one-shot device key, not a user credential.
            autoComplete="off"
            data-1p-ignore
            data-lpignore="true"
            data-form-type="other"
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
          />
          <button
            type="button"
            onClick={() => setRevealed((current) => !current)}
            className="bg-moto-orange hover:bg-moto-orange-hover rounded px-2 py-1 text-xs text-white"
          >
            {revealed ? "Verbergen" : "Anzeigen"}
          </button>
          <button
            type="button"
            onClick={() => void copy(apiKey)}
            className="rounded bg-gray-900 px-2 py-1 text-xs text-white hover:bg-gray-700"
          >
            {copied ? "Kopiert!" : "Kopieren"}
          </button>
        </div>
        <div className="border-moto-amber/20 bg-moto-amber-soft rounded-md border p-2">
          <div className="text-moto-amber-strong text-xs">
            Sicherheit: Bewahren Sie diesen Schlüssel sicher auf. Er ist nur bei
            der Erstellung sichtbar.
          </div>
        </div>
      </div>
    </InfoSection>
  );
}
