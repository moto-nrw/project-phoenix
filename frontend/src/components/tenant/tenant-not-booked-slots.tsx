"use client";

import dynamic from "next/dynamic";
import type { ReactNode } from "react";

import { NotBookedSlotProvider } from "~/components/ui/not-booked-slot";

// Loaded on demand: the root providers stay auth-free, and the explanation
// (session, module register) only loads when a module really is not booked.
const SLOTS = {
  Alert: dynamic(() =>
    import("./module-not-booked-state").then((m) => m.ModuleNotBookedAlert),
  ),
  State: dynamic(() =>
    import("./module-not-booked-state").then((m) => m.ModuleNotBookedState),
  ),
};

export function TenantNotBookedSlots({
  children,
}: {
  readonly children: ReactNode;
}) {
  return (
    <NotBookedSlotProvider slots={SLOTS}>{children}</NotBookedSlotProvider>
  );
}
