# Theme System

This directory contains the theme configuration for Project Phoenix.

## Usage

Import the theme configuration in your components:

```tsx
import { theme } from '@/lib';

// Use theme values directly
const primaryColor = theme.colors.primary[600];
const spacing = theme.spacing[4];
```

### Example with Tailwind classes

```tsx
// Continue using Tailwind classes that correspond to theme values
<div className="text-primary-600 bg-background-card p-4 rounded-lg">
  Themed content
</div>
```

### Example with inline styles

```tsx
// Use theme values directly with inline styles when needed
<div style={{ 
  color: theme.colors.primary[600],
  backgroundColor: theme.colors.background.card,
  padding: theme.spacing[4],
  borderRadius: theme.borderRadius.lg 
}}>
  Themed content
</div>
```

## Theme Properties

The theme configuration includes:

- **Colors**: Brand colors, text colors, background colors, status colors
- **Typography**: Font families, sizes, weights, and line heights
- **Spacing**: Margin, padding, and gap values
- **Border radius**: Different border radius options
- **Shadows**: Different shadow options
- **Transitions**: Default animation settings
- **Breakpoints**: Screen size breakpoints
- **Z-index**: Layer management

## Type Safety

The theme configuration is fully typed:

```tsx
// Get type-safe access to theme properties
import type { ThemeValue } from '@/lib';

type PrimaryColor = ThemeValue<'colors.primary.600'>;
```