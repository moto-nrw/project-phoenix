# Database Component Architecture

This directory contains shared configuration, themes, and service helpers for database management pages.

## Overview

Entity pages use a shared configuration object and service factory, then compose the page-specific list, detail, modal, and layout components they need.

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

### 2. Shared Components

- **DatabaseForm**: Renders forms based on configuration
- **DatabaseFormModal**: Wraps configured forms in create/edit modals
- **DatabasePageLayout**: Provides loading and page shell behavior
- **DatabaseEmptyState**, **DatabaseCreateAction**, and related helpers: Shared page controls

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
  
  concept: 'rooms',
  
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

### 2. Use the Configuration in a Page

```typescript
// app/database/rooms/page.tsx
"use client";

import { useMemo, useState } from "react";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";
import { RoomsMasterDetail } from "@/components/rooms/rooms-master-detail";
import { roomsConfig } from "@/components/database/configs/rooms.config";
import { createCrudService } from "@/lib/database/service-factory";

export default function RoomsPage() {
  const service = useMemo(() => createCrudService(roomsConfig), []);
  const [showCreateModal, setShowCreateModal] = useState(false);

  return (
    <DatabasePageLayout loading={loading} sessionLoading={sessionLoading}>
      {/* Fetch with service.getList(), then render the page-specific controls. */}
      <RoomsMasterDetail {...masterDetailProps} />
      <DatabaseFormModal<Room>
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        mode="create"
        config={roomsConfig}
        onSubmit={handleCreateRoom}
      />
    </DatabasePageLayout>
  );
}
```

`DatabaseFormModal` renders the config's `form.sections` inside the shared
modal; `mode` picks the `createModalTitle` / `editModalTitle` label (the label
for the used mode is required — the modal fails loudly when it is missing).

The configuration and service factory provide the shared CRUD behavior. Each page still owns its own data loading, selection state, filters, and entity-specific UI:
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

### Concepts

Each entity config carries a `concept` key instead of a hand-rolled theme
object. The key indexes `MOTO_CONCEPTS` in `lib/moto-concepts.ts`, which is the
single source for the icon and color tone a page header, section header and
card header render — see the `concept` field on `EntityConfig` in `types.ts`.

The former `lib/database/themes.tsx` and its `databaseThemes` export are gone;
adding a new entity means picking an existing `MotoConceptKey` or adding one to
`MOTO_CONCEPTS`, not defining a new color pair.

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
3. Create a CRUD service with `createCrudService(config)`
4. Render the page with `DatabasePageLayout` and the entity-specific list, detail, and modal components
5. Test thoroughly
6. Remove old components once verified

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
