"use client";

import { AnnouncementDetailLoadingPage } from "./page-skeleton";

/**
 * Route-level loading UI: dasselbe Gerüst, das die Seite zeigt, während die
 * Mitteilung lädt, damit der Wechsel ein durchgehendes Skelett ist.
 */
export default function AnnouncementDetailLoading() {
  return <AnnouncementDetailLoadingPage />;
}
