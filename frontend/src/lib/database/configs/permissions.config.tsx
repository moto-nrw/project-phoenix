'use client';

// Permission Entity Configuration

import { defineEntityConfig } from '../types';
import { databaseThemes } from '@/components/ui/database/themes';
import type { Permission, BackendPermission } from '@/lib/auth-helpers';
import { mapPermissionResponse } from '@/lib/auth-helpers';

// Localized labels for resources/actions to present German UI strings
const resourceLabels: Record<string, string> = {
  users: 'Benutzer',
  roles: 'Rollen',
  permissions: 'Berechtigungen',
  activities: 'Aktivitäten',
  rooms: 'Räume',
  groups: 'Gruppen',
  visits: 'Besuche',
  schedules: 'Zeitpläne',
  config: 'Konfiguration',
  feedback: 'Feedback',
  iot: 'Geräte',
  system: 'System',
  admin: 'Administration',
};
const actionLabels: Record<string, string> = {
  create: 'Erstellen',
  read: 'Ansehen',
  update: 'Bearbeiten',
  delete: 'Löschen',
  list: 'Auflisten',
  manage: 'Verwalten',
  assign: 'Zuweisen',
  enroll: 'Anmelden',
  '*': 'Alle',
};

function displayName(p: Permission) {
  const res = resourceLabels[p.resource] ?? p.resource;
  const act = actionLabels[p.action] ?? p.action;
  return `${res}: ${act}`;
}

export const permissionsConfig = defineEntityConfig<Permission>({
  name: {
    singular: 'Berechtigung',
    plural: 'Berechtigungen'
  },

  theme: databaseThemes.permissions,

  backUrl: '/database',

  api: {
    basePath: '/api/auth/permissions',
  },

  form: {
    sections: [
      {
        title: 'Berechtigungsdetails',
        backgroundColor: 'bg-pink-50/30',
        // Adjustments (horizontal sliders) to symbolize toggles/permissions
        iconPath: 'M5 13l4 4L19 7',
        columns: 2,
        fields: [
          {
            name: 'name',
            label: 'Anzeigename',
            type: 'text',
            required: true,
            placeholder: 'z.B. Benutzer erstellen',
            colSpan: 2,
          },
          {
            name: 'resource',
            label: 'Ressource',
            type: 'text',
            required: true,
            placeholder: 'z.B. users, roles, groups',
          },
          {
            name: 'action',
            label: 'Aktion',
            type: 'text',
            required: true,
            placeholder: 'z.B. create, read, update, delete',
          },
          {
            name: 'description',
            label: 'Beschreibung',
            type: 'textarea',
            required: false,
            placeholder: 'Kurze Beschreibung, was diese Berechtigung erlaubt',
            colSpan: 2,
          },
        ],
      },
    ],

    defaultValues: {},
  },

  detail: {
    header: {
      title: (p: Permission) => displayName(p),
      subtitle: (p: Permission) => p.description || 'Keine Beschreibung',
      avatar: {
        text: (p: Permission) => (p.resource?.[0] ?? 'P').toUpperCase(),
        size: 'lg',
      },
      badges: [
        {
          label: (p: Permission) => p.name || 'Systemberechtigung',
          color: 'bg-indigo-400/80',
          showWhen: () => true,
        },
      ],
    },
    sections: [
      {
        title: 'Technische Daten',
        titleColor: 'text-indigo-800',
        items: [
          { label: 'Ressource', value: (p: Permission) => p.resource },
          { label: 'Aktion', value: (p: Permission) => p.action },
          { label: 'Anzeigename', value: (p: Permission) => p.name },
          { label: 'Beschreibung', value: (p: Permission) => p.description || 'Keine Beschreibung' },
          { label: 'ID', value: (p: Permission) => p.id },
          { label: 'Erstellt am', value: (p: Permission) => new Date(p.createdAt).toLocaleDateString('de-DE') },
          { label: 'Aktualisiert am', value: (p: Permission) => new Date(p.updatedAt).toLocaleDateString('de-DE') },
        ],
      },
    ],
  },

  list: {
    title: 'Berechtigungen verwalten',
    description: 'Systemweite Berechtigungen definieren und prüfen',
    searchPlaceholder: 'Berechtigungen durchsuchen...',
    
    searchStrategy: 'frontend',
    searchableFields: ['name', 'description', 'resource', 'action'],
    minSearchLength: 0,

    item: {
      title: (p: Permission) => displayName(p),
      subtitle: (p: Permission) => p.name,
      description: (p: Permission) => p.description || '',
      avatar: {
        text: (p: Permission) => (p.resource?.[0] ?? 'P').toUpperCase(),
      },
      badges: [],
    },
  },

  service: {
    mapResponse: (data: unknown): Permission => {
      let actual = data;
      if (data && typeof data === 'object' && 'status' in data && 'data' in data) {
        actual = (data as { status: string; data: unknown }).data;
      }
      return mapPermissionResponse(actual as BackendPermission);
    },
  },

  labels: {
    createButton: 'Neue Berechtigung erstellen',
    createModalTitle: 'Neue Berechtigung',
    editModalTitle: 'Berechtigung bearbeiten',
    detailModalTitle: 'Berechtigungsdetails',
    deleteConfirmation: 'Diese Berechtigung wirklich löschen? Dies kann bestehende Rollen betreffen.',
  },
});
