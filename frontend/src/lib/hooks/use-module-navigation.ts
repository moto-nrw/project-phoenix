"use client";

import { useSession } from "next-auth/react";
import { useMemo } from "react";

import {
  moduleVisibility,
  PAGE_MODULES,
  SECTION_MODULES,
  seesLockedModules,
  type ModuleVisibility,
} from "~/lib/module-register";
import type { StaffNavSectionKey } from "~/lib/staff-navigation";
import { useBookedModules } from "~/lib/tenant-context";

export interface ModuleNavigation {
  /** Wie die Seite hinter `href` in der Navigation erscheint. */
  readonly page: (href: string) => ModuleVisibility;
  /** Wie ein aufklappbarer Bereich der Seitenleiste erscheint. */
  readonly section: (key: StaffNavSectionKey) => ModuleVisibility;
}

/**
 * Die Regel aus dem Modul-Register (#4894) für die angemeldete Person:
 * Seitenleiste und mobile Navigation fragen hier, ob ein Eintrag offen, mit
 * Schloss oder gar nicht erscheint.
 */
export function useModuleNavigation(): ModuleNavigation {
  const booked = useBookedModules();
  const { data: session } = useSession();
  const seesLocked = seesLockedModules(session);
  return useMemo(
    () => ({
      page: (href) => moduleVisibility(PAGE_MODULES[href], booked, seesLocked),
      section: (key) =>
        moduleVisibility(SECTION_MODULES[key], booked, seesLocked),
    }),
    [booked, seesLocked],
  );
}
