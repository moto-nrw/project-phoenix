// Device Entity Configuration

import { defineEntityConfig } from '../types';
import { databaseThemes } from '@/components/ui/database/themes';
import type { Device } from '@/lib/iot-helpers';
import { 
  prepareDeviceForBackend, 
  getDeviceTypeDisplayName,
  getDeviceStatusDisplayName,
  getDeviceStatusColor,
  formatLastSeen,
  getDeviceTypeEmoji,
  generateDefaultDeviceName
} from '@/lib/iot-helpers';

export const devicesConfig = defineEntityConfig<Device>({
  name: {
    singular: 'Gerät',
    plural: 'Geräte'
  },
  
  theme: databaseThemes.devices,
  
  api: {
    basePath: '/api/iot',
  },
  
  form: {
    sections: [
      {
        title: 'Geräteinformationen',
        backgroundColor: 'bg-amber-50',
        columns: 2,
        fields: [
          {
            name: 'device_id',
            label: 'Geräte-ID',
            type: 'text',
            required: true,
            placeholder: 'z.B. RFID-001',
            helperText: 'Eindeutige Kennung für das Gerät',
          },
          {
            name: 'device_type',
            label: 'Gerätetyp',
            type: 'select',
            required: true,
            options: [
              { value: 'rfid_reader', label: 'RFID-Leser' },
            ],
          },
          {
            name: 'name',
            label: 'Gerätename',
            type: 'text',
            placeholder: 'z.B. Haupteingang RFID-Leser',
            helperText: 'Optionaler Name zur besseren Identifikation',
          },
          {
            name: 'status',
            label: 'Status',
            type: 'select',
            required: true,
            options: [
              { value: 'active', label: 'Aktiv' },
              { value: 'inactive', label: 'Inaktiv' },
              { value: 'maintenance', label: 'Wartung' },
            ],
            helperText: 'Online/Offline wird automatisch basierend auf der letzten Kommunikation bestimmt',
          },
        ],
      },
    ],
    
    defaultValues: {
      status: 'active' as const,
      device_type: 'rfid_reader',
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
        text: (device) => getDeviceTypeEmoji(device.device_type),
        size: 'lg',
      },
      badges: [
        {
          label: (device: Device) => getDeviceStatusDisplayName(device.status),
          color: 'bg-blue-100 text-blue-800',
          showWhen: () => true,
        },
        {
          label: (device: Device) => device.is_online ? 'Online' : 'Offline',
          color: 'bg-green-100 text-green-800',
          showWhen: (device: Device) => device.status === 'active', // Only show online/offline for active devices
        },
      ],
    },
    
    sections: [
      {
        title: 'Geräteinformationen',
        titleColor: 'text-amber-800',
        items: [
          {
            label: 'Geräte-ID',
            value: (device) => device.device_id,
          },
          {
            label: 'Typ',
            value: (device) => getDeviceTypeDisplayName(device.device_type),
          },
          {
            label: 'Name',
            value: (device) => device.name ?? 'Nicht gesetzt',
          },
          {
            label: 'Status',
            value: (device) => (
              <span className={`inline-flex px-2 py-1 text-xs font-medium rounded-full ${getDeviceStatusColor(device.status)}`}>
                {getDeviceStatusDisplayName(device.status)}
              </span>
            ),
          },
          {
            label: 'Zuletzt gesehen',
            value: (device) => formatLastSeen(device.last_seen),
          },
        ],
      },
      {
        title: 'Systemdaten',
        titleColor: 'text-amber-700',
        columns: 2,
        items: [
          {
            label: 'Erstellt am',
            value: (device) => new Date(device.created_at).toLocaleString('de-DE'),
          },
          {
            label: 'Aktualisiert am',
            value: (device) => new Date(device.updated_at).toLocaleString('de-DE'),
          },
        ],
      },
      {
        title: 'API-Schlüssel',
        titleColor: 'text-amber-700',
        items: [
          {
            label: 'API-Schlüssel',
            value: (device) => device.api_key ? (
              <div className="space-y-2">
                <div className="flex items-center space-x-2">
                  <input 
                    type="password" 
                    value={device.api_key} 
                    readOnly 
                    className="font-mono text-xs bg-amber-50 border border-amber-200 px-2 py-1 rounded flex-1"
                    id={`api-key-${device.id}`}
                    onClick={(e) => (e.target as HTMLInputElement).select()}
                  />
                  <button 
                    onClick={() => {
                      const input = document.getElementById(`api-key-${device.id}`) as HTMLInputElement;
                      const btn = event?.target as HTMLButtonElement;
                      
                      if (input.type === 'password') {
                        input.type = 'text';
                        btn.textContent = 'Verbergen';
                      } else {
                        input.type = 'password';
                        btn.textContent = 'Anzeigen';
                      }
                    }}
                    className="px-2 py-1 bg-blue-600 text-white text-xs rounded hover:bg-blue-700"
                  >
                    Anzeigen
                  </button>
                  <button 
                    onClick={(e) => {
                      void navigator.clipboard.writeText(device.api_key!);
                      // Simple feedback - could be enhanced with toast
                      const btn = e.target as HTMLButtonElement;
                      const originalText = btn.textContent;
                      btn.textContent = 'Kopiert!';
                      setTimeout(() => {
                        btn.textContent = originalText;
                      }, 2000);
                    }}
                    className="px-2 py-1 bg-amber-600 text-white text-xs rounded hover:bg-amber-700"
                  >
                    Kopieren
                  </button>
                </div>
                <div className="bg-blue-50 border border-blue-200 rounded-md p-2">
                  <div className="flex items-start space-x-2">
                    <span className="text-blue-600 text-sm">🔐</span>
                    <span className="text-xs text-blue-800">
                      <strong>Sicherheit:</strong> API-Schlüssel ist standardmäßig verborgen. 
                      Klicken Sie &quot;Anzeigen&quot; um ihn sichtbar zu machen.
                    </span>
                  </div>
                </div>
              </div>
            ) : (
              <span className="text-xs text-gray-500">Nur bei Erstellung sichtbar</span>
            ),
          },
        ],
      },
    ],
  },
  
  list: {
    title: 'Gerät auswählen',
    description: 'Verwalte IoT-Geräte und deren Status',
    searchPlaceholder: 'Geräte suchen...',
    
    searchStrategy: 'frontend',
    searchableFields: ['device_id', 'device_type', 'name'],
    minSearchLength: 0,
    
    filters: [
      {
        id: 'device_type',
        label: 'Typ',
        type: 'select',
        options: 'dynamic',
      },
      {
        id: 'status',
        label: 'Status',
        type: 'select',
        options: [
          { value: 'active', label: 'Aktiv' },
          { value: 'inactive', label: 'Inaktiv' },
          { value: 'maintenance', label: 'Wartung' },
          { value: 'offline', label: 'Offline' },
        ],
      },
      {
        id: 'is_online',
        label: 'Online',
        type: 'select',
        options: [
          { value: 'true', label: 'Online' },
          { value: 'false', label: 'Offline' },
        ],
      },
    ],
    
    item: {
      title: (device) => device.name ?? device.device_id,
      subtitle: (device) => getDeviceTypeDisplayName(device.device_type),
      description: (device) => `Zuletzt gesehen: ${formatLastSeen(device.last_seen)}`,
      avatar: {
        text: (device) => getDeviceTypeEmoji(device.device_type),
      },
      badges: [
        {
          label: (device) => getDeviceTypeDisplayName(device.device_type),
          color: 'bg-blue-100 text-blue-800',
          showWhen: () => true,
        },
        {
          label: (device: Device) => getDeviceStatusDisplayName(device.status),
          color: 'bg-blue-100 text-blue-800',
          showWhen: () => true,
        },
        {
          label: () => '●',
          color: 'bg-green-500 text-green-500 border-green-500',
          showWhen: (device) => device.is_online,
        },
      ],
    },
  },
  
  service: {
    // mapResponse: removed because API route already handles mapping
    mapRequest: (data: Partial<Device>) => prepareDeviceForBackend(data) as Record<string, unknown>,
  },
  
  onCreateSuccess: (_device: Device) => {
    // The database page will automatically open the detail modal if the device has an API key
    // This callback can be used for additional logic if needed
  },
  
  labels: {
    createButton: 'Neues Gerät registrieren',
    createModalTitle: 'Neues Gerät',
    editModalTitle: 'Gerät bearbeiten',
    detailModalTitle: 'Gerätedetails',
    deleteConfirmation: 'Sind Sie sicher, dass Sie dieses Gerät löschen möchten? Dieser Vorgang kann nicht rückgängig gemacht werden.',
  },
});