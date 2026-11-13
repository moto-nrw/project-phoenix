"use client";

import { createContext, useContext, type ReactNode } from "react";

/**
 * Where the search button stands. `header`: the staff header from lg on, the
 * owner of the shortcut Cmd/Ctrl+K. `pageHead`: the magnifier in the page head
 * below lg, where the shell has no header (phone, tablet).
 */
export type SchoolSearchPlacement = "header" | "pageHead";

type RenderSchoolSearch = (placement: SchoolSearchPlacement) => ReactNode;

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
}: Readonly<{ placement: SchoolSearchPlacement }>) {
  const renderSearch = useContext(SchoolSearchSlotContext);
  return renderSearch ? renderSearch(placement) : null;
}
