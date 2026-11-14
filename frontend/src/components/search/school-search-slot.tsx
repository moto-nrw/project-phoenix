"use client";

import { createContext, useContext, type ReactNode } from "react";

/**
 * Where the search button stands. `header`: the staff header from lg on.
 * `pageHead`: the magnifier in the page head of a tablet (sm to lg), where the
 * shell has no header. `moreSheet`: the field at the top of the phone's
 * "Mehr" sheet; below sm the page head keeps its room for the page.
 */
export type SchoolSearchPlacement = "header" | "pageHead" | "moreSheet";

/** What the place around the button needs when the search opens. */
export interface SchoolSearchSlotOptions {
  /** Runs before the search opens: the "Mehr" sheet closes itself. */
  readonly onOpen?: () => void;
  /**
   * The element that gets the focus when the search closes, when the
   * button itself is gone by then (the sheet's own trigger).
   */
  readonly returnFocus?: () => HTMLElement | null;
}

type RenderSchoolSearch = (
  placement: SchoolSearchPlacement,
  options: SchoolSearchSlotOptions,
) => ReactNode;

/**
 * The staff layout puts its search button into this slot (#6954). The header
 * and the page head are shared with the parents, caterer, Träger and school
 * portals; they render the slot and never import the search, so its code stays
 * out of those portals' bundles.
 */
export const SchoolSearchSlotContext = createContext<RenderSchoolSearch | null>(
  null,
);

/** Renders the search button of the staff layout, or nothing elsewhere. */
export function SchoolSearchSlot({
  placement,
  onOpen,
  returnFocus,
}: Readonly<{ placement: SchoolSearchPlacement } & SchoolSearchSlotOptions>) {
  const renderSearch = useContext(SchoolSearchSlotContext);
  return renderSearch ? renderSearch(placement, { onOpen, returnFocus }) : null;
}
