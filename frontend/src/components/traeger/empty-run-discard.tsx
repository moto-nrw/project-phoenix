// Ein offener Lauf ohne Inhalt lässt sich verwerfen (#6626): Monatslauf und
// Jahreslauf zeigen dieselbe Karte und nach dem Verwerfen
// denselben Abschluss. Ob ein Lauf leer ist, entscheidet der Server unter der
// Sperre der Freigabe; die Seite blendet die Aktion nur bei leerer Vorschau
// ein.

import { ArchiveX, Inbox } from "lucide-react";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import { formatDate } from "~/lib/date-helpers";

/** What a run covers, in the words of its page. */
export interface RunPeriod {
  /** „Monatslauf“ or „Jahreslauf“. */
  readonly runLabel: string;
  /** „Januar 2030“ or „das Jahr 2030“, as it reads after „für“. */
  readonly period: string;
  /** The span the facts name: „Januar 2030“, „01.09.–31.12.2030“. */
  readonly range: string;
  /** „der Monat“ or „das Jahr“, as it reads before „ist frei“. */
  readonly periodNoun: string;
  /** „keine Rechnung“ or „keinen Bescheid“, what the run would issue. */
  readonly nothing: string;
  /** The names of its schools, „–“ without one. */
  readonly schools: string;
  readonly createdAt: string;
}

function RunFacts({
  run,
  discardedAt,
}: {
  readonly run: RunPeriod;
  readonly discardedAt?: string | null;
}) {
  return (
    <div className="mt-2 border-t border-gray-100 pt-5">
      <DataGrid columns={2}>
        <DataField label="Zeitraum">{run.range}</DataField>
        <DataField label="Schulen">{run.schools}</DataField>
        <DataField label="Gestartet am">{formatDate(run.createdAt)}</DataField>
        {discardedAt === undefined ? null : (
          <DataField label="Verworfen am">
            {discardedAt ? formatDate(discardedAt) : "–"}
          </DataField>
        )}
      </DataGrid>
    </div>
  );
}

/** The body of an open run that has nothing to issue. */
export function EmptyRunCard({ run }: { readonly run: RunPeriod }) {
  return (
    <SectionCard title={run.runLabel}>
      <EmptyState
        tone="neutral"
        icon={<Inbox className="h-7 w-7" aria-hidden="true" />}
        title="Dieser Lauf hat nichts abzurechnen"
        description={`Für ${run.period} gibt es ${run.nothing}. Es fehlt auch kein Preis. Sie können den Lauf verwerfen. Danach ist ${run.periodNoun} frei für einen neuen Lauf.`}
        className="py-6"
      />
      <RunFacts run={run} />
    </SectionCard>
  );
}

/** The body of a discarded run: what it was and when it was discarded. */
export function DiscardedRunCard({
  run,
  discardedAt,
}: {
  readonly run: RunPeriod;
  readonly discardedAt: string | null;
}) {
  return (
    <SectionCard title={run.runLabel}>
      <EmptyState
        tone="neutral"
        icon={<ArchiveX className="h-7 w-7" aria-hidden="true" />}
        title="Dieser Lauf ist verworfen"
        description={`Er hatte nichts abzurechnen und hat nichts ausgestellt. Für ${run.period} können Sie einen neuen Lauf starten.`}
        className="py-6"
      />
      <RunFacts run={run} discardedAt={discardedAt} />
    </SectionCard>
  );
}
