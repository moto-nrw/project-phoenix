/**
 * Examples of using DatabaseSelect in forms
 * This file is for documentation - not imported anywhere
 */

import { DatabaseSelect, GroupSelect, RoomSelect } from './database-select';
import type { FormField } from './database-form';

// Example 1: Using GroupSelect in a form field configuration
const groupField: FormField = {
  name: 'group_id',
  type: 'custom',
  customComponent: (
    <GroupSelect
      name="group_id"
      value={formData.group_id}
      onChange={(value) => handleFieldChange('group_id', value)}
      required={true}
      placeholder="Bitte wählen Sie eine Gruppe"
    />
  )
};

// Example 2: Static options dropdown
const roleField: FormField = {
  name: 'role',
  label: 'Rolle',
  type: 'custom',
  customComponent: (
    <DatabaseSelect
      name="role"
      label="Rolle"
      value={formData.role}
      onChange={(value) => handleFieldChange('role', value)}
      options={[
        { value: 'teacher', label: 'Lehrer' },
        { value: 'coordinator', label: 'Koordinator' },
        { value: 'assistant', label: 'Assistent' },
      ]}
    />
  )
};

// Example 3: Async loading with custom endpoint
const customDropdownField: FormField = {
  name: 'category_id',
  type: 'custom',
  customComponent: (
    <DatabaseSelect
      name="category_id"
      label="Kategorie"
      value={formData.category_id}
      onChange={(value) => handleFieldChange('category_id', value)}
      loadOptions={async () => {
        const response = await fetch('/api/categories');
        const data = await response.json();
        return data.map(cat => ({
          value: cat.id,
          label: cat.name,
        }));
      }}
      helperText="Wählen Sie eine Kategorie aus"
    />
  )
};

// Example 4: Complete form sections using our dropdowns
export const STUDENT_FORM_SECTIONS = [
  {
    title: "Persönliche Daten",
    fields: [
      { name: "first_name", label: "Vorname", type: "text", required: true },
      { name: "second_name", label: "Nachname", type: "text", required: true },
      { name: "school_class", label: "Klasse", type: "text", required: true },
      {
        name: "group_id",
        type: "custom",
        customComponent: (
          <GroupSelect
            name="group_id"
            value={formData.group_id}
            onChange={(groupId) => setFormData(prev => ({ ...prev, group_id: groupId }))}
            label="OGS Gruppe"
            required={true}
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
const roomWithFilterField: FormField = {
  name: 'room_id',
  type: 'custom',
  customComponent: (
    <RoomSelect
      name="room_id"
      value={formData.room_id}
      onChange={(value) => handleFieldChange('room_id', value)}
      filters={{ building: 'A', available: true }}
      placeholder="Wählen Sie einen Raum"
    />
  )
};