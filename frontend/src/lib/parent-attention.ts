import type { AggregatedOpenRequest } from "~/lib/change-request-list-api";
import { listAggregatedOpenRequests } from "~/lib/change-request-list-api";
import { createLogger } from "~/lib/logger";
import type { InboxThread } from "~/lib/parent-messages-api";

const logger = createLogger({ component: "ParentAttention" });

/**
 * Was auf der Kinderkachel als blaue Zahl steht (#6486): neue Nachrichten der
 * Eltern und offene Anfragen, die die Person entscheiden darf.
 */
export interface ParentAttention {
  readonly messages: number;
  readonly requests: number;
  readonly total: number;
}

/**
 * Zählt pro Kind. Die Nachrichten kommen aus dem Posteingang „Nur ungelesen“,
 * also mit genau den Zahlen, die dort an jeder Unterhaltung stehen. Eine neue
 * Anfrage zählt dort nie mit; sie kommt aus der offenen Anfragenliste, so wird
 * nichts doppelt gezählt.
 */
export function countParentAttention(
  unreadThreads: readonly InboxThread[],
  openRequests: readonly AggregatedOpenRequest[],
): ReadonlyMap<string, ParentAttention> {
  const messages = new Map<string, number>();
  for (const thread of unreadThreads) {
    if (thread.unread_count <= 0) continue;
    messages.set(
      thread.student_id,
      (messages.get(thread.student_id) ?? 0) + thread.unread_count,
    );
  }
  const requests = new Map<string, number>();
  for (const request of openRequests) {
    // Anmeldungsänderungen hängen an keinem Kind dieser Liste.
    if (!("student_id" in request)) continue;
    requests.set(
      request.student_id,
      (requests.get(request.student_id) ?? 0) + 1,
    );
  }
  const result = new Map<string, ParentAttention>();
  for (const studentId of new Set([...messages.keys(), ...requests.keys()])) {
    const messageCount = messages.get(studentId) ?? 0;
    const requestCount = requests.get(studentId) ?? 0;
    result.set(studentId, {
      messages: messageCount,
      requests: requestCount,
      total: messageCount + requestCount,
    });
  }
  return result;
}

/**
 * Obergrenze für die Seiten der offenen Anfragenliste (je 25). Mehr offene
 * Anfragen hat keine Schule; wird sie doch erreicht, fehlt die Zahl nur an
 * den Kindern der letzten Anfragen, und das Log sagt es.
 */
const MAX_OPEN_REQUEST_PAGES = 20;

/** Alle offenen Anfragen, die die Person sehen darf, über alle Seiten. */
export async function fetchAllOpenRequests(): Promise<AggregatedOpenRequest[]> {
  const items: AggregatedOpenRequest[] = [];
  let cursor: string | undefined;
  for (let page = 0; page < MAX_OPEN_REQUEST_PAGES; page++) {
    const result = await listAggregatedOpenRequests(cursor ? { cursor } : {});
    items.push(...result.items);
    cursor = result.next_cursor;
    if (!cursor) return items;
  }
  logger.warn("parent_attention_open_requests_truncated", {
    pages: MAX_OPEN_REQUEST_PAGES,
  });
  return items;
}
