"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import {
  RequestReviewCard,
  ReviewDiffPanel,
} from "~/components/students/request-review-card";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { Checkbox } from "~/components/ui/checkbox";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  OfferingRequestApiError,
  type OfferingRequestDiffLine,
  type StaffOfferingRequest,
  decideOfferingChangeRequest,
  listOfferingChangeRequests,
} from "~/lib/offering-request-review-api";

const logger = createLogger({ component: "OfferingRequestReviewList" });

// An approval genuinely applies the switch, so it can fail for reasons the
// office has to act on rather than retry. Name each one and say what to do; the
// row deliberately stays pending in all of these cases.
function decideErrorMessage(code: string | undefined): string {
  switch (code) {
    case "offering_change_capacity_full":
      return "Für ein gewünschtes Angebot ist kein Platz mehr frei. Die Anfrage bleibt offen: Bitte mit der Familie eine Alternative klären oder die Anfrage mit Begründung ablehnen.";
    case "change_request_not_pending":
      return "Diese Anfrage wurde bereits entschieden oder von den Eltern zurückgezogen. Bitte die Seite neu laden.";
    case "offering_changes_no_enrollment":
      return "Für dieses Kind liegt keine gültige Anmeldung mehr vor, auf die die Änderung angewendet werden könnte. Bitte die Anfrage ablehnen.";
    default:
      return "Die Entscheidung konnte nicht gespeichert werden.";
  }
}

// joinNames renders trigger names as „A“ und „B“ for the explanation line.
function joinNames(names: readonly string[]): string {
  return names.map((name) => `„${name}“`).join(" und ");
}

// automaticHint says why a line appeared. Kept to one short sentence: the
// review card must be readable at a glance.
function automaticHint(entry: OfferingRequestDiffLine): string {
  const names = entry.trigger_names ?? [];
  const partial =
    entry.automatic_days !== undefined && entry.automatic_days !== entry.new;
  if (names.length === 0) {
    return partial
      ? `Die Tage ${entry.automatic_days} kommen automatisch dazu.`
      : "Kommt automatisch dazu.";
  }
  return partial
    ? `Die Tage ${entry.automatic_days} kommen automatisch dazu, weil ${joinNames(names)} gewählt ist.`
    : `Kommt automatisch dazu, weil ${joinNames(names)} gewählt ist.`;
}

// removedOfferings resolves the cascade of unticked rule lines: a line whose
// NEW side is fully automatic falls away once every offering that triggers it
// is itself removed. Triggers outside the diff count as still selected. The
// server recomputes this authoritatively on approval; the preview mirrors it.
function removedOfferings(
  diff: readonly OfferingRequestDiffLine[],
  excluded: readonly string[],
): Set<string> {
  const removed = new Set(excluded);
  let changed = true;
  while (changed) {
    changed = false;
    for (const entry of diff) {
      if (removed.has(entry.offering_id)) continue;
      const triggers = entry.trigger_ids ?? [];
      if (!entry.automatic || triggers.length === 0) continue;
      if (entry.automatic_days !== entry.new) continue;
      if (triggers.every((id) => removed.has(id))) {
        removed.add(entry.offering_id);
        changed = true;
      }
    }
  }
  return removed;
}

// cascadeSourceName names the unticked line a greyed dependent hangs on.
function cascadeSourceName(
  entry: OfferingRequestDiffLine,
  diff: readonly OfferingRequestDiffLine[],
  removed: ReadonlySet<string>,
): string {
  for (const id of entry.trigger_ids ?? []) {
    if (!removed.has(id)) continue;
    const trigger = diff.find((line) => line.offering_id === id);
    if (trigger) return trigger.label;
  }
  return "";
}

// German-only staff UI — hardcoded strings like the sibling review lists (the
// staff shell ships no full message catalog).
export function OfferingRequestReviewList() {
  const [rows, setRows] = useState<StaffOfferingRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [reasons, setReasons] = useState<Record<string, string>>({});
  const [reasonErrors, setReasonErrors] = useState<Record<string, boolean>>({});
  // Rule-added offerings the reviewer unticked, per request id (#2370).
  const [excludedByRow, setExcludedByRow] = useState<
    Record<string, readonly string[]>
  >({});
  const [notice, setNotice] = useState<string | null>(null);
  // Set while THIS list dispatches change-requests-refresh so its own listener
  // doesn't refetch the row it already removed optimistically.
  const suppressSelfReloadRef = useRef(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setRows(await listOfferingChangeRequests());
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.warn("offering_request_review_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const reloadInPlace = useCallback(async () => {
    try {
      setRows(await listOfferingChangeRequests());
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.warn("offering_request_review_reload_failed", { error: message });
    }
  }, []);

  // Same refresh contract as the sibling queues: decisions in this window
  // dispatch change-requests-refresh, decisions elsewhere arrive as the
  // SSE-derived messages-unread-refresh. Swap rows in place either way so an
  // open page never offers a decision on an already-decided request.
  useEffect(() => {
    const handler = () => {
      if (suppressSelfReloadRef.current) return;
      void reloadInPlace();
    };
    window.addEventListener("change-requests-refresh", handler);
    window.addEventListener("messages-unread-refresh", handler);
    return () => {
      window.removeEventListener("change-requests-refresh", handler);
      window.removeEventListener("messages-unread-refresh", handler);
    };
  }, [reloadInPlace]);

  const decide = useCallback(
    async (row: StaffOfferingRequest, approve: boolean) => {
      const reason = reasons[row.id]?.trim() ?? "";
      if (!approve && reason === "") {
        setReasonErrors((prev) => ({ ...prev, [row.id]: true }));
        return;
      }
      setBusyId(row.id);
      setError(null);
      setNotice(null);
      const excluded = approve ? (excludedByRow[row.id] ?? []) : [];
      try {
        if (excluded.length > 0) {
          await decideOfferingChangeRequest(
            row.id,
            approve,
            reason || undefined,
            excluded,
          );
        } else {
          await decideOfferingChangeRequest(
            row.id,
            approve,
            reason || undefined,
          );
        }
        setRows((prev) => prev.filter((r) => r.id !== row.id));
        suppressSelfReloadRef.current = true;
        window.dispatchEvent(new Event("change-requests-refresh"));
        suppressSelfReloadRef.current = false;
        setNotice(
          approve
            ? `Änderung übernommen, gültig ab ${formatDate(row.effective_from)}`
            : "Angebots-Anfrage abgelehnt",
        );
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        const code =
          err instanceof OfferingRequestApiError ? err.code : undefined;
        logger.warn("offering_request_review_decide_failed", {
          error: message,
          request_id: row.id,
          ...(code ? { code } : {}),
        });
        setError(decideErrorMessage(code));
      } finally {
        setBusyId(null);
      }
    },
    [reasons, excludedByRow],
  );

  const toggleExcluded = useCallback((rowId: string, offeringId: string) => {
    setExcludedByRow((prev) => {
      const current = prev[rowId] ?? [];
      const next = current.includes(offeringId)
        ? current.filter((id) => id !== offeringId)
        : [...current, offeringId];
      return { ...prev, [rowId]: next };
    });
  }, []);

  if (loading) {
    return (
      <SkeletonRegion label="Angebots-Anfragen werden geladen">
        <ListSkeleton rows={3} avatar={false} />
      </SkeletonRegion>
    );
  }

  return (
    <div className="space-y-3">
      {error && (
        <div className="rounded-xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-3 text-sm text-[#CC2626]">
          {error}
        </div>
      )}
      {notice && (
        <div className="rounded-xl border border-gray-200 bg-gray-50 p-3 text-sm text-gray-700">
          {notice}
        </div>
      )}
      {rows.length === 0 ? (
        <p className="rounded-2xl border border-gray-200 bg-white p-4 text-sm text-gray-500 shadow-sm">
          Keine offenen Anfragen zu Betreuungsangeboten.
        </p>
      ) : (
        rows.map((row) => (
          <RequestReviewCard
            key={row.id}
            childName={row.student_name}
            summary={`ab ${formatDate(row.effective_from)}`}
            reason={reasons[row.id] ?? ""}
            onReasonChange={(value) => {
              setReasons((prev) => ({ ...prev, [row.id]: value }));
              setReasonErrors((prev) => ({ ...prev, [row.id]: false }));
            }}
            reasonPlaceholder="Begründung (Pflicht bei Ablehnung)"
            reasonError={
              reasonErrors[row.id]
                ? "Für eine Ablehnung ist eine Begründung erforderlich."
                : undefined
            }
            busy={busyId === row.id}
            onApprove={() => void decide(row, true)}
            onReject={() => void decide(row, false)}
          >
            <ReviewDiffPanel>
              {row.diff.length === 0 && (
                <span className="text-sm text-gray-500">—</span>
              )}
              {(() => {
                const excluded = excludedByRow[row.id] ?? [];
                const removed = removedOfferings(row.diff, excluded);
                return row.diff.map((entry) => {
                  const isExcluded = excluded.includes(entry.offering_id);
                  const isRemoved = removed.has(entry.offering_id);
                  const cascaded = isRemoved && !isExcluded;
                  return (
                    <div
                      key={entry.offering_id}
                      className={`text-sm ${isRemoved ? "opacity-50" : ""}`}
                    >
                      <span className="text-xs text-gray-500">
                        {entry.label}:{" "}
                      </span>
                      {entry.automatic && (
                        <StatusBadge
                          tone="blue"
                          label="Automatisch mitgebucht"
                        />
                      )}
                      <div className="flex flex-wrap items-baseline gap-2">
                        <span className="text-gray-400 line-through">
                          {entry.old}
                        </span>
                        <span className="text-gray-400" aria-hidden="true">
                          →
                        </span>
                        <span
                          className={
                            isRemoved
                              ? "font-medium text-gray-500 line-through"
                              : "font-medium text-gray-900"
                          }
                        >
                          {entry.new}
                        </span>
                      </div>
                      {entry.automatic && !cascaded && (
                        <p className="mt-0.5 text-xs text-gray-500">
                          {automaticHint(entry)}
                        </p>
                      )}
                      {cascaded && (
                        <p className="mt-0.5 text-xs text-gray-500">
                          Entfällt, weil{" "}
                          {`„${cascadeSourceName(entry, row.diff, removed)}“`}{" "}
                          nicht mitgebucht wird.
                        </p>
                      )}
                      {entry.optoutable && !cascaded && (
                        <label
                          htmlFor={`mitbuchen-${row.id}-${entry.offering_id}`}
                          className="mt-1 flex w-fit cursor-pointer items-center gap-2 text-xs text-gray-700"
                        >
                          <Checkbox
                            id={`mitbuchen-${row.id}-${entry.offering_id}`}
                            checked={!isExcluded}
                            onChange={() =>
                              toggleExcluded(row.id, entry.offering_id)
                            }
                            disabled={busyId === row.id}
                            aria-label={`${entry.label} automatisch mitbuchen`}
                          />
                          Automatisch mitbuchen
                        </label>
                      )}
                      {isExcluded && (
                        <p className="mt-0.5 text-xs text-gray-500">
                          Wird bei der Freigabe nicht gebucht.
                        </p>
                      )}
                    </div>
                  );
                });
              })()}
              {row.note && (
                <p className="mt-2 text-xs text-gray-500">
                  Nachricht der Eltern: {row.note}
                </p>
              )}
            </ReviewDiffPanel>
          </RequestReviewCard>
        ))
      )}
    </div>
  );
}
