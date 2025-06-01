# Database Component Architecture

This directory contains a reusable system for creating database management pages with minimal code duplication.

## Overview

Instead of writing 300+ lines of boilerplate for each entity page, you can now create a complete CRUD interface with just a configuration object and a single line page component.

## Architecture

### 1. Entity Configuration (`configs/`)

Each entity has a configuration file that defines:
- Form fields and sections
- Detail view layout
- List view appearance
- API endpoints
- Data transformation
- Business logic hooks

Example: `configs/students.config.tsx`

### 2. Generic Components

- **DatabasePage**: Handles all CRUD operations, state management, and modals
- **DatabaseForm**: Renders forms based on configuration
- **DatabaseDetailView**: Displays entity details with configurable sections
- **DatabaseListPage/Item**: List view components (existing)

### 3. Service Factory

The `service-factory.ts` automatically generates CRUD services from configuration:
- getList (with pagination)
- getOne
- create
- update
- delete

## Usage Example

### 1. Create Entity Configuration

```typescript
// configs/rooms.config.tsx
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
        fields: [
          {
            name: 'name',
            label: 'Raumname',
            type: 'text',
            required: true,
          },
          // ... more fields
        ],
      },
    ],
  },
  
  detail: {
    sections: [
      {
        title: 'Raumdetails',
        items: [
          {
            label: 'Raumname',
            value: (room) => room.name,
          },
          // ... more items
        ],
      },
    ],
  },
  
  list: {
    title: 'Raum auswählen',
    searchPlaceholder: 'Raum suchen...',
    item: {
      title: (room) => room.name,
      subtitle: (room) => `Etage ${room.floor}`,
    },
  },
});
```

### 2. Create Page Component

```typescript
// app/database/rooms/page.tsx
"use client";

import { DatabasePage } from "@/components/ui/database";
import { roomsConfig } from "@/lib/database";

export default function RoomsPage() {
  return <DatabasePage config={roomsConfig} />;
}
```

That's it! You now have a complete CRUD interface with:
- List view with search and filters
- Create modal with form validation
- Detail view with edit/delete actions
- Responsive design
- Error handling
- Loading states

## Configuration Reference

### Field Types

- `text`: Standard text input
- `email`: Email input with validation
- `password`: Password input
- `textarea`: Multi-line text
- `select`: Dropdown with options
- `checkbox`: Boolean checkbox
- `custom`: Custom component (e.g., GroupSelect)

### Themes

Pre-defined themes in `components/ui/database/themes.tsx`:
- `students`: Teal/Blue
- `teachers`: Purple/Indigo
- `rooms`: Green/Emerald
- `activities`: Orange/Red
- `groups`: Indigo/Purple

### Hooks

Optional lifecycle hooks for business logic:
- `beforeCreate`: Transform data before creation
- `afterCreate`: Side effects after creation
- `beforeUpdate`: Validate/transform updates
- `afterUpdate`: Side effects after update
- `beforeDelete`: Confirm deletion
- `afterDelete`: Cleanup after deletion

## Migration Guide

To migrate an existing page:

1. Analyze the existing page structure
2. Create entity configuration matching the fields and layout
3. Replace the page component with `<DatabasePage config={config} />`
4. Test thoroughly
5. Remove old components once verified

## Benefits

- **Consistency**: All pages follow the same UX patterns
- **Maintainability**: Changes to common behavior update everywhere
- **Speed**: New entities can be added in minutes
- **Type Safety**: Full TypeScript support
- **Customization**: Override any part when needed

## Future Enhancements

- Field dependencies (show/hide based on other fields)
- Bulk operations
- Import/Export functionality
- Advanced filtering UI
- Inline editing in list view