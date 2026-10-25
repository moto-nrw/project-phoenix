"use client";

import { createContext, useContext } from "react";

import type { ModulePortal } from "~/lib/portal-not-booked";

const ModulePortalContext = createContext<ModulePortal>("staff");

/**
 * Names the portal a refused module is explained in (#4895). The parents
 * and school providers set it; everything else is the OGS team portal.
 */
export function ModulePortalProvider({
  portal,
  children,
}: Readonly<{ portal: ModulePortal; children: React.ReactNode }>) {
  return (
    <ModulePortalContext.Provider value={portal}>
      {children}
    </ModulePortalContext.Provider>
  );
}

export function useModulePortal(): ModulePortal {
  return useContext(ModulePortalContext);
}
