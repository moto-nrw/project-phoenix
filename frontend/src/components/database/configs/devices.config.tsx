// Device Entity Configuration

"use client";

import { useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { defineEntityConfig } from "@/lib/database/types";
import type { Device } from "@/lib/iot-helpers";
import {
  prepareDeviceForBackend,
  getDeviceTypeDisplayName,
  getDeviceStatusDisplayName,
  getDeviceStatusColor,
  formatLastSeen,
  formatRelativeLastSeen,
  getDeviceTypeEmoji,
  generateDefaultDeviceName,
  DEVICE_TYPE_OPTIONS,
} from "@/lib/iot-helpers";

/**
 * Der API-Schlüssel eines Geräts: verborgen, mit zwei Schaltflächen aus dem
 * Kit. Vorher zwei bunte Eigenbau-Knöpfe, die ihren Text per DOM-Zugriff
 * umschrieben, und ein blauer Hinweiskasten mit Schloss-Emoji.
 */
function ApiKeyField({ device }: Readonly<{ device: Device }>) {
  const [visible, setVisible] = useState(false);
  const [copied, setCopied] = useState(false);

  if (!device.api_key) {
    return (
      <span className="text-xs text-gray-500">Nur bei Erstellung sichtbar</span>
    );
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        <input
          aria-label={`API-Schlüssel für ${device.name}`}
          type={visible ? "text" : "password"}
          value={device.api_key}
          readOnly
          className="flex-1 rounded-md border border-gray-200 bg-gray-50 px-2 py-1 font-mono text-xs"
          onClick={(event) => event.currentTarget.select()}
        />
        <Button
          type="button"
          variant="outline"
          size="compact"
          onClick={() => setVisible((value) => !value)}
        >
          {visible ? "Verbergen" : "Anzeigen"}
        </Button>
        <Button
          type="button"
          variant="outline"
          size="compact"
          onClick={() => {
            void navigator.clipboard.writeText(device.api_key!);
            setCopied(true);
            setTimeout(() => setCopied(false), 2000);
          }}
        >
          {copied ? "Kopiert" : "Kopieren"}
        </Button>
      </div>
      <Alert
        type="info"
        message={
          'Der Schlüssel ist verborgen. Mit „Anzeigen" machen Sie ihn sichtbar.'
        }
      />
    </div>
  );
}

export const devicesConfig = defineEntityConfig<Device>({
  name: {
    singular: "Gerät",
    plural: "Geräte",
  },

  concept: "devices",

  backUrl: "/database",

  api: {
    basePath: "/api/iot",
  },

  form: {
    sections: [
      {
        title: "Geräteinformationen",
        concept: "devices",
        columns: 2,
        fields: [
          {
            name: "device_id",
            label: "Geräte-ID",
            type: "text",
            required: true,
            placeholder: "z.B. T-001",
            helperText: "Eindeutige Kennung für das Gerät",
          },
          {
            name: "device_type",
            label: "Gerätetyp",
            type: "select",
            required: true,
            options: Object.entries(DEVICE_TYPE_OPTIONS).map(
              ([value, label]) => ({ value, label }),
            ),
            helperText: "Art des Geräts",
          },
          {
            name: "name",
            label: "Gerätename",
            type: "text",
            placeholder: "z.B. Eingangsbereich Terminal",
            helperText: "Optionaler Name zur besseren Identifikation",
          },
        ],
      },
    ],

    defaultValues: {
      device_type: "terminal",
    },

    transformBeforeSubmit: (data) => {
      // Auto-generate name if not provided
      if (!data.name && data.device_id && data.device_type) {
        return {
          ...data,
          name: generateDefaultDeviceName(data.device_type, data.device_id),
        };
      }
      return data;
    },
  },

  detail: {
    header: {
      title: (device) => device.name ?? device.device_id,
      subtitle: (device) => getDeviceTypeDisplayName(device.device_type),
      avatar: {
        text: (device) => device.name?.[0] ?? device.device_id?.[0] ?? "D",
        size: "lg",
      },
    },

    sections: [
      {
        title: "Geräteinformationen",
        items: [
          {
            label: "Geräte-ID",
            value: (device) => device.device_id,
          },
          {
            label: "Typ",
            value: (device) => getDeviceTypeDisplayName(device.device_type),
          },
          {
            label: "Name",
            value: (device) => device.name ?? "Nicht gesetzt",
          },
          {
            label: "Verbindung",
            value: (device) => (
              <span className="inline-flex items-center gap-1.5">
                <span
                  className={`inline-block h-2 w-2 rounded-full ${device.is_online ? "bg-moto-green" : "bg-gray-400"}`}
                />
                <span className="font-medium">
                  {device.is_online ? "Online" : "Offline"}
                </span>
                {!device.is_online && (
                  <span className="text-gray-500">
                    · {formatRelativeLastSeen(device.last_seen)}
                  </span>
                )}
              </span>
            ),
          },
          {
            label: "Letzter Standort",
            value: (device) => device.room_name ?? "Noch nicht verwendet",
          },
          {
            label: "Status",
            value: (device) => (
              <span
                className={`inline-flex rounded-full px-2 py-1 text-xs font-medium ${getDeviceStatusColor(device.status)}`}
              >
                {getDeviceStatusDisplayName(device.status)}
              </span>
            ),
          },
        ],
      },
      {
        title: "Systemdaten",
        columns: 2,
        items: [
          {
            label: "Erstellt am",
            value: (device) =>
              new Date(device.created_at).toLocaleString("de-DE"),
          },
          {
            label: "Aktualisiert am",
            value: (device) =>
              new Date(device.updated_at).toLocaleString("de-DE"),
          },
        ],
      },
      {
        title: "API-Schlüssel",
        items: [
          {
            label: "API-Schlüssel",
            value: (device) => <ApiKeyField device={device} />,
          },
        ],
      },
    ],
  },

  list: {
    title: "Gerät auswählen",
    description: "Verwalte IoT-Geräte und deren Status",
    searchPlaceholder: "Geräte suchen…",

    searchStrategy: "frontend",
    searchableFields: ["device_id", "device_type", "name"],
    minSearchLength: 0,

    filters: [
      {
        id: "device_type",
        label: "Typ",
        type: "select",
        options: "dynamic",
      },
      {
        id: "status",
        label: "Status",
        type: "select",
        options: [
          { value: "active", label: "Aktiv" },
          { value: "inactive", label: "Inaktiv" },
          { value: "maintenance", label: "Wartung" },
          { value: "offline", label: "Offline" },
        ],
      },
      {
        id: "is_online",
        label: "Online",
        type: "select",
        options: [
          { value: "true", label: "Online" },
          { value: "false", label: "Offline" },
        ],
      },
    ],

    item: {
      title: (device) => device.name ?? device.device_id,
      subtitle: (device) => getDeviceTypeDisplayName(device.device_type),
      description: (device) =>
        `Zuletzt gesehen: ${formatLastSeen(device.last_seen)}`,
      avatar: {
        text: (device) => getDeviceTypeEmoji(device.device_type),
      },
    },
  },

  service: {
    // mapResponse: removed because API route already handles mapping
    mapRequest: (data: Partial<Device>) =>
      prepareDeviceForBackend(data) as Record<string, unknown>,
  },

  onCreateSuccess: (_device: Device) => {
    // The database page will automatically open the detail modal if the device has an API key
    // This callback can be used for additional logic if needed
  },

  labels: {
    createButton: "Neues Gerät registrieren",
    createModalTitle: "Neues Gerät",
    editModalTitle: "Gerät bearbeiten",
    detailModalTitle: "Gerätedetails",
    deleteConfirmation:
      "Sind Sie sicher, dass Sie dieses Gerät löschen möchten? Dieser Vorgang kann nicht rückgängig gemacht werden.",
  },
});
