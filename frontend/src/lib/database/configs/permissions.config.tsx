'use client';

// Permission Entity Configuration

import { defineEntityConfig } from '../types';
import type { Permission, BackendPermission } from '@/lib/auth-helpers';
import { mapPermissionResponse } from '@/lib/auth-helpers';

// Resource descriptions
const resourceDescriptions: Record<string, string> = {
  'users': 'Verwaltung von Benutzerkonten, Rollen und Zugriffsrechten',
  'roles': 'Rollenverwaltung, Berechtigungen und Authentifizierung',
  'permissions': 'Rollenverwaltung, Berechtigungen und Authentifizierung',
  'auth': 'Rollenverwaltung, Berechtigungen und Authentifizierung',
  'activities': 'AGs und Nachmittagsaktivitäten für Schüler',
  'rooms': 'Räume und Klassenzimmer im Schulgebäude',
  'groups': 'Schulklassen und Schülergruppen',
  'visits': 'Anwesenheitsverfolgung von Schülern in Räumen',
  'schedules': 'Zeitpläne und Terminverwaltung',
  'config': 'Systemkonfiguration und Einstellungen',
  'feedback': 'Rückmeldungen und Kommentare',
  'iot': 'RFID-Leser und IoT-Geräte',
  'system': 'Systemverwaltung und Administratorrechte',
  'admin': 'Systemverwaltung und Administratorrechte',
};

// Action descriptions
const actionDescriptions: Record<string, string> = {
  'create': 'Neue Einträge erstellen',
  'read': 'Daten anzeigen und einsehen',
  'update': 'Bestehende Daten bearbeiten',
  'delete': 'Einträge löschen',
  'list': 'Übersichten und Listen abrufen',
  'manage': 'Volle Kontrolle über die Resource',
  'assign': 'Zuweisungen vornehmen (z.B. Schüler zu Gruppen)',
  'enroll': 'Schüler in Aktivitäten einschreiben',
  '*': 'Alle Aktionen (Administratorrechte)',
};

// Add permissions theme to the themes file
const permissionsTheme = {
  primary: 'amber-500',
  secondary: 'orange-600',
  accent: 'amber',
  background: 'amber-50',
  border: 'amber-200',
  textAccent: 'amber-800',
  icon: (
    <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
    </svg>
  ),
  avatarGradient: 'from-amber-400 to-orange-500'
};

export const permissionsConfig = defineEntityConfig<Permission>({
  name: {
    singular: 'Berechtigung',
    plural: 'Berechtigungen'
  },
  
  theme: permissionsTheme,
  
  backUrl: '/database',
  
  api: {
    basePath: '/api/auth/permissions',
  },
  
  // No form section - permissions are read-only
  form: {
    sections: [],
    defaultValues: {},
  },
  
  detail: {
    header: {
      title: (permission: Permission) => permission.name,
      subtitle: (permission: Permission) => permission.description,
      avatar: {
        text: (permission: Permission) => {
          // Use first letter of resource
          return permission.resource[0]?.toUpperCase() ?? 'P';
        },
        size: 'lg',
      },
      badges: [
        {
          label: (permission: Permission) => permission.resource,
          color: 'bg-amber-400/80',
          showWhen: () => true,
        },
        {
          label: (permission: Permission) => permission.action,
          color: 'bg-orange-400/80',
          showWhen: () => true,
        },
      ],
    },
    
    sections: [
      {
        title: 'Berechtigungsdetails',
        titleColor: 'text-amber-800',
        items: [
          {
            label: 'Name',
            value: (permission: Permission) => permission.name,
          },
          {
            label: 'Beschreibung',
            value: (permission: Permission) => permission.description,
          },
          {
            label: 'Resource',
            value: (permission: Permission) => (
              <div>
                <div className="font-medium">{permission.resource}</div>
                <div className="text-sm text-gray-600 mt-1">
                  {resourceDescriptions[permission.resource] ?? 'Keine Beschreibung verfügbar'}
                </div>
              </div>
            ),
          },
          {
            label: 'Action',
            value: (permission: Permission) => (
              <div>
                <div className="font-medium">{permission.action}</div>
                <div className="text-sm text-gray-600 mt-1">
                  {actionDescriptions[permission.action] ?? 'Keine Beschreibung verfügbar'}
                </div>
              </div>
            ),
          },
        ],
      },
      {
        title: 'Erklärung',
        titleColor: 'text-amber-800',
        items: [
          {
            label: 'Was bedeutet diese Berechtigung?',
            value: (permission: Permission) => (
              <div className="space-y-3">
                <div className="bg-amber-50 p-4 rounded-lg">
                  <h4 className="font-semibold text-amber-900 mb-2">Resource: {permission.resource}</h4>
                  <p className="text-amber-800 text-sm">
                    {resourceDescriptions[permission.resource] ?? 'Keine Beschreibung verfügbar'}
                  </p>
                </div>
                <div className="bg-orange-50 p-4 rounded-lg">
                  <h4 className="font-semibold text-orange-900 mb-2">Action: {permission.action}</h4>
                  <p className="text-orange-800 text-sm">
                    {actionDescriptions[permission.action] ?? 'Keine Beschreibung verfügbar'}
                  </p>
                </div>
                <div className="bg-gray-50 p-4 rounded-lg">
                  <h4 className="font-semibold text-gray-900 mb-2">Zusammenfassung</h4>
                  <p className="text-gray-700 text-sm">
                    Diese Berechtigung erlaubt es, die Aktion &quot;{actionDescriptions[permission.action] ?? permission.action}&quot; 
                    auf der Resource &quot;{permission.resource}&quot; auszuführen.
                  </p>
                </div>
              </div>
            ),
            colSpan: 2,
          },
        ],
      },
    ],
    
    actions: {
      edit: false, // No editing permissions
      delete: false, // No deleting permissions
    },
  },
  
  list: {
    title: 'System-Berechtigungen',
    description: 'Übersicht aller Systemberechtigungen',
    searchPlaceholder: 'Berechtigungen suchen...',
    
    // Info section
    infoSection: {
      title: 'Über System-Berechtigungen',
      content: 'Diese Seite zeigt alle im System definierten Berechtigungen. Diese sind fest im Backend implementiert und können nicht über die Benutzeroberfläche geändert werden. Berechtigungen werden Rollen zugewiesen, die wiederum Benutzern zugeordnet werden können.',
      icon: (
        <svg className="h-4 w-4 md:h-5 md:w-5 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      ),
    },
    
    // Frontend search configuration
    searchStrategy: 'frontend',
    searchableFields: ['name', 'description', 'resource', 'action'],
    minSearchLength: 0,
    
    // Custom filters for resource and action
    filters: [
      {
        id: 'resource',
        label: 'Resource',
        type: 'select',
        options: 'dynamic', // Will be populated from data
      },
      {
        id: 'action',
        label: 'Action',
        type: 'select',
        options: 'dynamic', // Will be populated from data
      },
    ],
    
    item: {
      title: (permission: Permission) => permission.name,
      subtitle: (permission: Permission) => permission.description,
      description: (permission: Permission) => `${permission.resource} → ${permission.action}`,
      avatar: {
        text: (permission: Permission) => permission.resource[0]?.toUpperCase() ?? 'P',
      },
      badges: [],
    },
    
    // Explicitly disable create functionality
    features: {
      create: false,
      search: true,
      pagination: true,
      filters: true,
    },
  },
  
  service: {
    mapResponse: (data: unknown): Permission => {
      // Handle wrapped response format
      let actualData = data;
      if (data && typeof data === 'object' && 'status' in data && 'data' in data) {
        actualData = (data as { data: unknown }).data;
      }
      
      return mapPermissionResponse(actualData as BackendPermission);
    },
    
    // Override standard CRUD methods to disable create/update/delete
    create: undefined,
    update: undefined,
    delete: undefined,
    
    // Custom getOne that filters from the list since there's no individual GET endpoint
    customMethods: {
      getOne: async (id?: string): Promise<Permission> => {
        if (!id) {
          throw new Error('Permission ID is required');
        }
        try {
          // Import auth service dynamically to avoid circular dependencies
          const { authService } = await import('@/lib/auth-service');
          
          // Get all permissions
          const permissions = await authService.getPermissions();
          
          // Find the permission by id
          const permission = permissions.find(p => p.id === id);
          
          if (!permission) {
            throw new Error(`Permission with ID ${id} not found`);
          }
          
          return permission;
        } catch (error) {
          console.error('Error fetching permission:', error);
          throw error;
        }
      }
    },
  },
  
  labels: {
    createButton: '', // No create button
    createModalTitle: '',
    editModalTitle: '',
    detailModalTitle: 'Berechtigungsdetails',
    deleteConfirmation: '',
    emptyState: 'Keine Berechtigungen gefunden',
  },
});