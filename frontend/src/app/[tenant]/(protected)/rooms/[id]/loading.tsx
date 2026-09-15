"use client";

import { RoomDetailLoadingPage } from "./page-skeleton";

/**
 * Route-level loading UI: dasselbe Gerüst, das die Seite zeigt, während der
 * Raum lädt, damit der Wechsel ein durchgehendes Skelett ist.
 */
export default function RoomDetailLoading() {
  return <RoomDetailLoadingPage />;
}
