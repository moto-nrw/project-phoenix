"use client";

/**
 * Die Leer-Zustände der Anfragenliste (#2267). Drei verschiedene Fälle, die
 * sich nicht gleich anfühlen dürfen: nichts zu tun, nichts gefunden, oder
 * nichts entscheiden dürfen. Der dritte Fall sah vorher aus wie „alles
 * erledigt" und ließ Gruppenleitungen im Unklaren. Alle drei tragen dasselbe
 * Anfragen-Symbol; unterscheidbar bleiben sie über Überschrift und Text.
 *
 * Jeder Zustand sitzt zentriert auf einer eigenen Fläche, denn die Hülle der
 * Anfragenseite ist keine: freier Text auf dem gemusterten Grund liest sich
 * als Lücke, nicht als Zustand (#4360, wie `student-grid.tsx`).
 */

import { EmptyState } from "~/components/ui/empty-state";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { SectionCard } from "~/components/ui/section-card";
import type { RequestReviewAccess } from "~/lib/change-request-list-api";

const REQUESTS_ICON = <MotoConceptIcon concept="requests" size={40} />;

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
      <SectionCard>
        <EmptyState
          icon={REQUESTS_ICON}
          title="Sie dürfen Elternanfragen zurzeit nicht entscheiden."
          description="Die Leitung der OGS kann das erlauben. Der Schalter steht in den Einstellungen im Bereich Elternportal."
        />
      </SectionCard>
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
      : view === "open"
        ? "Neue Anfragen von Eltern erscheinen hier."
        : "Entschiedene Anfragen erscheinen hier.";
  return (
    <SectionCard>
      <EmptyState
        icon={REQUESTS_ICON}
        title={title}
        description={description}
      />
    </SectionCard>
  );
}

/** Rechte Spalte, solange in der breiten Ansicht kein Kind gewählt ist. */
export function NoCaseSelectedState() {
  return (
    <SectionCard>
      <EmptyState
        icon={REQUESTS_ICON}
        title="Wählen Sie links ein Kind."
        description="Dann sehen Sie hier alle Anfragen dieses Kindes und können entscheiden."
      />
    </SectionCard>
  );
}
