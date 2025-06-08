// Student Entity Configuration

import { defineEntityConfig } from '../types';
import { databaseThemes } from '@/components/ui/database/themes';
import { GroupSelect } from '@/components/ui/database';
import type { Student } from '@/lib/api';
import dynamic from 'next/dynamic';

const PrivacyConsentSection = dynamic(
  () => import('@/components/students/privacy-consent-section'),
  { 
    ssr: false,
    loading: () => <div className="text-gray-500 text-sm">Lade...</div>
  }
);

export const studentsConfig = defineEntityConfig<Student>({
  name: {
    singular: 'Schüler',
    plural: 'Schüler'
  },
  
  theme: databaseThemes.students,
  
  api: {
    basePath: '/api/students',
    // No listParams needed - the API route handles pagination internally
  },
  
  form: {
    sections: [
      {
        title: 'Persönliche Daten',
        backgroundColor: 'bg-blue-50',
        columns: 2,
        fields: [
          {
            name: 'first_name',
            label: 'Vorname',
            type: 'text',
            required: true,
          },
          {
            name: 'second_name',
            label: 'Nachname',
            type: 'text',
            required: true,
          },
          {
            name: 'school_class',
            label: 'Klasse',
            type: 'text',
            required: true,
          },
          {
            name: 'group_id',
            label: 'OGS Gruppe',
            type: 'custom',
            required: true,
            component: (props: { value: unknown; onChange: (value: unknown) => void; label: string; required?: boolean }) => 
              GroupSelect({ 
                name: 'group_id', 
                value: props.value as string, 
                onChange: props.onChange as (value: string) => void 
              }),
          },
        ],
      },
      {
        title: 'Erziehungsberechtigte',
        backgroundColor: 'bg-purple-50',
        columns: 2,
        fields: [
          {
            name: 'name_lg',
            label: 'Name des Erziehungsberechtigten',
            type: 'text',
            required: true,
          },
          {
            name: 'contact_lg',
            label: 'Kontakt des Erziehungsberechtigten',
            type: 'text',
            required: true,
            placeholder: 'E-Mail oder Telefonnummer',
            helperText: 'Bitte E-Mail-Adresse oder Telefonnummer eingeben',
          },
        ],
      },
      {
        title: 'Busfahrer',
        backgroundColor: 'bg-green-50',
        columns: 1,
        fields: [
          {
            name: 'bus',
            label: 'Fährt mit dem Bus',
            type: 'checkbox',
            helperText: 'Aktivieren, wenn der Schüler mit dem Bus fährt',
          },
        ],
      },
      {
        title: 'Datenschutz',
        backgroundColor: 'bg-yellow-50',
        columns: 2,
        fields: [
          {
            name: 'privacy_consent_accepted',
            label: 'Datenschutzeinwilligung erteilt',
            type: 'checkbox',
            helperText: 'Aktivieren, wenn die Einwilligung zur Datenverarbeitung vorliegt',
          },
          {
            name: 'data_retention_days',
            label: 'Aufbewahrungsdauer (Tage)',
            type: 'number',
            min: 1,
            max: 31,
            required: false,
            helperText: 'Anzahl der Tage für die Datenspeicherung (1-31)',
          },
        ],
      },
    ],
    
    defaultValues: {
      in_house: false,
      wc: false,
      school_yard: false,
      bus: false,
    },
    
    transformBeforeSubmit: (data) => {
      // Add computed fields
      return {
        ...data,
        name: `${data.first_name} ${data.second_name}`,
        current_location: 'Home' as const,
      };
    },
  },
  
  detail: {
    header: {
      title: (student) => `${student.first_name} ${student.second_name}`,
      subtitle: (student) => student.school_class,
      avatar: {
        text: (student) => `${student.first_name?.[0] ?? ''}${student.second_name?.[0] ?? ''}`,
        size: 'lg',
      },
      badges: [
        {
          label: 'Im Haus',
          color: 'bg-green-400/80',
          showWhen: (student) => !!student.in_house,
        },
        {
          label: 'Toilette',
          color: 'bg-blue-400/80',
          showWhen: (student) => !!student.wc,
        },
        {
          label: 'Schulhof',
          color: 'bg-yellow-400/80',
          showWhen: (student) => !!student.school_yard,
        },
        {
          label: 'Bus',
          color: 'bg-orange-400/80',
          showWhen: (student) => !!student.bus,
        },
      ],
    },
    
    sections: [
      {
        title: 'Persönliche Daten',
        titleColor: 'text-blue-800',
        items: [
          {
            label: 'Vorname',
            value: (student) => student.first_name,
          },
          {
            label: 'Nachname',
            value: (student) => student.second_name,
          },
          {
            label: 'Klasse',
            value: (student) => student.school_class,
          },
          {
            label: 'Gruppe',
            value: (student) => student.group_name ?? 'Keine Gruppe zugewiesen',
          },
          {
            label: 'IDs',
            value: (student) => (
              <div className="flex flex-col text-xs text-gray-600">
                <span>Student: {student.id}</span>
                {student.custom_users_id && (
                  <span>Benutzer: {student.custom_users_id}</span>
                )}
                {student.group_id && (
                  <span>Gruppe: {student.group_id}</span>
                )}
              </div>
            ),
          },
        ],
      },
      {
        title: 'Erziehungsberechtigte',
        titleColor: 'text-purple-800',
        items: [
          {
            label: 'Name',
            value: (student) => student.name_lg ?? 'Nicht angegeben',
          },
          {
            label: 'Kontakt',
            value: (student) => student.contact_lg ?? 'Nicht angegeben',
          },
        ],
      },
      {
        title: 'Status',
        titleColor: 'text-green-800',
        columns: 2,
        items: [
          {
            label: 'Im Haus',
            value: (student) => (
              <div
                className={`rounded-lg p-2 md:p-3 text-sm ${
                  student.in_house 
                    ? 'bg-green-100 text-green-800' 
                    : 'bg-gray-100 text-gray-500'
                }`}
              >
                <span className="flex items-center">
                  <span
                    className={`mr-2 inline-block h-3 w-3 rounded-full flex-shrink-0 ${
                      student.in_house ? 'bg-green-500' : 'bg-gray-300'
                    }`}
                  />
                  <span className="truncate">Im Haus</span>
                </span>
              </div>
            ),
          },
          {
            label: 'Toilette',
            value: (student) => (
              <div
                className={`rounded-lg p-2 md:p-3 text-sm ${
                  student.wc 
                    ? 'bg-blue-100 text-blue-800' 
                    : 'bg-gray-100 text-gray-500'
                }`}
              >
                <span className="flex items-center">
                  <span
                    className={`mr-2 inline-block h-3 w-3 rounded-full flex-shrink-0 ${
                      student.wc ? 'bg-blue-500' : 'bg-gray-300'
                    }`}
                  />
                  <span className="truncate">Toilette</span>
                </span>
              </div>
            ),
          },
          {
            label: 'Schulhof',
            value: (student) => (
              <div
                className={`rounded-lg p-2 md:p-3 text-sm ${
                  student.school_yard 
                    ? 'bg-yellow-100 text-yellow-800' 
                    : 'bg-gray-100 text-gray-500'
                }`}
              >
                <span className="flex items-center">
                  <span
                    className={`mr-2 inline-block h-3 w-3 rounded-full flex-shrink-0 ${
                      student.school_yard ? 'bg-yellow-500' : 'bg-gray-300'
                    }`}
                  />
                  <span className="truncate">Schulhof</span>
                </span>
              </div>
            ),
          },
          {
            label: 'Bus',
            value: (student) => (
              <div
                className={`rounded-lg p-2 md:p-3 text-sm ${
                  student.bus 
                    ? 'bg-orange-100 text-orange-800' 
                    : 'bg-gray-100 text-gray-500'
                }`}
              >
                <span className="flex items-center">
                  <span
                    className={`mr-2 inline-block h-3 w-3 rounded-full flex-shrink-0 ${
                      student.bus ? 'bg-orange-500' : 'bg-gray-300'
                    }`}
                  />
                  <span className="truncate">Bus</span>
                </span>
              </div>
            ),
          },
        ],
      },
      {
        title: 'Datenverwaltung',
        titleColor: 'text-yellow-800',
        items: [
          {
            label: 'Datenschutzeinstellungen',
            value: (student) => <PrivacyConsentSection studentId={student.id} />,
          },
        ],
      },
    ],
  },
  
  list: {
    title: 'Schüler auswählen',
    description: 'Verwalte Schülerdaten und Gruppenzuweisungen',
    searchPlaceholder: 'Schüler suchen...',
    
    // Frontend search configuration (loads all data at once)
    searchStrategy: 'frontend',
    searchableFields: ['first_name', 'second_name', 'school_class', 'group_name', 'name_lg'],
    minSearchLength: 0, // Start searching immediately
    
    filters: [
      {
        id: 'groupId',
        label: 'Gruppe',
        type: 'select',
        options: 'dynamic', // Will extract from data
      },
      {
        id: 'school_class',
        label: 'Klasse',
        type: 'select',
        options: 'dynamic', // Will extract from data
      },
      {
        id: 'bus',
        label: 'Bus',
        type: 'select',
        options: [
          { value: 'true', label: 'Ja' },
          { value: 'false', label: 'Nein' },
        ],
      },
    ],
    
    item: {
      title: (student: Student) => `${student.first_name} ${student.second_name}`,
      subtitle: (student: Student) => student.name_lg ?? 'Kein Erziehungsberechtigter',
      description: (student: Student) => student.contact_lg ?? '',
      avatar: {
        text: (student: Student) => `${student.first_name?.[0] ?? ''}${student.second_name?.[0] ?? ''}`,
      },
      badges: [
        {
          label: (student: Student) => student.school_class ?? 'Keine Klasse',
          color: 'bg-blue-100 text-blue-700',
          showWhen: (student: Student) => !!student.school_class,
        },
        {
          label: (student: Student) => student.group_name ?? 'Keine Gruppe',
          color: 'bg-purple-100 text-purple-700',
          showWhen: (student: Student) => !!student.group_name,
        },
        {
          field: 'bus',
          label: 'Bus',
          color: 'bg-orange-100 text-orange-700',
          showWhen: (student: Student) => !!student.bus,
        },
      ],
    },
  },
  
  service: {
    // mapResponse handled by API route already - no double mapping needed
    
    mapRequest: (data: Partial<Student>) => ({
      ...data,
      // Backend expects these as numbers
      group_id: data.group_id ? parseInt(data.group_id) : undefined,
    }),
  },
  
  labels: {
    createButton: 'Neuen Schüler erstellen',
    createModalTitle: 'Neuer Schüler',
    editModalTitle: 'Schüler bearbeiten',
    detailModalTitle: 'Schülerdetails',
    deleteConfirmation: 'Sind Sie sicher, dass Sie diesen Schüler löschen möchten?',
  },
});