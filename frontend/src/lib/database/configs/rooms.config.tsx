// Room Entity Configuration

import { defineEntityConfig } from '../types';
import { databaseThemes } from '@/components/ui/database/themes';
import { mapRoomResponse, prepareRoomForBackend } from '@/lib/room-helpers';
import type { Room } from '@/lib/room-helpers';

export const roomsConfig = defineEntityConfig<Room>({
  name: {
    singular: 'Raum',
    plural: 'Räume'
  },
  
  theme: databaseThemes.rooms,
  
  api: {
    basePath: '/api/rooms',
  },
  
  form: {
    sections: [
      {
        title: 'Raumdetails',
        backgroundColor: 'bg-green-50',
        columns: 2,
        fields: [
          {
            name: 'name',
            label: 'Raumname',
            type: 'text',
            required: true,
            placeholder: 'z.B. Klassenraum 101',
          },
          {
            name: 'category',
            label: 'Kategorie',
            type: 'select',
            required: true,
            options: [
              // German values (preferred)
              { value: 'Klassenzimmer', label: 'Klassenzimmer' },
              { value: 'Labor', label: 'Labor' },
              { value: 'Sport', label: 'Sport' },
              { value: 'Kunst', label: 'Kunst' },
              { value: 'Musik', label: 'Musik' },
              { value: 'Computer', label: 'Computer' },
              { value: 'Bibliothek', label: 'Bibliothek' },
              { value: 'Lernraum', label: 'Lernraum' },
              { value: 'Speiseraum', label: 'Speiseraum' },
              { value: 'Versammlung', label: 'Versammlung' },
              { value: 'Medizin', label: 'Medizin' },
              { value: 'Büro', label: 'Büro' },
              { value: 'Besprechung', label: 'Besprechung' },
              // English values (for backward compatibility)
              { value: 'classroom', label: 'Klassenzimmer (alt)' },
              { value: 'grouproom', label: 'Gruppenraum (alt)' },
              { value: 'specialroom', label: 'Fachraum (alt)' },
              { value: 'other', label: 'Sonstiges (alt)' },
            ],
          },
          {
            name: 'capacity',
            label: 'Kapazität',
            type: 'text',
            placeholder: 'Anzahl der Plätze',
            validation: (value) => {
              if (value && isNaN(parseInt(value as string))) {
                return 'Bitte geben Sie eine gültige Zahl ein';
              }
              return null;
            },
          },
          {
            name: 'building',
            label: 'Gebäude',
            type: 'text',
            placeholder: 'z.B. Gebäude A',
          },
          {
            name: 'floor',
            label: 'Etage',
            type: 'text',
            required: true,
            placeholder: 'z.B. 0, 1, 2',
            validation: (value) => {
              if (!value || isNaN(parseInt(value as string))) {
                return 'Bitte geben Sie eine gültige Etage ein';
              }
              return null;
            },
          },
        ],
      },
    ],
    
    defaultValues: {
      category: 'Klassenzimmer',
      floor: 0,
      capacity: 30,
    },
    
    transformBeforeSubmit: (data) => {
      // Ensure floor and capacity are numbers
      return {
        ...data,
        floor: typeof data.floor === 'string' ? parseInt(data.floor, 10) : data.floor,
        capacity: typeof data.capacity === 'string' ? parseInt(data.capacity, 10) : data.capacity,
      };
    },
  },
  
  detail: {
    header: {
      title: (room) => room.name,
      subtitle: (room) => room.building ? `${room.building}, Etage ${room.floor}` : `Etage ${room.floor}`,
      avatar: {
        text: (room) => room.name?.[0] ?? 'R',
        size: 'md',
      },
    },
    
    sections: [
      {
        title: 'Raumdetails',
        titleColor: 'text-green-800',
        items: [
          {
            label: 'Raumname',
            value: (room) => room.name,
          },
          {
            label: 'Kategorie',
            value: (room) => {
              const categoryMap: Record<string, string> = {
                classroom: 'Klassenraum',
                grouproom: 'Gruppenraum',
                specialroom: 'Fachraum',
                other: 'Sonstiges',
              };
              return categoryMap[room.category] || room.category;
            },
          },
          {
            label: 'Kapazität',
            value: (room) => room.capacity ? `${room.capacity} Plätze` : 'Nicht angegeben',
          },
          {
            label: 'Gebäude',
            value: (room) => room.building || 'Nicht angegeben',
          },
          {
            label: 'Etage',
            value: (room) => `Etage ${room.floor}`,
          },
          {
            label: 'Status',
            value: (room) => room.isOccupied ? 'Belegt' : 'Frei',
            colSpan: 2,
          },
        ],
      },
    ],
  },
  
  list: {
    title: 'Raum auswählen',
    description: 'Verwalte Räume und deren Eigenschaften',
    searchPlaceholder: 'Raum suchen...',
    
    // No filters needed for ~20 rooms - search is sufficient
    
    // Frontend search configuration
    searchStrategy: 'frontend',
    searchableFields: ['name', 'category', 'building'],
    minSearchLength: 0,
    
    item: {
      title: (room) => room.name,
      subtitle: (room) => {
        // Show occupancy status as subtitle
        if (room.isOccupied) {
          const parts = ['Belegt'];
          if (room.groupName) parts.push(`Gruppe: ${room.groupName}`);
          if (room.activityName) parts.push(room.activityName);
          return parts.join(' • ');
        }
        return 'Frei';
      },
      description: (room) => {
        const parts = [];
        if (room.building) parts.push(`Gebäude ${room.building}`);
        parts.push(`Etage ${room.floor}`);
        if (room.capacity) parts.push(`${room.capacity} Plätze`);
        if (room.isOccupied && room.studentCount !== undefined) {
          parts.push(`${room.studentCount} Schüler`);
        }
        return parts.join(' • ');
      },
      avatar: {
        text: (room) => {
          // Use icon based on category or first letter
          const category = room.category?.toLowerCase();
          if (category?.includes('klassenraum') || category === 'klassenzimmer') return '📚';
          if (category?.includes('labor')) return '🔬';
          if (category?.includes('sport')) return '🏃';
          if (category?.includes('kunst')) return '🎨';
          if (category?.includes('musik')) return '🎵';
          if (category?.includes('computer')) return '💻';
          if (category?.includes('bibliothek')) return '📖';
          if (category?.includes('speise') || category?.includes('mensa')) return '🍽️';
          if (category?.includes('versammlung') || category?.includes('aula')) return '🎭';
          if (category?.includes('medizin') || category?.includes('kranken')) return '🏥';
          if (category?.includes('büro')) return '🏢';
          if (category?.includes('besprechung') || category?.includes('konferenz')) return '💬';
          return room.name ? room.name[0] : 'R';
        },
        backgroundColor: databaseThemes.rooms.primary,
      },
      badges: [
        // Category badge
        {
          label: (room) => {
            const categoryMap: Record<string, string> = {
              // German values (keep as is)
              'Klassenzimmer': 'Klassenzimmer',
              'Labor': 'Labor',
              'Sport': 'Sport',
              'Kunst': 'Kunst',
              'Musik': 'Musik',
              'Computer': 'Computer',
              'Bibliothek': 'Bibliothek',
              'Lernraum': 'Lernraum',
              'Speiseraum': 'Speiseraum',
              'Versammlung': 'Versammlung',
              'Medizin': 'Medizin',
              'Büro': 'Büro',
              'Besprechung': 'Besprechung',
              // English values (map to German)
              'classroom': 'Klassenzimmer',
              'grouproom': 'Gruppenraum',
              'specialroom': 'Fachraum',
              'other': 'Sonstiges',
              'standard': 'Standard',
            };
            return categoryMap[room.category] || room.category || 'Standard';
          },
          color: 'bg-indigo-100 text-indigo-800',
          showWhen: (room) => !!room.category,
        },
        // Building and floor badge  
        {
          label: (room) => room.building ? `${room.building} - Etage ${room.floor}` : `Etage ${room.floor}`,
          color: 'bg-gray-100 text-gray-800',
          showWhen: (room) => room.floor !== undefined,
        },
        // Capacity badge
        {
          label: (room) => `${room.capacity} Plätze`,
          color: 'bg-blue-100 text-blue-800',
          showWhen: (room) => !!room.capacity,
        },
        // Occupancy status badge
        {
          label: 'Belegt',
          color: 'bg-red-100 text-red-800',
          showWhen: (room) => room.isOccupied === true,
        },
        {
          label: 'Frei',
          color: 'bg-green-100 text-green-800',
          showWhen: (room) => room.isOccupied === false,
        },
      ],
    },
  },
  
  service: {
    mapResponse: mapRoomResponse,
    mapRequest: prepareRoomForBackend,
  },
  
  labels: {
    createButton: 'Neuen Raum erstellen',
    createModalTitle: 'Neuer Raum',
    editModalTitle: 'Raum bearbeiten',
    detailModalTitle: 'Raumdetails',
    deleteConfirmation: 'Sind Sie sicher, dass Sie diesen Raum löschen möchten?',
  },
});