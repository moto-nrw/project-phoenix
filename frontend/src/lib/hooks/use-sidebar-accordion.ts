"use client";

import { useState, useCallback, useEffect } from "react";

export type AccordionSection = "groups" | "supervisions" | "database" | null;

const STORAGE_KEY = "sidebar-accordion-expanded";

/**
 * Derives which accordion section should be expanded based on the current pathname.
 */
function sectionFromPathname(pathname: string): AccordionSection {
  if (pathname.startsWith("/ogs-groups")) return "groups";
  if (pathname.startsWith("/active-supervisions")) return "supervisions";
  if (pathname.startsWith("/database")) return "database";
  return null;
}

/**
 * Hook to manage exclusive accordion expansion in the sidebar.
 * Only one section can be open at a time.
 * Persists the expanded section to localStorage.
 * Auto-expands from pathname when navigating to a sub-page.
 */
export function useSidebarAccordion(pathname: string) {
  const [expanded, setExpanded] = useState<AccordionSection>(() => {
    // Pathname takes priority over localStorage on initial render
    const fromPath = sectionFromPathname(pathname);
    if (fromPath) return fromPath;

    // Fall back to localStorage
    if (typeof window !== "undefined") {
      const stored = localStorage.getItem(STORAGE_KEY);
      if (
        stored === "groups" ||
        stored === "supervisions" ||
        stored === "database"
      ) {
        return stored;
      }
    }
    return null;
  });

  // Auto-expand/collapse when navigating:
  // - Navigate to accordion page → expand that section
  // - Navigate to non-accordion page → collapse all
  useEffect(() => {
    const fromPath = sectionFromPathname(pathname);
    if (fromPath !== expanded) {
      setExpanded(fromPath);
    }
    // Only react to pathname changes, not expanded changes
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pathname]);

  // Persist to localStorage whenever expanded changes
  useEffect(() => {
    if (typeof window !== "undefined") {
      if (expanded) {
        localStorage.setItem(STORAGE_KEY, expanded);
      } else {
        localStorage.removeItem(STORAGE_KEY);
      }
    }
  }, [expanded]);

  const toggle = useCallback((section: AccordionSection) => {
    setExpanded((prev) => (prev === section ? null : section));
  }, []);

  return { expanded, toggle };
}
