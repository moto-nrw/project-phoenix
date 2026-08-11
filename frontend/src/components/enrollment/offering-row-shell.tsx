// The chrome every care-offering row shares: radius, border, padding and text
// size. Extracted so the selectable rows in the parent/manual enrollment form
// and the blocked rows in the admin adjustment dialog cannot drift apart
// (#2186 review) — the same reason FeaturePill lives beside this file.
//
// Deliberately NOT one of the kit cards: InfoCard and SectionCard are
// rounded-2xl `moto-content-surface` page-level surfaces with an icon and a
// heading, whereas this is a dense row inside a form section. It stays in
// components/enrollment/ rather than components/ui/ because "an offering row"
// is enrollment vocabulary, not a general-purpose primitive.

import type { ReactNode } from "react";

export function OfferingRowShell({
  tone,
  invalid,
  children,
}: Readonly<{
  /** Border/background utilities for this row's state. */
  tone: string;
  invalid?: boolean;
  children: ReactNode;
}>) {
  return (
    <div
      aria-invalid={invalid ? true : undefined}
      className={`rounded-lg border p-3 text-sm transition-colors ${tone}`}
    >
      {children}
    </div>
  );
}

/** The muted, dashed look every "not selectable for this child" row uses. */
export const BLOCKED_OFFERING_ROW_TONE =
  "border-dashed border-gray-300 bg-gray-50";
