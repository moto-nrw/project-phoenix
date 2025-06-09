import type { ReactNode } from "react";

/**
 * Theme configuration for database pages
 * Each entity type gets its own color scheme while maintaining consistent UX
 */
export interface DatabaseTheme {
  /** Main gradient start color (e.g., 'teal-500') */
  primary: string;
  /** Main gradient end color (e.g., 'blue-600') */
  secondary: string;
  /** Section header colors (e.g., 'blue') */
  accent: string;
  /** Section background colors (e.g., 'blue-50') */
  background: string;
  /** Border colors for sections (e.g., 'blue-200') */
  border: string;
  /** Text color for section headers (e.g., 'blue-800') */
  textAccent: string;
  /** Empty state icon */
  icon: ReactNode;
  /** List item avatar gradient */
  avatarGradient: string;
}

// Icons will be imported when used
const UsersIcon = () => (
  <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
  </svg>
);

const AcademicCapIcon = () => (
  <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 14l9-5-9-5-9 5 9 5z" />
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 14l6.16-3.422a12.083 12.083 0 01.665 6.479A11.952 11.952 0 0012 20.055a11.952 11.952 0 00-6.824-2.998 12.078 12.078 0 01.665-6.479L12 14z" />
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 14l9-5-9-5-9 5 9 5zm0 0l6.16-3.422a12.083 12.083 0 01.665 6.479A11.952 11.952 0 0012 20.055a11.952 11.952 0 00-6.824-2.998 12.078 12.078 0 01.665-6.479L12 14zm-4 6v-7.5l4-2.222" />
  </svg>
);

const BuildingIcon = () => (
  <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
  </svg>
);

const CalendarIcon = () => (
  <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
  </svg>
);

const GroupIcon = () => (
  <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
  </svg>
);

const ShieldIcon = () => (
  <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
  </svg>
);

const DeviceIcon = () => (
  <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
  </svg>
);

/**
 * Predefined themes for different entity types
 * Based on the students page design as the standard
 */
export const databaseThemes = {
  students: {
    primary: 'teal-500',
    secondary: 'blue-600',
    accent: 'blue',
    background: 'blue-50',
    border: 'blue-200',
    textAccent: 'blue-800',
    icon: <UsersIcon />,
    avatarGradient: 'from-teal-400 to-blue-500'
  },
  teachers: {
    primary: 'purple-500',
    secondary: 'indigo-600',
    accent: 'purple',
    background: 'purple-50',
    border: 'purple-200',
    textAccent: 'purple-800',
    icon: <AcademicCapIcon />,
    avatarGradient: 'from-purple-400 to-indigo-500'
  },
  rooms: {
    primary: 'green-500',
    secondary: 'emerald-600',
    accent: 'green',
    background: 'green-50',
    border: 'green-200',
    textAccent: 'green-800',
    icon: <BuildingIcon />,
    avatarGradient: 'from-green-400 to-emerald-500'
  },
  activities: {
    primary: 'orange-500',
    secondary: 'red-600',
    accent: 'orange',
    background: 'orange-50',
    border: 'orange-200',
    textAccent: 'orange-800',
    icon: <CalendarIcon />,
    avatarGradient: 'from-orange-400 to-red-500'
  },
  groups: {
    primary: 'indigo-500',
    secondary: 'purple-600',
    accent: 'indigo',
    background: 'indigo-50',
    border: 'indigo-200',
    textAccent: 'indigo-800',
    icon: <GroupIcon />,
    avatarGradient: 'from-indigo-400 to-purple-500'
  },
  roles: {
    primary: 'gray-500',
    secondary: 'slate-600',
    accent: 'gray',
    background: 'gray-50',
    border: 'gray-200',
    textAccent: 'gray-800',
    icon: <ShieldIcon />,
    avatarGradient: 'from-gray-400 to-slate-500'
  },
  devices: {
    primary: 'amber-500',
    secondary: 'orange-600',
    accent: 'amber',
    background: 'amber-50',
    border: 'amber-200',
    textAccent: 'amber-800',
    icon: <DeviceIcon />,
    avatarGradient: 'from-amber-400 to-orange-500'
  }
} as const;

export type DatabaseThemeKey = keyof typeof databaseThemes;

/**
 * Helper function to get dynamic class names based on theme
 * Note: This is for documentation purposes. In practice, Tailwind CSS
 * requires complete class names to be present in the code for proper compilation.
 */
export function getThemeClasses(theme: DatabaseTheme) {
  return {
    // Gradient classes
    headerGradient: `from-${theme.primary} to-${theme.secondary}`,
    avatarGradient: theme.avatarGradient,
    
    // Background classes
    sectionBackground: `bg-${theme.background}`,
    
    // Border classes
    sectionBorder: `border-${theme.border}`,
    
    // Text classes
    sectionTitle: `text-${theme.textAccent}`,
    
    // Complete class strings for Tailwind to pick up
    // These need to be explicitly defined for each theme
    gradientClasses: {
      'teal-500': 'from-teal-500',
      'blue-600': 'to-blue-600',
      'purple-500': 'from-purple-500',
      'indigo-600': 'to-indigo-600',
      'green-500': 'from-green-500',
      'emerald-600': 'to-emerald-600',
      'orange-500': 'from-orange-500',
      'red-600': 'to-red-600',
      'indigo-500': 'from-indigo-500',
      'purple-600': 'to-purple-600'
    }
  };
}

/**
 * Get complete Tailwind classes based on theme values
 * This ensures all dynamic classes are present in the build
 */
export function getThemeClassNames(theme: DatabaseTheme) {
  const classMap = {
    // Header gradient classes
    gradient: {
      'from-teal-500 to-blue-600': theme.primary === 'teal-500' && theme.secondary === 'blue-600',
      'from-purple-500 to-indigo-600': theme.primary === 'purple-500' && theme.secondary === 'indigo-600',
      'from-green-500 to-emerald-600': theme.primary === 'green-500' && theme.secondary === 'emerald-600',
      'from-orange-500 to-red-600': theme.primary === 'orange-500' && theme.secondary === 'red-600',
      'from-indigo-500 to-purple-600': theme.primary === 'indigo-500' && theme.secondary === 'purple-600',
      'from-gray-500 to-slate-600': theme.primary === 'gray-500' && theme.secondary === 'slate-600',
      'from-amber-500 to-orange-600': theme.primary === 'amber-500' && theme.secondary === 'orange-600',
    },
    
    // Background classes
    background: {
      'bg-blue-50': theme.background === 'blue-50',
      'bg-purple-50': theme.background === 'purple-50',
      'bg-green-50': theme.background === 'green-50',
      'bg-orange-50': theme.background === 'orange-50',
      'bg-indigo-50': theme.background === 'indigo-50',
      'bg-gray-50': theme.background === 'gray-50',
      'bg-amber-50': theme.background === 'amber-50',
    },
    
    // Border classes
    border: {
      'border-blue-200': theme.border === 'blue-200',
      'border-purple-200': theme.border === 'purple-200',
      'border-green-200': theme.border === 'green-200',
      'border-orange-200': theme.border === 'orange-200',
      'border-indigo-200': theme.border === 'indigo-200',
      'border-gray-200': theme.border === 'gray-200',
      'border-amber-200': theme.border === 'amber-200',
    },
    
    // Text classes
    text: {
      'text-blue-800': theme.textAccent === 'blue-800',
      'text-purple-800': theme.textAccent === 'purple-800',
      'text-green-800': theme.textAccent === 'green-800',
      'text-orange-800': theme.textAccent === 'orange-800',
      'text-indigo-800': theme.textAccent === 'indigo-800',
      'text-gray-800': theme.textAccent === 'gray-800',
      'text-amber-800': theme.textAccent === 'amber-800',
    }
  };
  
  return {
    gradient: Object.entries(classMap.gradient).find(([_, match]) => match)?.[0] ?? '',
    background: Object.entries(classMap.background).find(([_, match]) => match)?.[0] ?? '',
    border: Object.entries(classMap.border).find(([_, match]) => match)?.[0] ?? '',
    text: Object.entries(classMap.text).find(([_, match]) => match)?.[0] ?? '',
  };
}