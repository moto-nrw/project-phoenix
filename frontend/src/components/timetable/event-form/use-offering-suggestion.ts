"use client";

import { useEffect, useState } from "react";

import { createLogger } from "~/lib/logger";
import type {
  OfferingSuggestion,
  OfferingSuggestionQuery,
} from "~/lib/offering-suggestion";
import { timetableService } from "~/lib/timetable-api";

const logger = createLogger({ component: "useOfferingSuggestion" });

/** Wait this long after the last change before asking the backend. */
const SUGGESTION_DEBOUNCE_MS = 400;

/**
 * Asks which offering matches the children the editor plans (#5104). It
 * waits until the selection rests, so typing never makes the line flicker.
 * The suggestion is information only: a failed read shows nothing and is
 * logged. A null query clears the suggestion at once.
 */
export function useOfferingSuggestion(
  query: OfferingSuggestionQuery | null,
): OfferingSuggestion | null {
  const [suggestion, setSuggestion] = useState<OfferingSuggestion | null>(null);
  const key = query ? JSON.stringify(query) : null;

  useEffect(() => {
    if (key === null) return;
    const current = JSON.parse(key) as OfferingSuggestionQuery;
    let active = true;
    const timer = setTimeout(() => {
      timetableService
        .getOfferingSuggestion(current)
        .then((answer) => {
          if (active) setSuggestion(answer);
        })
        .catch((err: unknown) => {
          // Bewusst ohne Anzeige: der Vorschlag ist nur ein Hinweis, der
          // Regeltermin lässt sich ohne ihn genauso anlegen.
          logger.warn("offering_suggestion_failed", {
            period_id: current.periodId,
            error: err instanceof Error ? err.message : String(err),
          });
          if (active) setSuggestion(null);
        });
    }, SUGGESTION_DEBOUNCE_MS);
    return () => {
      active = false;
      clearTimeout(timer);
    };
  }, [key]);

  // While the next answer is pending the last one stays, so a changed
  // weekday does not make the line vanish and come back.
  return key === null ? null : suggestion;
}
