'use client';

// Role Entity Configuration

import { defineEntityConfig } from '../types';
import { databaseThemes } from '@/components/ui/database/themes';
import type { Role, BackendRole } from '@/lib/auth-helpers';
import { mapRoleResponse } from '@/lib/auth-helpers';

export const rolesConfig = defineEntityConfig<Role>({
  name: {
    singular: 'Rolle',
    plural: 'Rollen'
  },
  
  theme: databaseThemes.roles, // Will add this theme next
  
  api: {
    basePath: '/api/auth/roles',
  },
  
  form: {
    sections: [
      {
        title: 'Rolleninformationen',
        backgroundColor: 'bg-gray-50',
        columns: 1,
        fields: [
          {
            name: 'name',
            label: 'Name',
            type: 'text',
            required: true,
            placeholder: 'z.B. Pädagogische Fachkraft, Administrator',
          },
          {
            name: 'description',
            label: 'Beschreibung',
            type: 'textarea',
            required: false,
            placeholder: 'Beschreiben Sie die Aufgaben und Verantwortlichkeiten dieser Rolle',
          },
        ],
      },
    ],
    
    defaultValues: {},
  },
  
  detail: {
    header: {
      title: (role: Role) => role.name,
      subtitle: (role: Role) => role.description || 'Keine Beschreibung',
      avatar: {
        text: (role: Role) => role.name?.[0] ?? 'R',
        size: 'lg',
      },
      badges: [
        {
          label: (role: Role) => `${role.permissions?.length ?? 0} Berechtigungen`,
          color: 'bg-blue-400/80',
          showWhen: () => true,
        },
      ],
    },
    
    sections: [
      {
        title: 'Rolleninformationen',
        titleColor: 'text-gray-800',
        items: [
          {
            label: 'Name',
            value: (role: Role) => role.name,
          },
          {
            label: 'Beschreibung',
            value: (role: Role) => role.description || 'Keine Beschreibung',
          },
          {
            label: 'Berechtigungen',
            value: (role: Role) => `${role.permissions?.length ?? 0} Berechtigungen zugewiesen`,
          },
          {
            label: 'Erstellt am',
            value: (role: Role) => new Date(role.createdAt).toLocaleDateString('de-DE'),
          },
          {
            label: 'Aktualisiert am',
            value: (role: Role) => new Date(role.updatedAt).toLocaleDateString('de-DE'),
          },
        ],
      },
    ],
  },
  
  list: {
    title: 'Rollen verwalten',
    description: 'Verwalten Sie Systemrollen und deren Berechtigungen',
    searchPlaceholder: 'Rollen durchsuchen...',
    
    // Frontend search configuration
    searchStrategy: 'frontend',
    searchableFields: ['name', 'description'],
    minSearchLength: 0,
    
    item: {
      title: (role: Role) => role.name,
      subtitle: (role: Role) => role.description || 'Keine Beschreibung',
      avatar: {
        text: (role: Role) => role.name?.[0] ?? 'R',
      },
      badges: [],
    },
  },
  
  service: {
    mapResponse: (data: unknown): Role => {
      // Handle wrapped response format
      let actualData = data;
      if (data && typeof data === 'object' && 'status' in data && 'data' in data) {
        actualData = (data as { status: string; data: unknown }).data;
      }
      
      return mapRoleResponse(actualData as BackendRole);
    },
  },
  
  labels: {
    createButton: 'Neue Rolle erstellen',
    createModalTitle: 'Neue Rolle',
    editModalTitle: 'Rolle bearbeiten',
    detailModalTitle: 'Rollendetails',
    deleteConfirmation: 'Sind Sie sicher, dass Sie diese Rolle löschen möchten? Alle Benutzer mit dieser Rolle verlieren ihre Berechtigungen.',
  },
});