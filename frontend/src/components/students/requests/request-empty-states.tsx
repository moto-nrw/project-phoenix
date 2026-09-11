"use client";

/**
 * Die Leer-Zustände der Anfragenliste (#2267). Drei verschiedene Fälle, die
 * sich nicht gleich anfühlen dürfen: nichts zu tun, nichts gefunden, oder
 * nichts entscheiden dürfen. Der dritte Fall sah vorher aus wie „alles
 * erledigt" und ließ Gruppenleitungen im Unklaren.
 */

import { TrayIcon } from "@phosphor-icons/react/ssr";

import { EmptyState } from "~/components/ui/empty-state";
import type { RequestReviewAccess } from "~/lib/change-request-list-api";

export function RequestEmptyState({
  view,
  hasMore,
  hasActiveFilters,
  reviewAccess,
}: Readonly<{
  view: "open" | "history";
  hasMore: boolean;
  hasActiveFilters: boolean;
  reviewAccess?: RequestReviewAccess;
}>) {
  if (view === "open" && reviewAccess === "none") {
    return (
      <EmptyState
        icon={<TrayIcon size={32} aria-hidden="true" />}
        title="Sie dürfen Elternanfragen zurzeit nicht entscheiden."
        description="Die Leitung der OGS kann das erlauben. Der Schalter steht in den Einstellungen im Bereich Elternportal."
        variant="compact"
      />
    );
  }
  const title = hasMore
    ? "Hier ist noch nichts gefunden."
    : view === "open"
      ? "Keine offenen Anfragen."
      : "Noch keine entschiedenen Anfragen.";
  const description = hasMore
    ? "Ältere Einträge sind noch nicht geladen. Mit „Weitere Einträge laden“ weitersuchen."
    : hasActiveFilters
      ? "Für die aktuelle Suche und Filter gibt es keine Treffer."
      : undefined;
  return (
    <EmptyState
      icon={<TrayIcon size={32} aria-hidden="true" />}
      title={title}
      description={description}
      variant="compact"
    />
  );
}

/** Rechte Spalte, solange in der breiten Ansicht kein Kind gewählt ist. */
export function NoCaseSelectedState() {
  return (
    <EmptyState
      icon={<TrayIcon size={32} aria-hidden="true" />}
      title="Wählen Sie links ein Kind."
      description="Dann sehen Sie hier alle Anfragen dieses Kindes und können entscheiden."
      variant="compact"
    />
  );
}
