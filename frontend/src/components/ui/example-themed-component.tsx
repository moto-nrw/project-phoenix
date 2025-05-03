"use client";

/**
 * Example component showing how to use the theme configuration directly
 * This showcases using theme values directly in the component
 */
export function ExampleThemedComponent() {
  // Hardcoded values from the theme.ts file (without importing directly)
  // This is just an example for styling components directly with values from theme
  const styles = {
    container: {
      padding: "1rem",
      borderRadius: "0.75rem",
      backgroundColor: "rgba(255, 255, 255, 0.95)",
      boxShadow:
        "0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04)",
      maxWidth: "28rem", // equiv to max-w-md
    },
    heading: {
      fontSize: "1.875rem",
      fontWeight: "700",
      color: "#0d9488",
      marginBottom: "1rem",
    },
    paragraph: {
      fontSize: "1rem",
      color: "#4b5563",
      marginBottom: "1.5rem",
    },
    button: {
      backgroundColor: "#0d9488",
      color: "#ffffff",
      padding: "0.75rem 1rem",
      borderRadius: "0.5rem",
      fontFamily: '"Geist Sans", ui-sans-serif, system-ui, sans-serif',
      fontSize: "0.875rem",
      fontWeight: "500",
      boxShadow:
        "0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06)",
      transition: "all 0.2s ease",
    },
  };

  return (
    <div style={styles.container}>
      <h2 style={styles.heading}>Theme Example</h2>
      <p style={styles.paragraph}>
        This component demonstrates using the theme values directly with inline
        styles. For most components, you&apos;ll likely use the theme with
        Tailwind classes.
      </p>
      <button style={styles.button}>Themed Button</button>
    </div>
  );
}
