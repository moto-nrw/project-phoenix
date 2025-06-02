/**
 * Examples of using DatabaseSelect in forms
 * This file is for documentation - not imported anywhere
 */

import { DatabaseSelect, GroupSelect, RoomSelect } from './database-select';
import type { FormField, FormSection } from './database-form';

// Example 1: Using GroupSelect in a form field configuration
// Note: These examples assume formData and handleFieldChange are defined in your component
// eslint-disable-next-line @typescript-eslint/no-unused-vars
const _groupFieldExample = (_formData: { group_id: string }, _handleFieldChange: (name: string, value: string) => void): FormField => ({
  name: 'group_id',
  label: 'Gruppe',
  type: 'custom',
  component: ({ value, onChange, required }) => (
    <GroupSelect
      name="group_id"
      value={value as string}
      onChange={onChange as (value: string) => void}
      required={required}
      placeholder="Bitte wählen Sie eine Gruppe"
    />
  )
});

// Example 2: Static options dropdown
// eslint-disable-next-line @typescript-eslint/no-unused-vars
const _roleFieldExample = (_formData: { role: string }, _handleFieldChange: (name: string, value: string) => void): FormField => ({
  name: 'role',
  label: 'Rolle',
  type: 'custom',
  component: ({ value, onChange }) => (
    <DatabaseSelect
      name="role"
      label="Rolle"
      value={value as string}
      onChange={onChange as (value: string) => void}
      options={[
        { value: 'teacher', label: 'Lehrer' },
        { value: 'coordinator', label: 'Koordinator' },
        { value: 'assistant', label: 'Assistent' },
      ]}
    />
  )
});

// Example 3: Async loading with custom endpoint
// eslint-disable-next-line @typescript-eslint/no-unused-vars
const _customDropdownFieldExample = (_formData: { category_id: string }, _handleFieldChange: (name: string, value: string) => void): FormField => ({
  name: 'category_id',
  label: 'Kategorie',
  type: 'custom',
  component: ({ value, onChange }) => (
    <DatabaseSelect
      name="category_id"
      label="Kategorie"
      value={value as string}
      onChange={onChange as (value: string) => void}
      loadOptions={async () => {
        const response = await fetch('/api/categories');
        const data = await response.json() as Array<{ id: string; name: string }>;
        return data.map(cat => ({
          value: cat.id,
          label: cat.name,
        }));
      }}
      helperText="Wählen Sie eine Kategorie aus"
    />
  )
});

// Example 4: Complete form sections using our dropdowns
// This would be used inside a component where formData and setFormData are defined
type StudentFormData = {
  first_name: string;
  second_name: string;
  school_class: string;
  group_id: string;
};

export const createStudentFormSections = (
  _formData: StudentFormData,
  _setFormData: React.Dispatch<React.SetStateAction<StudentFormData>>
): FormSection[] => [
  {
    title: "Persönliche Daten",
    fields: [
      { name: "first_name", label: "Vorname", type: "text", required: true },
      { name: "second_name", label: "Nachname", type: "text", required: true },
      { name: "school_class", label: "Klasse", type: "text", required: true },
      {
        name: "group_id",
        label: "OGS Gruppe",
        type: "custom",
        component: ({ value, onChange, required }) => (
          <GroupSelect
            name="group_id"
            value={value as string}
            onChange={onChange as (value: string) => void}
            label="OGS Gruppe"
            required={required}
            includeEmpty={true}
            emptyOptionLabel="Bitte wählen Sie eine Gruppe"
          />
        )
      }
    ],
    columns: 2
  }
];

// Example 5: Room selection with filtering
// eslint-disable-next-line @typescript-eslint/no-unused-vars
const _roomWithFilterFieldExample = (_formData: { room_id: string }, _handleFieldChange: (name: string, value: string) => void): FormField => ({
  name: 'room_id',
  label: 'Raum',
  type: 'custom',
  component: ({ value, onChange }) => (
    <RoomSelect
      name="room_id"
      value={value as string}
      onChange={onChange as (value: string) => void}
      filters={{ building: 'A', available: true }}
      placeholder="Wählen Sie einen Raum"
    />
  )
});