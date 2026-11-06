"use client";

import {
  createContext,
  useContext,
  type ComponentType,
  type ReactNode,
} from "react";

import type { ModuleKey } from "~/lib/modules";

/**
 * How the kit shows a module the school did not book (#4894). The explanation
 * reads the session and the portal, so it lives in `components/tenant/`; the
 * kit only leaves a place for it (#5586). Without a provider `FormErrorAlert`
 * and `TenantPage` show the ordinary error, and a page counts as open.
 */
export interface NotBookedSlots {
  readonly Alert: ComponentType<{
    readonly module: ModuleKey;
    readonly className?: string;
  }>;
  readonly State: ComponentType<{ readonly module: ModuleKey }>;
}

const NotBookedSlotContext = createContext<NotBookedSlots | null>(null);

export function NotBookedSlotProvider({
  slots,
  children,
}: {
  readonly slots: NotBookedSlots;
  readonly children: ReactNode;
}) {
  return (
    <NotBookedSlotContext.Provider value={slots}>
      {children}
    </NotBookedSlotContext.Provider>
  );
}

export function useNotBookedSlots(): NotBookedSlots | null {
  return useContext(NotBookedSlotContext);
}
