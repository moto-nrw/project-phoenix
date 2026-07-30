"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { Loading } from "~/components/ui/loading";
import {
  RequestReviewCard,
  ReviewDiffPanel,
} from "~/components/students/request-review-card";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  OfferingRequestApiError,
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

// German-only staff UI — hardcoded strings like the sibling review lists (the
// staff shell ships no full message catalog).
export function OfferingRequestReviewList() {
  const [rows, setRows] = useState<StaffOfferingRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [reasons, setReasons] = useState<Record<string, string>>({});
  const [reasonErrors, setReasonErrors] = useState<Record<string, boolean>>({});
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
      try {
        await decideOfferingChangeRequest(row.id, approve, reason || undefined);
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
    [reasons],
  );

  if (loading) return <Loading fullPage={false} />;

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
              {row.diff.map((entry) => (
                <div key={entry.label} className="text-sm">
                  <span className="text-xs text-gray-500">{entry.label}: </span>
                  <div className="flex flex-wrap items-baseline gap-2">
                    <span className="text-gray-400 line-through">
                      {entry.old}
                    </span>
                    <span className="text-gray-400" aria-hidden="true">
                      →
                    </span>
                    <span className="font-medium text-gray-900">
                      {entry.new}
                    </span>
                  </div>
                </div>
              ))}
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
