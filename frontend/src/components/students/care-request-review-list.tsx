"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { Loading } from "~/components/ui/loading";
import {
  RequestReviewCard,
  ReviewDiffPanel,
} from "~/components/students/request-review-card";
import { createLogger } from "~/lib/logger";
import {
  type StaffCareRequest,
  decideCareScheduleChangeRequest,
  listCareScheduleChangeRequests,
} from "~/lib/care-request-review-api";

const logger = createLogger({ component: "CareRequestReviewList" });

// One-line summary for the collapsed card: the distinct change kinds (e.g.
// "Abholzeit + Abholart"), taken from the part after "·" in each diff label.
function careSummary(diff: StaffCareRequest["diff"]): string {
  const kinds = [
    ...new Set(
      diff.map((entry) => {
        const parts = entry.label.split("·");
        return (parts.at(-1) ?? entry.label).trim();
      }),
    ),
  ];
  return kinds.join(" + ") || "Änderung";
}

// German-only staff UI — hardcoded strings like MasterDataReviewList (the
// staff shell ships no full message catalog).
export function CareRequestReviewList() {
  const [rows, setRows] = useState<StaffCareRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [reasons, setReasons] = useState<Record<string, string>>({});
  const [reasonErrors, setReasonErrors] = useState<Record<string, boolean>>({});
  const [notice, setNotice] = useState<string | null>(null);
  // Set while THIS list dispatches change-requests-refresh so its own listener
  // (below) doesn't refetch — it already removed the decided row optimistically.
  // dispatchEvent is synchronous, so the flag only has to cover that one call.
  const suppressSelfReloadRef = useRef(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setRows(await listCareScheduleChangeRequests());
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.warn("care_request_review_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  // Refetch without the full-page spinner. A master-data
  // `allowed_departure_modes` decision on the same child changes the very
  // departure modes this queue's "current → requested" diffs are computed
  // against, so a sibling decision can leave these diffs stale. Both queues
  // emit change-requests-refresh after a decision; listen for it and swap rows
  // in place (never blank the section) so reviewers act on fresh values.
  const reloadInPlace = useCallback(async () => {
    try {
      setRows(await listCareScheduleChangeRequests());
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.warn("care_request_review_reload_failed", { error: message });
    }
  }, []);

  useEffect(() => {
    const handler = () => {
      if (suppressSelfReloadRef.current) return;
      void reloadInPlace();
    };
    window.addEventListener("change-requests-refresh", handler);
    return () => window.removeEventListener("change-requests-refresh", handler);
  }, [reloadInPlace]);

  const decide = useCallback(
    async (row: StaffCareRequest, approve: boolean) => {
      const reason = reasons[row.id]?.trim() ?? "";
      if (!approve && reason === "") {
        // The backend requires a reason for rejections; surface that before
        // the request instead of a generic error after it.
        setReasonErrors((prev) => ({ ...prev, [row.id]: true }));
        return;
      }
      setBusyId(row.id);
      setError(null);
      setNotice(null);
      try {
        await decideCareScheduleChangeRequest(
          row.id,
          approve,
          reason || undefined,
        );
        setRows((prev) => prev.filter((r) => r.id !== row.id));
        // Clear/decrement the Änderungsanfragen sidebar badge without waiting
        // for the 60s cache or an SSE round-trip, and prompt the sibling
        // Stammdaten queue to refetch its (possibly now-stale) diffs. Suppress
        // our own listener: the decided row is already removed above.
        suppressSelfReloadRef.current = true;
        window.dispatchEvent(new Event("change-requests-refresh"));
        suppressSelfReloadRef.current = false;
        setNotice(
          approve
            ? "Betreuungszeiten übernommen"
            : "Betreuungszeit-Anfrage abgelehnt",
        );
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        logger.warn("care_request_review_decide_failed", {
          error: message,
          request_id: row.id,
        });
        setError("Die Entscheidung konnte nicht gespeichert werden.");
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
          Keine offenen Betreuungszeit-Anfragen.
        </p>
      ) : (
        rows.map((row) => (
          <RequestReviewCard
            key={row.id}
            childName={`${row.first_name} ${row.last_name}`}
            summary={careSummary(row.diff)}
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
            </ReviewDiffPanel>
          </RequestReviewCard>
        ))
      )}
    </div>
  );
}
