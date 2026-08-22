/**
 * Zusammenführung mehrerer Anfrage-Quellen zu EINER Liste (#2435).
 *
 * Der Eltern-Reiter zeigt Anfragen aus zwei Endpunkten: die vier
 * users:update-Arten aus dem Aggregator und die Anmeldungsänderungen, die
 * hinter config:manage liegen und deshalb einen eigenen Endpunkt haben. Beide
 * liefern Seiten in derselben Reihenfolge (neueste zuerst) und stempeln jede
 * Zeile mit `occurred_at` — dem Zeitpunkt, nach dem sortiert wird
 * (Einreichung in der Arbeitsliste, Entscheidung in der Historie).
 *
 * Hier werden sie wie beim Sortieren zweier vorsortierter Stapel verschränkt:
 * Es wird immer die Zeile ausgegeben, deren `occurred_at` am spätesten liegt.
 * Jede Quelle behält dabei ihren eigenen Cursor, damit „Weitere Einträge
 * laden" genau dort weitermacht, wo die jeweilige Quelle aufgehört hat.
 */

export interface FeedPage<T> {
  readonly items: readonly T[];
  readonly next_cursor?: string;
}

export interface FeedSource<T> {
  /** Stabiler Schlüssel der Quelle, nur zur Zustandsführung. */
  readonly key: string;
  readonly fetchPage: (cursor?: string) => Promise<FeedPage<T>>;
}

/** Jede Zeile trägt den Zeitpunkt, nach dem zusammengeführt wird. */
export interface FeedItem {
  readonly occurred_at: string;
}

interface SourceState<T> {
  buffer: T[];
  cursor?: string;
  exhausted: boolean;
}

export type FeedState<T> = Record<string, SourceState<T>>;

export function createFeedState<T>(
  sources: readonly FeedSource<T>[],
): FeedState<T> {
  const state: FeedState<T> = {};
  for (const source of sources) {
    state[source.key] = { buffer: [], exhausted: false };
  }
  return state;
}

/**
 * Wie viele Folgeseiten je Quelle nachgezogen werden, solange eine Antwort
 * leer bleibt aber noch einen Cursor trägt. Das passiert beim Aggregator, wenn
 * enge Filter fast alles verwerfen; ohne diese Nachläufe müsste man blind auf
 * „Weitere Einträge laden" drücken.
 */
const MAX_REFILLS_PER_SOURCE = 4;

async function refill<T>(
  source: FeedSource<T>,
  state: SourceState<T>,
): Promise<void> {
  for (
    let attempt = 0;
    state.buffer.length === 0 &&
    !state.exhausted &&
    attempt < MAX_REFILLS_PER_SOURCE;
    attempt++
  ) {
    const page = await source.fetchPage(state.cursor);
    state.buffer.push(...page.items);
    state.cursor = page.next_cursor;
    state.exhausted = !page.next_cursor;
  }
}

/**
 * Nimmt bis zu `limit` Zeilen über alle Quellen hinweg ab, neueste zuerst, und
 * schreibt den Fortschritt in `state`. `hasMore` sagt, ob noch etwas
 * nachzuladen ist — gepuffert oder serverseitig.
 */
export async function takeMergedPage<T extends FeedItem>(
  sources: readonly FeedSource<T>[],
  state: FeedState<T>,
  limit: number,
): Promise<{ items: T[]; hasMore: boolean }> {
  const items: T[] = [];
  while (items.length < limit) {
    let bestSource: SourceState<T> | null = null;
    let bestTime = Number.NEGATIVE_INFINITY;
    for (const source of sources) {
      const sourceState = state[source.key];
      if (!sourceState) continue;
      await refill(source, sourceState);
      const head = sourceState.buffer[0];
      if (!head) continue;
      // Über Date.parse verglichen, nicht als Zeichenkette: beide Endpunkte
      // liefern ISO-Zeitpunkte, aber nicht zwingend mit derselben Zeitzone.
      const headTime = Date.parse(head.occurred_at);
      if (bestSource === null || headTime > bestTime) {
        bestSource = sourceState;
        bestTime = headTime;
      }
    }
    if (bestSource === null) break;
    items.push(bestSource.buffer.shift()!);
  }
  const hasMore = sources.some((source) => {
    const sourceState = state[source.key];
    if (!sourceState) return false;
    return sourceState.buffer.length > 0 || !sourceState.exhausted;
  });
  return { items, hasMore };
}
