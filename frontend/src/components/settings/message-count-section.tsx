"use client";

import { useEffect, useState } from "react";
import { MessageCircle } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { Radio } from "~/components/ui/radio";
import { Skeleton } from "~/components/ui/skeleton";
import { SectionCard } from "~/components/ui/section-card";
import { createLogger } from "~/lib/logger";
import {
  fetchMessageCountSetting,
  saveMessageCountScope,
  type MessageCountScope,
} from "~/lib/parent-messages-api";
import { useTenantSafe } from "~/lib/tenant-context";

const logger = createLogger({ component: "MessageCountSection" });

const OPTIONS: ReadonlyArray<{
  value: MessageCountScope;
  label: string;
  description: string;
}> = [
  {
    value: "all",
    label: "Alle Nachrichten",
    description: "Jede neue Nachricht von Eltern zählt.",
  },
  {
    value: "own_groups",
    label: "Nur Kinder aus meinen Gruppen",
    description: "Nachrichten zu anderen Kindern zählen nicht.",
  },
  {
    value: "none",
    label: "Keine Zahl anzeigen",
    description: "Die Nachrichten stehen trotzdem unter „Nachrichten“.",
  },
];

const NO_GROUP_HINT =
  "Sie gehören zurzeit zu keiner Gruppe. Dann zeigt die Zahl nichts.";

/**
 * Personal count scope for parent messages (#3673): which conversations the
 * own counter at "Nachrichten" counts. It changes only this person's number;
 * the inbox, colleagues and parents see no change.
 *
 * Self-service like the birthday switch: a colleague who does not answer
 * parent messages turns the number off here instead of clearing it every day.
 * The card hides itself while the school has messaging off, and for accounts
 * the backend does not answer (no staff read access).
 */
export function MessageCountSection() {
  const messagingEnabled = useTenantSafe()?.tenant?.messagingEnabled === true;
  const [scope, setScope] = useState<MessageCountScope>("all");
  const [hasOwnGroups, setHasOwnGroups] = useState(true);
  const [loading, setLoading] = useState(true);
  const [available, setAvailable] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!messagingEnabled) return;
    let cancelled = false;
    void (async () => {
      try {
        const setting = await fetchMessageCountSetting();
        if (cancelled) return;
        setScope(setting.scope);
        setHasOwnGroups(setting.hasOwnGroups);
      } catch (err) {
        if (!cancelled) setAvailable(false);
        logger.info("message_count_scope_unavailable", {
          error: err instanceof Error ? err.message : String(err),
        });
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [messagingEnabled]);

  const choose = async (next: MessageCountScope) => {
    if (next === scope || busy) return;
    // Optimistic: the choice answers immediately and rolls back on failure.
    const previous = scope;
    setBusy(true);
    setScope(next);
    setError(null);
    try {
      await saveMessageCountScope(next);
      window.dispatchEvent(new CustomEvent("messages-unread-refresh"));
    } catch (err) {
      logger.error("message_count_scope_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setScope(previous);
      setError("Die Einstellung konnte nicht gespeichert werden.");
    } finally {
      setBusy(false);
    }
  };

  if (!messagingEnabled || !available) return null;

  return (
    <SectionCard
      icon={MessageCircle}
      headingLevel={3}
      title="Zahl bei Nachrichten"
      description="Die Zahl bei „Nachrichten“ zeigt neue Nachrichten von Eltern. Die Einstellung gilt nur für Sie."
    >
      {error && (
        <div className="mb-3">
          <Alert type="error" message={error} />
        </div>
      )}

      {loading ? (
        <Skeleton className="h-36 w-full" />
      ) : (
        <fieldset className="space-y-2">
          <legend className="sr-only">Was die Zahl zählt</legend>
          {OPTIONS.map((option) => {
            const id = `message-count-scope-${option.value}`;
            const selected = scope === option.value;
            return (
              <ChoiceTile
                key={option.value}
                htmlFor={id}
                selected={selected}
                disabled={busy}
                className="items-start p-3"
              >
                <Radio
                  id={id}
                  name="message-count-scope"
                  value={option.value}
                  checked={selected}
                  disabled={busy}
                  onChange={() => void choose(option.value)}
                  className="mt-0.5"
                />
                <span className="min-w-0 flex-1">
                  <span className="block text-sm font-medium text-gray-900">
                    {option.label}
                  </span>
                  <span className="mt-0.5 block text-xs font-normal text-gray-600">
                    {option.description}
                  </span>
                  {option.value === "own_groups" && !hasOwnGroups && (
                    <span className="mt-1 block text-xs font-normal text-gray-600">
                      {NO_GROUP_HINT}
                    </span>
                  )}
                </span>
              </ChoiceTile>
            );
          })}
        </fieldset>
      )}
    </SectionCard>
  );
}
