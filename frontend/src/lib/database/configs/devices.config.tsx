// Device Entity Configuration

import { defineEntityConfig } from "../types";
import { databaseThemes } from "@/components/ui/database/themes";
import type { Device } from "@/lib/iot-helpers";
import {
  prepareDeviceForBackend,
  getDeviceTypeDisplayName,
  getDeviceStatusDisplayName,
  getDeviceStatusColor,
  formatLastSeen,
  getDeviceTypeEmoji,
  generateDefaultDeviceName,
} from "@/lib/iot-helpers";

export const devicesConfig = defineEntityConfig<Device>({
  name: {
    singular: "Gerät",
    plural: "Geräte",
  },

  theme: databaseThemes.devices,

  backUrl: "/database",

  api: {
    basePath: "/api/iot",
  },

  form: {
    sections: [
      {
        title: "Geräteinformationen",
        backgroundColor: "bg-yellow-50/30",
        iconPath:
          "M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01",
        columns: 2,
        fields: [
          {
            name: "device_id",
            label: "Geräte-ID",
            type: "text",
            required: true,
            placeholder: "z.B. RFID-001",
            helperText: "Eindeutige Kennung für das Gerät",
          },
          // Gerätetyp ist immer RFID-Leser; Feld in der UI ausgeblendet (Default wird genutzt)
          {
            name: "name",
            label: "Gerätename",
            type: "text",
            placeholder: "z.B. Haupteingang RFID-Leser",
            helperText: "Optionaler Name zur besseren Identifikation",
          },
          {
            name: "status",
            label: "Status",
            type: "select",
            required: true,
            options: [
              { value: "active", label: "Aktiv" },
              { value: "inactive", label: "Inaktiv" },
              { value: "maintenance", label: "Wartung" },
            ],
            helperText:
              "Online/Offline wird automatisch basierend auf der letzten Kommunikation bestimmt",
          },
        ],
      },
    ],

    defaultValues: {
      status: "active" as const,
      device_type: "rfid_reader",
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
      badges: [],
    },

    sections: [
      {
        title: "Geräteinformationen",
        titleColor: "text-yellow-800",
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
            label: "Status",
            value: (device) => (
              <span
                className={`inline-flex rounded-full px-2 py-1 text-xs font-medium ${getDeviceStatusColor(device.status)}`}
              >
                {getDeviceStatusDisplayName(device.status)}
              </span>
            ),
          },
          {
            label: "Zuletzt gesehen",
            value: (device) => formatLastSeen(device.last_seen),
          },
        ],
      },
      {
        title: "Systemdaten",
        titleColor: "text-yellow-700",
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
        titleColor: "text-yellow-700",
        items: [
          {
            label: "API-Schlüssel",
            value: (device) =>
              device.api_key ? (
                <div className="space-y-2">
                  <div className="flex items-center space-x-2">
                    <input
                      type="password"
                      value={device.api_key}
                      readOnly
                      className="flex-1 rounded border border-yellow-200 bg-yellow-50 px-2 py-1 font-mono text-xs"
                      id={`api-key-${device.id}`}
                      onClick={(e) => (e.target as HTMLInputElement).select()}
                    />
                    <button
                      onClick={() => {
                        const input = document.getElementById(
                          `api-key-${device.id}`,
                        ) as HTMLInputElement;
                        const btn = event?.target as HTMLButtonElement;

                        if (input.type === "password") {
                          input.type = "text";
                          btn.textContent = "Verbergen";
                        } else {
                          input.type = "password";
                          btn.textContent = "Anzeigen";
                        }
                      }}
                      className="rounded bg-blue-600 px-2 py-1 text-xs text-white hover:bg-blue-700"
                    >
                      Anzeigen
                    </button>
                    <button
                      onClick={(e) => {
                        void navigator.clipboard.writeText(device.api_key!);
                        // Simple feedback - could be enhanced with toast
                        const btn = e.target as HTMLButtonElement;
                        const originalText = btn.textContent;
                        btn.textContent = "Kopiert!";
                        setTimeout(() => {
                          btn.textContent = originalText;
                        }, 2000);
                      }}
                      className="rounded bg-yellow-600 px-2 py-1 text-xs text-white hover:bg-yellow-700"
                    >
                      Kopieren
                    </button>
                  </div>
                  <div className="rounded-md border border-blue-200 bg-blue-50 p-2">
                    <div className="flex items-start space-x-2">
                      <span className="text-sm text-blue-600">🔐</span>
                      <span className="text-xs text-blue-800">
                        <strong>Sicherheit:</strong> API-Schlüssel ist
                        standardmäßig verborgen. Klicken Sie
                        &quot;Anzeigen&quot; um ihn sichtbar zu machen.
                      </span>
                    </div>
                  </div>
                </div>
              ) : (
                <span className="text-xs text-gray-500">
                  Nur bei Erstellung sichtbar
                </span>
              ),
          },
        ],
      },
    ],
  },

  list: {
    title: "Gerät auswählen",
    description: "Verwalte IoT-Geräte und deren Status",
    searchPlaceholder: "Geräte suchen...",

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
      badges: [
        {
          label: (device) => getDeviceTypeDisplayName(device.device_type),
          color: "bg-blue-100 text-blue-800",
          showWhen: () => true,
        },
      ],
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
