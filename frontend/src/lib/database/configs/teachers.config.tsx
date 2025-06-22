// Teacher Entity Configuration

import { defineEntityConfig } from '../types';
import { databaseThemes } from '@/components/ui/database/themes';
import type { Teacher, TeacherWithCredentials } from '@/lib/teacher-api';
import { teacherService } from '@/lib/teacher-api';

// Map teacher response from backend to frontend format
function mapTeacherResponse(data: unknown): Teacher {
  const typedData = data as Record<string, unknown>;
  
  // Extract person data if nested
  const person = typedData.person as Record<string, unknown> | undefined;
  
  // Get account_id from either direct data or nested person object
  const accountId = typedData.account_id as number | undefined ?? 
                    person?.account_id as number | undefined;
  
  // Get email from either direct data or nested person object
  const email = typedData.email as string | undefined ?? 
                person?.email as string | undefined;
                
  // Get first and last name from either direct data or nested person object
  const firstName = (typedData.first_name as string | undefined) ?? (person?.first_name as string | undefined) ?? '';
  const lastName = (typedData.last_name as string | undefined) ?? (person?.last_name as string | undefined) ?? '';
  
  // Get tag_id from either direct data or nested person object
  const tagId = typedData.tag_id as string | null | undefined ?? 
                person?.tag_id as string | null | undefined;
  
  // Debug logging to check account_id mapping
  if (process.env.NODE_ENV === 'development') {
    console.log('Teacher mapping debug:', {
      raw_data: typedData,
      person_data: person,
      extracted_account_id: accountId,
      email: email,
    });
  }
  
  return {
    id: typedData.id?.toString() ?? '',
    name: (typedData.name as string) ?? `${firstName} ${lastName}`,
    first_name: firstName,
    last_name: lastName,
    email: email,
    specialization: (typedData.specialization as string) ?? '',
    role: typedData.role as string | null | undefined,
    qualifications: typedData.qualifications as string | null | undefined,
    tag_id: tagId,
    staff_notes: typedData.staff_notes as string | null | undefined,
    created_at: typedData.created_at as string | undefined,
    updated_at: typedData.updated_at as string | undefined,
    person_id: typedData.person_id as number | undefined,
    account_id: accountId,
    is_teacher: typedData.is_teacher as boolean | undefined,
    staff_id: typedData.staff_id as string | undefined,
    teacher_id: typedData.teacher_id as string | undefined,
    person: person,
  };
}

// Prepare teacher data for backend
function prepareTeacherForBackend(data: Partial<Teacher> & { password?: string }): Record<string, unknown> {
  return {
    first_name: data.first_name,
    last_name: data.last_name,
    email: data.email,
    specialization: data.specialization,
    role: data.role,
    qualifications: data.qualifications,
    tag_id: data.tag_id,
    staff_notes: data.staff_notes,
    // Include password if present (for creation)
    ...(data.password && { password: data.password }),
  };
}

export const teachersConfig = defineEntityConfig<Teacher>({
  name: {
    singular: 'Pädagogische Fachkraft',
    plural: 'Pädagogische Fachkräfte'
  },
  
  theme: databaseThemes.teachers,
  
  api: {
    basePath: '/api/staff',
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
            placeholder: 'z.B. Max',
          },
          {
            name: 'last_name',
            label: 'Nachname',
            type: 'text',
            required: true,
            placeholder: 'z.B. Mustermann',
          },
          {
            name: 'email',
            label: 'E-Mail',
            type: 'email',
            placeholder: 'fachkraft@schule.de',
            helperText: 'Wird für die Anmeldung verwendet',
          },
          {
            name: 'tag_id',
            label: 'RFID-Karte',
            type: 'select',
            placeholder: 'RFID-Karte auswählen',
            options: async () => {
              try {
                const response = await fetch('/api/users/rfid-cards/available');
                if (response.ok) {
                  const data = await response.json() as { data?: Array<{ tag_id: string }> } | Array<{ tag_id: string }>;
                  const cards = Array.isArray(data) ? data : (data as Record<string, unknown>).data ?? [];
                  return (cards as Array<{ tag_id: string }>).map((card) => ({
                    value: card.tag_id,
                    label: `RFID: ${card.tag_id}`
                  }));
                }
                return [];
              } catch (error) {
                console.error('Error fetching RFID cards:', error);
                return [];
              }
            },
          },
        ],
      },
      {
        title: 'Berufliche Informationen',
        backgroundColor: 'bg-indigo-50',
        columns: 2,
        fields: [
          {
            name: 'specialization',
            label: 'Fachgebiet',
            type: 'text',
            required: true,
            placeholder: 'z.B. Mathematik, Deutsch',
          },
          {
            name: 'role',
            label: 'Rolle',
            type: 'text',
            placeholder: 'z.B. Gruppenbetreuer, Fachbetreuer',
          },
          {
            name: 'qualifications',
            label: 'Qualifikationen',
            type: 'textarea',
            placeholder: 'Zusätzliche Qualifikationen und Zertifikate',
            colSpan: 2,
          },
          {
            name: 'staff_notes',
            label: 'Interne Notizen',
            type: 'textarea',
            placeholder: 'Notizen nur für Verwaltung sichtbar',
            colSpan: 2,
          },
        ],
      },
      {
        title: 'Zugangsdaten',
        backgroundColor: 'bg-green-50',
        fields: [
          {
            name: 'password',
            label: 'Temporäres Passwort',
            type: 'password' as const,
            required: true,
            placeholder: 'Starkes Passwort erstellen',
            helperText: 'Die pädagogische Fachkraft sollte das Passwort bei der ersten Anmeldung ändern.',
          },
        ],
      },
    ],
    
    defaultValues: {
      specialization: '',
      role: '',
    },
    
    validation: (data: Record<string, unknown>) => {
      const errors: Record<string, string> = {};
      
      if (!data.first_name) {
        errors.first_name = 'Vorname ist erforderlich';
      }
      if (!data.last_name) {
        errors.last_name = 'Nachname ist erforderlich';
      }
      if (!data.specialization) {
        errors.specialization = 'Fachgebiet ist erforderlich';
      }
      if (!data.id && !data.password) {
        errors.password = 'Passwort ist erforderlich für neue pädagogische Fachkräfte';
      }
      
      return Object.keys(errors).length > 0 ? errors : null;
    },
    
    transformBeforeSubmit: prepareTeacherForBackend,
  },
  
  detail: {
    header: {
      title: (teacher: Teacher) => teacher.name ?? `${teacher.first_name} ${teacher.last_name}`,
      subtitle: (teacher: Teacher) => {
        const parts: string[] = [];
        if (teacher.specialization) parts.push(teacher.specialization);
        if (teacher.role) parts.push(teacher.role);
        return parts.join(' • ') || 'Pädagogische Fachkraft';
      },
      avatar: {
        text: (teacher: Teacher) => {
          const initials = `${teacher.first_name?.[0] ?? ''}${teacher.last_name?.[0] ?? ''}`.toUpperCase();
          return initials || 'L';
        },
        size: 'lg',
      },
    },
    
    sections: [
      {
        title: 'Persönliche Daten',
        titleColor: 'text-blue-800',
        items: [
          {
            label: 'Name',
            value: (teacher: Teacher) => `${teacher.first_name} ${teacher.last_name}`,
          },
          {
            label: 'E-Mail',
            value: (teacher: Teacher) => teacher.email ?? 'Nicht angegeben',
          },
          {
            label: 'RFID-Karte',
            value: (teacher: Teacher) => teacher.tag_id ? `RFID: ${teacher.tag_id}` : 'Keine Karte zugewiesen',
          },
        ],
      },
      {
        title: 'Berufliche Informationen',
        titleColor: 'text-indigo-800',
        items: [
          {
            label: 'Fachgebiet',
            value: (teacher: Teacher) => teacher.specialization ?? 'Nicht angegeben',
          },
          {
            label: 'Rolle',
            value: (teacher: Teacher) => teacher.role ?? 'Nicht angegeben',
          },
          {
            label: 'Qualifikationen',
            value: (teacher: Teacher) => teacher.qualifications ?? 'Keine angegeben',
            colSpan: 2,
          },
          {
            label: 'Interne Notizen',
            value: (teacher: Teacher) => teacher.staff_notes ?? 'Keine Notizen',
            colSpan: 2,
          },
        ],
      },
      {
        title: 'Konto-Status',
        titleColor: 'text-purple-800',
        items: [
          {
            label: 'Konto-Informationen',
            value: (teacher: Teacher) => {
              if (!teacher.account_id) {
                return (
                  <div className="text-sm text-gray-500">
                    Kein Konto verknüpft - Erstellen Sie ein Konto, um Zugriffsrechte zu verwalten
                  </div>
                );
              }
              
              return (
                <div className="space-y-2">
                  <div className="text-sm">
                    <span className="font-medium">Konto-ID:</span> {teacher.account_id}
                  </div>
                  {teacher.email && (
                    <div className="text-sm">
                      <span className="font-medium">E-Mail:</span> {teacher.email}
                    </div>
                  )}
                  <div className="text-xs text-gray-500 mt-2">
                    Verwenden Sie die Aktionsbuttons oben, um Rollen und Berechtigungen zu verwalten
                  </div>
                </div>
              );
            },
            colSpan: 2,
          },
        ],
      },
    ],
  },
  
  list: {
    title: 'Pädagogische Fachkraft auswählen',
    description: 'Verwalten Sie Profile der pädagogischen Fachkräfte und Zuordnungen',
    searchPlaceholder: 'Pädagogische Fachkraft suchen...',
    
    // Frontend search for small dataset
    searchStrategy: 'frontend',
    searchableFields: ['name', 'first_name', 'last_name', 'email', 'specialization', 'role'],
    minSearchLength: 0,
    
    filters: [
      {
        id: 'role',
        label: 'Rolle',
        type: 'select',
        options: 'dynamic', // Will extract from data
      },
    ],
    
    item: {
      title: (teacher: Teacher) => teacher.name ?? `${teacher.first_name} ${teacher.last_name}`,
      subtitle: (teacher: Teacher) => teacher.specialization ?? 'Pädagogische Fachkraft',
      description: (teacher: Teacher) => {
        const parts: string[] = [];
        if (teacher.role) parts.push(teacher.role);
        if (teacher.email) parts.push(teacher.email);
        return parts.join(' • ');
      },
      avatar: {
        text: (teacher: Teacher) => {
          const initials = `${teacher.first_name?.[0] ?? ''}${teacher.last_name?.[0] ?? ''}`.toUpperCase();
          return initials || 'L';
        },
        backgroundColor: databaseThemes.teachers.primary,
      },
      badges: [
        {
          label: (teacher: Teacher) => teacher.role ?? '',
          color: 'bg-purple-100 text-purple-800',
          showWhen: (teacher: Teacher) => !!teacher.role,
        },
      ],
    },
  },
  
  service: {
    mapResponse: mapTeacherResponse,
    mapRequest: prepareTeacherForBackend,
    
    // Custom create handler for teacher-specific flow
    create: async (data) => {
      // Teacher creation requires multiple API calls (account, person, staff)
      // Use the teacher service which handles this complex flow
      console.log('Creating teacher with data:', data);
      const teacherData = data as Partial<Teacher> & { password?: string };
      const result = await teacherService.createTeacher(teacherData as Omit<Teacher, "id" | "name" | "created_at" | "updated_at"> & { password?: string });
      return result;
    },
    
    // Custom update handler for teacher-specific flow
    update: async (id, data) => {
      // Teacher update requires updating both person and staff records
      const result = await teacherService.updateTeacher(id, data);
      return result;
    },
  },
  
  labels: {
    createButton: 'Neue pädagogische Fachkraft erstellen',
    createModalTitle: 'Neue pädagogische Fachkraft',
    editModalTitle: 'Pädagogische Fachkraft bearbeiten',
    detailModalTitle: 'Details der pädagogischen Fachkraft',
    deleteConfirmation: 'Sind Sie sicher, dass Sie diese pädagogische Fachkraft löschen möchten?',
    emptyState: 'Keine pädagogischen Fachkräfte gefunden',
  },
  
  // Custom credential display after creation
  onCreateSuccess: (result: TeacherWithCredentials) => {
    if (result.temporaryCredentials) {
      return {
        type: 'credentials' as const,
        title: 'Lehrer erfolgreich erstellt!',
        message: 'Bitte notieren Sie sich die folgenden temporären Zugangsdaten:',
        credentials: {
          email: result.temporaryCredentials.email,
          password: result.temporaryCredentials.password,
        },
        note: 'Der Lehrer sollte das Passwort bei der ersten Anmeldung ändern.',
      };
    }
    return undefined;
  },
});