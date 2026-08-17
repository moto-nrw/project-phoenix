"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import {
  RequestReviewCard,
  ReviewDiffPanel,
} from "~/components/students/request-review-card";
import { CONTACT_METHODS, getLanguageLabel } from "~/lib/guardian-helpers";
import { createLogger } from "~/lib/logger";
import {
  type StaffMasterDataChange,
  decideMasterDataChangeRequest,
  listMasterDataChangeRequests,
} from "~/lib/master-data-review-api";
import {
  DEPARTURE_MODE_LABELS,
  DEPARTURE_WEEKDAYS,
  type DepartureMode,
} from "~/lib/student-helpers";

const logger = createLogger({ component: "MasterDataReviewList" });

// German-only staff UI: the staff shell ships a minimal client message catalog
// (parentNav only — see shell-nav-intl-provider.tsx), so this page hardcodes its
// German strings like the rest of the staff/admin surface instead of using
// useTranslations, which would resolve to raw keys here.
const EMPTY_VALUE = "—";

const FIELD_LABELS: Record<string, string> = {
  first_name: "Vorname",
  last_name: "Nachname",
  birthday: "Geburtsdatum",
  health_info: "Gesundheitshinweise",
  email: "E-Mail",
  primary: "Telefonnummer",
  address_street: "Straße",
  address_city: "Ort",
  address_postal_code: "PLZ",
  preferred_contact_method: "Kontaktweg",
  language_preference: "Sprache",
  allowed_departure_modes: "Dauerhafte Gehzeiten",
};

function fieldLabel(field: string): string {
  // Known field keys have German labels; fall back to the raw key.
  return FIELD_LABELS[field] ?? field;
}

// German labels for the wire values (#2362). Unknown keys fall back to the raw
// value so future wire additions stay visible instead of crashing or vanishing.
const WEEKDAY_LABELS: Record<string, string> = Object.fromEntries(
  DEPARTURE_WEEKDAYS.map((day) => [day.key, day.label]),
);

const CONTACT_METHOD_LABELS: Record<string, string> = Object.fromEntries(
  CONTACT_METHODS.map((method) => [method.value, method.label]),
);

function departureModeLabel(mode: unknown): string {
  return DEPARTURE_MODE_LABELS[mode as DepartureMode] ?? String(mode);
}

function formatValue(field: string, value: unknown, empty: string): string {
  if (value === null || value === undefined || value === "") return empty;
  if (typeof value === "string") {
    if (field === "preferred_contact_method")
      return CONTACT_METHOD_LABELS[value] ?? value;
    if (field === "language_preference") return getLanguageLabel(value);
    return value;
  }
  if (typeof value === "object") {
    // Departure modes: { mon: ["pickup"], ... } — render a compact summary
    // with German weekday and mode labels ("Montag: Wird abgeholt").
    return Object.entries(value as Record<string, unknown>)
      .map(([day, modes]) => {
        const dayLabel = WEEKDAY_LABELS[day] ?? day;
        return Array.isArray(modes)
          ? `${dayLabel}: ${modes.map(departureModeLabel).join(" / ")}`
          : dayLabel;
      })
      .join(", ");
  }
  return String(value);
}

export function MasterDataReviewList() {
  const [rows, setRows] = useState<StaffMasterDataChange[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [reasons, setReasons] = useState<Record<string, string>>({});
  const [notice, setNotice] = useState<string | null>(null);
  // Set while THIS list dispatches change-requests-refresh so its own listener
  // (below) doesn't refetch — it already removed the decided row optimistically.
  // dispatchEvent is synchronous, so the flag only has to cover that one call.
  const suppressSelfReloadRef = useRef(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setRows(await listMasterDataChangeRequests());
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.warn("master_data_review_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  // Refetch without the full-page spinner. An `allowed_departure_modes`
  // decision here changes the departure modes the sibling Betreuungszeiten
  // queue computes its diffs against (and a care decision changes ours), so a
  // sibling decision can leave these "current → requested" diffs stale. Both
  // queues emit change-requests-refresh after a decision; listen for it and
  // swap rows in place (never blank the section) so reviewers act on fresh
  // values.
  const reloadInPlace = useCallback(async () => {
    try {
      setRows(await listMasterDataChangeRequests());
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.warn("master_data_review_reload_failed", { error: message });
    }
  }, []);

  // change-requests-refresh covers decisions made in THIS window (both queues
  // dispatch it). A decision made elsewhere (another tab, another staffer)
  // never fires it here; it only reaches this window as the parent-message pill
  // the backend emits, which use-global-sse fans out as messages-unread-refresh
  // (the same SSE-derived path the sidebar badge listens to). Without this an
  // already-open review page keeps showing pending rows the backend has already
  // decided and would let the reviewer submit a now-conflicting decision until
  // a manual reload. Listen to both and swap rows in place.
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
    async (row: StaffMasterDataChange, approve: boolean) => {
      setBusyId(row.id);
      setError(null);
      setNotice(null);
      try {
        await decideMasterDataChangeRequest(
          row.id,
          approve,
          reasons[row.id]?.trim() || undefined,
        );
        setRows((prev) => prev.filter((r) => r.id !== row.id));
        // Clear/decrement the Änderungsanfragen sidebar badge without waiting
        // for the 60s cache or an SSE round-trip, and prompt the sibling
        // Betreuungszeiten queue to refetch its (possibly now-stale) diffs.
        // Suppress our own listener: the decided row is already removed above.
        suppressSelfReloadRef.current = true;
        window.dispatchEvent(new Event("change-requests-refresh"));
        suppressSelfReloadRef.current = false;
        setNotice(approve ? "Änderung übernommen" : "Änderung abgelehnt");
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        logger.warn("master_data_review_decide_failed", {
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

  if (loading) {
    return (
      <SkeletonRegion label="Änderungsanfragen werden geladen">
        <ListSkeleton rows={3} avatar={false} />
      </SkeletonRegion>
    );
  }

  return (
    <div className="space-y-3">
      {error && (
        <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-xl border p-3 text-sm">
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
          Keine offenen Änderungsanfragen.
        </p>
      ) : (
        rows.map((row) => (
          <RequestReviewCard
            key={row.id}
            childName={`${row.first_name} ${row.last_name}`}
            summary={fieldLabel(row.field_key)}
            reason={reasons[row.id] ?? ""}
            onReasonChange={(value) =>
              setReasons((prev) => ({ ...prev, [row.id]: value }))
            }
            reasonPlaceholder="Begründung (optional)"
            busy={busyId === row.id}
            onApprove={() => void decide(row, true)}
            onReject={() => void decide(row, false)}
          >
            <ReviewDiffPanel>
              {/* Field name lives in the collapsed summary; the expanded panel
                  shows only the value change. */}
              <div className="flex flex-wrap items-baseline gap-2 text-sm">
                <span className="text-gray-400 line-through">
                  {formatValue(row.field_key, row.old_value, EMPTY_VALUE)}
                </span>
                <span className="text-gray-400" aria-hidden="true">
                  →
                </span>
                <span className="font-medium text-gray-900">
                  {formatValue(row.field_key, row.new_value, EMPTY_VALUE)}
                </span>
              </div>
            </ReviewDiffPanel>
          </RequestReviewCard>
        ))
      )}
    </div>
  );
}
