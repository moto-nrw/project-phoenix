'use client';

// Permission Entity Configuration

import { defineEntityConfig } from '../types';
import { databaseThemes } from '@/components/ui/database/themes';
import type { Permission, BackendPermission } from '@/lib/auth-helpers';
import { mapPermissionResponse } from '@/lib/auth-helpers';

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
        backgroundColor: 'bg-indigo-50/30',
        // Small icon in section header
        iconPath: 'M15 7a2 2 0 012 2v1a2 2 0 11-4 0V9a2 2 0 012-2m-6 6h3l3 3 3-3 3 3-7 7-5-5v-2a2 2 0 012-2',
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
      title: (p: Permission) => `${p.resource}: ${p.action}`,
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
      title: (p: Permission) => `${p.resource}: ${p.action}`,
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

