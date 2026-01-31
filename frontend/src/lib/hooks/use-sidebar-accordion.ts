"use client";

import { useState, useCallback, useEffect } from "react";

export type AccordionSection = "groups" | "supervisions" | "database" | null;

const STORAGE_KEY = "sidebar-accordion-expanded";

/**
 * Derives which accordion section should be expanded based on the current pathname.
 * Also considers the `from` query param for child pages (e.g. student detail)
 * so the accordion stays open when drilling down from an accordion section.
 */
function sectionFromPathname(
  pathname: string,
  fromParam?: string | null,
): AccordionSection {
  if (pathname.startsWith("/ogs-groups")) return "groups";
  if (pathname.startsWith("/active-supervisions")) return "supervisions";
  if (pathname.startsWith("/database")) return "database";

  // Child pages: keep the originating accordion section open
  if (fromParam) {
    if (fromParam.startsWith("/ogs-groups")) return "groups";
    if (fromParam.startsWith("/active-supervisions")) return "supervisions";
    if (fromParam.startsWith("/database")) return "database";
  }

  return null;
}

/**
 * Hook to manage exclusive accordion expansion in the sidebar.
 * Only one section can be open at a time.
 * Persists the expanded section to localStorage.
 * Auto-expands from pathname when navigating to a sub-page.
 *
 * @param pathname - Current route pathname
 * @param fromParam - Value of the `from` search param (for child page navigation)
 */
export function useSidebarAccordion(
  pathname: string,
  fromParam?: string | null,
) {
  // Initialize from pathname only (safe for SSR — no localStorage during render)
  const [expanded, setExpanded] = useState<AccordionSection>(() =>
    sectionFromPathname(pathname, fromParam),
  );

  // Restore from localStorage on mount when pathname doesn't determine a section.
  // This runs client-only after hydration, avoiding SSR/client mismatches.
  useEffect(() => {
    if (sectionFromPathname(pathname, fromParam)) return; // pathname already decided
    const stored = localStorage.getItem(STORAGE_KEY);
    if (
      stored === "groups" ||
      stored === "supervisions" ||
      stored === "database"
    ) {
      setExpanded(stored);
    }
    // Only run on mount
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Auto-expand/collapse when navigating:
  // - Navigate to accordion page → expand that section
  // - Navigate to child page with ?from= → keep that section open
  // - Navigate to unrelated page → collapse all
  useEffect(() => {
    const fromPath = sectionFromPathname(pathname, fromParam);
    if (fromPath !== expanded) {
      setExpanded(fromPath);
    }
    // Only react to pathname/fromParam changes, not expanded changes
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pathname, fromParam]);

  // Persist to localStorage whenever expanded changes
  useEffect(() => {
    if (expanded) {
      localStorage.setItem(STORAGE_KEY, expanded);
    } else {
      localStorage.removeItem(STORAGE_KEY);
    }
  }, [expanded]);

  const toggle = useCallback((section: AccordionSection) => {
    setExpanded((prev) => (prev === section ? null : section));
  }, []);

  return { expanded, toggle };
}
