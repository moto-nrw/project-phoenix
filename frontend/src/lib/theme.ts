/**
 * Project Phoenix Theme Configuration
 * 
 * This file centralizes all design tokens used throughout the application
 * to ensure consistent styling across components.
 */

// Define the theme as a const with explicit types
export const theme = {
  colors: {
    primary: {
      500: '#14b8a6' as const, // teal-500
      600: '#0d9488' as const, // teal-600
      800: '#115e59' as const, // teal-800
    },
    secondary: {
      500: '#3b82f6' as const, // blue-500
      600: '#2563eb' as const, // blue-600
      700: '#1d4ed8' as const, // blue-700
    },
    gray: {
      50: '#f9fafb' as const,
      100: '#f3f4f6' as const,
      200: '#e5e7eb' as const,
      300: '#d1d5db' as const,
      500: '#6b7280' as const,
      600: '#4b5563' as const,
      700: '#374151' as const,
      800: '#1f2937' as const,
      900: '#111827' as const,
    },
    success: {
      50: '#ecfdf5' as const,  // green-50
      100: '#d1fae5' as const, // green-100
      700: '#047857' as const, // green-700
    },
    warning: {
      50: '#fffbeb' as const,  // yellow-50
      100: '#fef3c7' as const, // yellow-100
      700: '#b45309' as const, // yellow-700
    },
    error: {
      50: '#fef2f2' as const,  // red-50
      100: '#fee2e2' as const, // red-100
      600: '#dc2626' as const, // red-600
      700: '#b91c1c' as const, // red-700
    },
    info: {
      50: '#eff6ff' as const,  // blue-50
      100: '#dbeafe' as const, // blue-100
      700: '#1d4ed8' as const, // blue-700
    },
    background: {
      canvas: '#ffffff' as const,
      card: 'rgba(255, 255, 255, 0.95)' as const,
      wrapper: 'rgba(255, 255, 255, 0.2)' as const,
      gradientColors: ['#FF8080', '#80D8FF', '#A5D6A7', '#FFA726', '#9575CD'] as readonly string[],
    },
    text: {
      primary: '#1f2937' as const, // gray-800
      secondary: '#4b5563' as const, // gray-600
      disabled: '#9ca3af' as const, // gray-400
      onPrimary: '#ffffff' as const,
    },
  },
  
  typography: {
    fontFamily: {
      primary: '"Geist Sans", ui-sans-serif, system-ui, sans-serif, "Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol", "Noto Color Emoji"' as const,
    },
    fontSize: {
      xs: '0.75rem' as const,    // 12px
      sm: '0.875rem' as const,   // 14px
      base: '1rem' as const,     // 16px
      lg: '1.125rem' as const,   // 18px
      xl: '1.25rem' as const,    // 20px
      '2xl': '1.5rem' as const,  // 24px
      '3xl': '1.875rem' as const, // 30px
    },
    fontWeight: {
      normal: '400' as const,
      medium: '500' as const,
      bold: '700' as const,
    },
    lineHeight: {
      tight: '1.25' as const,
      normal: '1.5' as const,
      relaxed: '1.75' as const,
    },
  },
  
  spacing: {
    0: '0px' as const,
    1: '0.25rem' as const,  // 4px
    2: '0.5rem' as const,   // 8px
    3: '0.75rem' as const,  // 12px
    4: '1rem' as const,     // 16px
    6: '1.5rem' as const,   // 24px
    8: '2rem' as const,     // 32px
    10: '2.5rem' as const,  // 40px
    12: '3rem' as const,    // 48px
    16: '4rem' as const,    // 64px
  },
  
  borderRadius: {
    none: '0' as const,
    sm: '0.125rem' as const,  // 2px
    md: '0.375rem' as const,  // 6px
    lg: '0.5rem' as const,    // 8px
    xl: '0.75rem' as const,   // 12px
    '2xl': '1rem' as const,   // 16px
    full: '9999px' as const,
  },
  
  boxShadow: {
    sm: '0 1px 2px 0 rgba(0, 0, 0, 0.05)' as const,
    md: '0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)' as const,
    lg: '0 10px 15px -3px rgba(0, 0, 0, 0.1), 0 4px 6px -2px rgba(0, 0, 0, 0.05)' as const,
    xl: '0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04)' as const,
  },
  
  transition: {
    default: 'all 0.2s ease' as const,
    fast: 'all 0.1s ease' as const,
    slow: 'all 0.3s ease' as const,
  },
  
  breakpoints: {
    sm: '640px' as const,
    md: '768px' as const,
    lg: '1024px' as const,
    xl: '1280px' as const,
    '2xl': '1536px' as const,
  },
  
  zIndex: {
    background: -10 as const,
    backgroundWrapper: -5 as const,
    base: 0 as const,
    dropdown: 10 as const,
    modal: 50 as const,
    toast: 100 as const,
  },
} as const;

export type Theme = typeof theme;

/**
 * Helper type to access nested theme properties
 * Usage: ThemeValue<'colors.primary.500'> will return the type of that value
 */
export type ThemeValue<T extends string> = T extends `${infer K}.${infer Rest}`
  ? K extends keyof Theme
    ? Rest extends keyof Theme[K]
      ? Theme[K][Rest]
      : ThemeValue<Rest>
    : never
  : never;

export default theme;