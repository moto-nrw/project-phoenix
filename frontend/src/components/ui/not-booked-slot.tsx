"use client";

import {
  createContext,
  useContext,
  type ComponentType,
  type ReactNode,
} from "react";

import type { ModuleKey } from "~/lib/modules";

/**
 * How the kit shows a module the school did not book (#4894). Which page
 * belongs to an unbooked module and how the explanation reads depend on the
 * session, the school and the portal, so `components/tenant/` provides them;
 * the kit only leaves a place for them (#5586, #6577).
 *
 * `PageBody` is the one place that renders the state: for the page's own
 * module (`pageModule`) and for a load refused with `module.not_booked`.
 * `TenantPage` then keeps only its head; `FormErrorAlert` and
 * `LoadErrorAlert` show `Alert`. Without a provider they show the ordinary
 * error, and a page counts as open.
 */
export interface NotBookedSlots {
  readonly Alert: ComponentType<{
    readonly module: ModuleKey;
    readonly className?: string;
  }>;
  readonly State: ComponentType<{ readonly module: ModuleKey }>;
  /** The unbooked module the current page belongs to, else absent or `null`. */
  readonly pageModule?: ModuleKey | null;
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

/** The unbooked module of the current page, else `null`. */
export function useNotBookedPageModule(): ModuleKey | null {
  return useNotBookedSlots()?.pageModule ?? null;
}
