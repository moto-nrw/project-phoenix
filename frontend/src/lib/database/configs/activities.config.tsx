// Activity Entity Configuration

import { defineEntityConfig } from '../types';
import { databaseThemes } from '@/components/ui/database/themes';
import type { Activity } from '@/lib/api';
import { getSession } from 'next-auth/react';

export const activitiesConfig = defineEntityConfig<Activity>({
  name: {
    singular: 'Aktivität',
    plural: 'Aktivitäten'
  },
  
  theme: databaseThemes.activities,
  
  api: {
    basePath: '/api/activities',
  },
  
  form: {
    sections: [
      {
        title: 'Grundinformationen',
        backgroundColor: 'bg-red-50',
        columns: 2,
        fields: [
          {
            name: 'name',
            label: 'Name',
            type: 'text',
            required: true,
            placeholder: 'z.B. Fußball AG',
          },
          {
            name: 'ag_category_id',
            label: 'Kategorie',
            type: 'select',
            required: true,
            options: async () => {
              // Fetch categories from API
              const response = await fetch('/api/activities/categories', {
                headers: {
                  'Authorization': `Bearer ${(await getSession())?.user?.token}`,
                },
              });
              const result = await response.json() as { data: Array<{ id: number; name: string }> };
              const categories = result.data || [];
              return categories.map((cat) => ({
                value: cat.id.toString(),
                label: cat.name,
              }));
            },
          },
          {
            name: 'max_participant',
            label: 'Maximale Teilnehmer',
            type: 'number',
            required: true,
            min: 1,
            placeholder: '20',
          },
          {
            name: 'description',
            label: 'Beschreibung',
            type: 'textarea',
            colSpan: 2,
            placeholder: 'Beschreiben Sie die Aktivität...',
          },
        ],
      },
    ],
    
    defaultValues: {
      max_participant: 20,
      is_open_ags: true,
    },
    
    transformBeforeSubmit: (data) => {
      return {
        ...data,
        ag_category_id: data.ag_category_id ? parseInt(data.ag_category_id) : undefined,
      };
    },
  },
  
  detail: {
    header: {
      title: (activity) => activity.name,
      subtitle: (activity) => activity.category_name ?? 'Keine Kategorie',
      avatar: {
        text: (activity) => {
          // Use emoji based on category or activity name
          const name = activity.name?.toLowerCase();
          const category = activity.category_name?.toLowerCase();
          
          // Sports activities
          if (name?.includes('fußball') || name?.includes('fussball')) return '⚽';
          if (name?.includes('basketball')) return '🏀';
          if (name?.includes('volleyball')) return '🏐';
          if (name?.includes('tennis')) return '🎾';
          if (name?.includes('schwimm')) return '🏊';
          if (name?.includes('lauf') || name?.includes('athletik')) return '🏃';
          if (name?.includes('turnen') || name?.includes('gym')) return '🤸';
          if (name?.includes('sport') || category?.includes('sport')) return '🏃';
          
          // Creative activities
          if (name?.includes('kunst') || name?.includes('mal') || name?.includes('zeich')) return '🎨';
          if (name?.includes('musik') || name?.includes('chor') || name?.includes('band')) return '🎵';
          if (name?.includes('theater') || name?.includes('drama')) return '🎭';
          if (name?.includes('tanz') || name?.includes('dance')) return '💃';
          if (name?.includes('foto') || name?.includes('photo')) return '📸';
          if (name?.includes('film') || name?.includes('video')) return '🎬';
          
          // Academic activities
          if (name?.includes('mathematik') || name?.includes('mathe')) return '🔢';
          if (name?.includes('physik') || name?.includes('chemie') || name?.includes('labor')) return '🔬';
          if (name?.includes('biologie') || name?.includes('natur')) return '🌿';
          if (name?.includes('computer') || name?.includes('informatik') || name?.includes('coding')) return '💻';
          if (name?.includes('robotik') || name?.includes('technik')) return '🤖';
          if (name?.includes('sprach') || name?.includes('english') || name?.includes('französisch')) return '🗣️';
          if (name?.includes('lesen') || name?.includes('buch') || name?.includes('literatur')) return '📚';
          if (name?.includes('schreib') || name?.includes('journal')) return '✍️';
          
          // Practical activities
          if (name?.includes('koch') || name?.includes('küche') || name?.includes('back')) return '🍳';
          if (name?.includes('garten') || name?.includes('pflanzen')) return '🌱';
          if (name?.includes('werk') || name?.includes('holz') || name?.includes('handwerk')) return '🔨';
          if (name?.includes('näh') || name?.includes('textil') || name?.includes('schneid')) return '🧵';
          
          // Games and fun
          if (name?.includes('schach')) return '♟️';
          if (name?.includes('spiel') || name?.includes('game')) return '🎲';
          if (name?.includes('puzzle') || name?.includes('rätsel')) return '🧩';
          
          // Other activities
          if (name?.includes('meditation') || name?.includes('yoga') || name?.includes('entspann')) return '🧘';
          if (name?.includes('erste hilfe') || name?.includes('sanitäter')) return '🚑';
          if (name?.includes('umwelt') || name?.includes('recycl') || name?.includes('nachhaltig')) return '♻️';
          if (name?.includes('feuer') || name?.includes('pfadfinder')) return '🔥';
          
          // Default fallback to first two letters
          return activity.name ? activity.name.substring(0, 2).toUpperCase() : 'AG';
        },
        size: 'lg',
      },
      badges: [
        {
          label: (activity) => `${activity.participant_count || 0}/${activity.max_participant}`,
          color: 'bg-blue-100 text-blue-700',
          showWhen: () => true,
        },
        {
          label: 'Voll',
          color: 'bg-red-100 text-red-700',
          showWhen: (activity) => (activity.participant_count || 0) >= activity.max_participant,
        },
      ],
    },
    
    sections: [
      {
        title: 'Grundinformationen',
        titleColor: 'text-red-800',
        items: [
          {
            label: 'Name',
            value: (activity) => activity.name,
          },
          {
            label: 'Kategorie',
            value: (activity) => activity.category_name ?? 'Keine Kategorie',
          },
          {
            label: 'Beschreibung',
            value: (activity) => activity.description || 'Keine Beschreibung',
          },
          {
            label: 'Maximale Teilnehmer',
            value: (activity) => activity.max_participant.toString(),
          },
          {
            label: 'Aktuelle Teilnehmer',
            value: (activity) => (activity.participant_count || 0).toString(),
          },
        ],
      },
      {
        title: 'Betreuer',
        titleColor: 'text-purple-800',
        items: [
          {
            label: 'Hauptbetreuer',
            value: (activity) => {
              const primary = activity.supervisors?.find(s => s.is_primary);
              return primary 
                ? `${primary.first_name} ${primary.last_name}`
                : 'Kein Hauptbetreuer zugewiesen';
            },
          },
          {
            label: 'Weitere Betreuer',
            value: (activity) => {
              const secondary = activity.supervisors?.filter(s => !s.is_primary) || [];
              return secondary.length > 0
                ? secondary.map(s => `${s.first_name} ${s.last_name}`).join(', ')
                : 'Keine weiteren Betreuer';
            },
          },
        ],
      },
    ],
    
    actions: {
      edit: true,
      delete: true,
      custom: [
        {
          label: 'Schüler verwalten',
          onClick: (activity) => {
            window.location.href = `/database/activities/${activity.id}/students`;
          },
          color: 'blue',
        },
        {
          label: 'Zeiten verwalten',
          onClick: (activity) => {
            window.location.href = `/database/activities/${activity.id}/times`;
          },
          color: 'green',
        },
      ],
    },
  },
  
  list: {
    title: 'Aktivität auswählen',
    description: 'Verwalte Aktivitäten und deren Teilnehmer',
    searchPlaceholder: 'Aktivität suchen...',
    
    // Frontend search for better UX
    searchStrategy: 'frontend',
    searchableFields: ['name', 'category_name', 'description'],
    minSearchLength: 0,
    
    filters: [
      {
        id: 'ag_category_id',
        label: 'Kategorie',
        type: 'select',
        options: 'dynamic', // Will be extracted from the loaded data
      },
      {
        id: 'supervisor_id',
        label: 'Betreuer',
        type: 'select',
        options: async () => {
          // Fetch supervisors from API
          const response = await fetch('/api/activities/supervisors', {
            headers: {
              'Authorization': `Bearer ${(await getSession())?.user?.token}`,
            },
          });
          const data = await response.json() as Array<{ id: string; name: string }>;
          return data.map((sup) => ({
            value: sup.id,
            label: sup.name,
          }));
        },
      },
    ],
    
    item: {
      title: (activity) => activity.name,
      subtitle: (activity) => {
        const primary = activity.supervisors?.find(s => s.is_primary);
        return primary 
          ? `${primary.first_name} ${primary.last_name}`
          : 'Kein Hauptbetreuer';
      },
      description: (activity) => {
        if (activity.is_open_ags) {
          return 'Anmeldung offen';
        }
        return 'Anmeldung geschlossen';
      },
      avatar: {
        text: (activity) => {
          // Use emoji based on category or activity name
          const name = activity.name?.toLowerCase();
          const category = activity.category_name?.toLowerCase();
          
          // Sports activities
          if (name?.includes('fußball') || name?.includes('fussball')) return '⚽';
          if (name?.includes('basketball')) return '🏀';
          if (name?.includes('volleyball')) return '🏐';
          if (name?.includes('tennis')) return '🎾';
          if (name?.includes('schwimm')) return '🏊';
          if (name?.includes('lauf') || name?.includes('athletik')) return '🏃';
          if (name?.includes('turnen') || name?.includes('gym')) return '🤸';
          if (name?.includes('sport') || category?.includes('sport')) return '🏃';
          
          // Creative activities
          if (name?.includes('kunst') || name?.includes('mal') || name?.includes('zeich')) return '🎨';
          if (name?.includes('musik') || name?.includes('chor') || name?.includes('band')) return '🎵';
          if (name?.includes('theater') || name?.includes('drama')) return '🎭';
          if (name?.includes('tanz') || name?.includes('dance')) return '💃';
          if (name?.includes('foto') || name?.includes('photo')) return '📸';
          if (name?.includes('film') || name?.includes('video')) return '🎬';
          
          // Academic activities
          if (name?.includes('mathematik') || name?.includes('mathe')) return '🔢';
          if (name?.includes('physik') || name?.includes('chemie') || name?.includes('labor')) return '🔬';
          if (name?.includes('biologie') || name?.includes('natur')) return '🌿';
          if (name?.includes('computer') || name?.includes('informatik') || name?.includes('coding')) return '💻';
          if (name?.includes('robotik') || name?.includes('technik')) return '🤖';
          if (name?.includes('sprach') || name?.includes('english') || name?.includes('französisch')) return '🗣️';
          if (name?.includes('lesen') || name?.includes('buch') || name?.includes('literatur')) return '📚';
          if (name?.includes('schreib') || name?.includes('journal')) return '✍️';
          
          // Practical activities
          if (name?.includes('koch') || name?.includes('küche') || name?.includes('back')) return '🍳';
          if (name?.includes('garten') || name?.includes('pflanzen')) return '🌱';
          if (name?.includes('werk') || name?.includes('holz') || name?.includes('handwerk')) return '🔨';
          if (name?.includes('näh') || name?.includes('textil') || name?.includes('schneid')) return '🧵';
          
          // Games and fun
          if (name?.includes('schach')) return '♟️';
          if (name?.includes('spiel') || name?.includes('game')) return '🎲';
          if (name?.includes('puzzle') || name?.includes('rätsel')) return '🧩';
          
          // Other activities
          if (name?.includes('meditation') || name?.includes('yoga') || name?.includes('entspann')) return '🧘';
          if (name?.includes('erste hilfe') || name?.includes('sanitäter')) return '🚑';
          if (name?.includes('umwelt') || name?.includes('recycl') || name?.includes('nachhaltig')) return '♻️';
          if (name?.includes('feuer') || name?.includes('pfadfinder')) return '🔥';
          
          // Default fallback to first two letters
          return activity.name ? activity.name.substring(0, 2).toUpperCase() : 'AG';
        },
      },
      badges: [
        {
          label: (activity) => activity.category_name ?? 'Keine Kategorie',
          color: 'bg-purple-100 text-purple-700',
          showWhen: () => true,
        },
        {
          label: (activity) => `${activity.participant_count || 0}/${activity.max_participant}`,
          color: 'bg-blue-100 text-blue-700',
          showWhen: () => true,
        },
        {
          label: 'Voll',
          color: 'bg-red-100 text-red-700',
          showWhen: (activity) => (activity.participant_count || 0) >= activity.max_participant,
        },
      ],
    },
  },
  
  service: {
    // No custom mapping needed - the API routes already handle the mapping
    // using mapActivityResponse and prepareActivityForBackend
  },
  
  labels: {
    createButton: 'Neue Aktivität erstellen',
    createModalTitle: 'Neue Aktivität',
    editModalTitle: 'Aktivität bearbeiten',
    detailModalTitle: 'Aktivitätsdetails',
    deleteConfirmation: 'Sind Sie sicher, dass Sie diese Aktivität löschen möchten?',
  },
});