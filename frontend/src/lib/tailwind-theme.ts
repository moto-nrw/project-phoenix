/**
 * Project Phoenix Tailwind Theme Functions
 *
 * This file provides functions for integrating the design tokens
 * with Tailwind CSS configuration.
 */

import { theme } from "./theme";

/**
 * Converts the theme colors to a format compatible with Tailwind colors configuration
 */
export function getThemeColorsTailwind() {
  const colors: Record<string, Record<string, string>> = {};

  // Process primary colors
  colors.primary = {
    500: theme.colors.primary[500],
    600: theme.colors.primary[600],
    800: theme.colors.primary[800],
  };

  // Process secondary colors
  colors.secondary = {
    500: theme.colors.secondary[500],
    600: theme.colors.secondary[600],
    700: theme.colors.secondary[700],
  };

  // Process gray
  colors.gray = {
    50: theme.colors.gray[50],
    100: theme.colors.gray[100],
    200: theme.colors.gray[200],
    300: theme.colors.gray[300],
    500: theme.colors.gray[500],
    600: theme.colors.gray[600],
    700: theme.colors.gray[700],
    800: theme.colors.gray[800],
    900: theme.colors.gray[900],
  };

  // Map other color categories
  colors.success = theme.colors.success;
  colors.warning = theme.colors.warning;
  colors.error = theme.colors.error;
  colors.info = theme.colors.info;

  return colors;
}

const tailwindTheme = {
  getThemeColorsTailwind,
};

export default tailwindTheme;
