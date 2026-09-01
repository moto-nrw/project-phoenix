"use client";

/**
 * Eine einzelne Anfrage im Detailbereich eines Kindes (#2267). Der Rahmen um
 * die Entscheiden-Karte der jeweiligen Art: Auswahl für die Sammelfreigabe,
 * der sichtbare Grund, warum eine Anfrage nur einzeln geht, die Warnung bei
 * einem inzwischen geänderten Wert, und der Abschluss für Anfragen, die nur
 * noch vergangene Tage betreffen.
 */

import { useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { CareRequestReviewItem } from "~/components/students/care-request-review-item";
import { EnrollmentRequestItem } from "~/components/students/enrollment-request-item";
import { ExcusedRequestReviewItem } from "~/components/students/excused-request-review-item";
import { MasterDataReviewItem } from "~/components/students/master-data-review-item";
import { OfferingRequestReviewItem } from "~/components/students/offering-request-review-item";
import { createLogger } from "~/lib/logger";
import {
  ChangeRequestStaleError,
  markRequestDone,
  type AggregatedOpenRequest,
  type ParentRequestKind,
} from "~/lib/change-request-list-api";
import { hasReviewMetadata, itemKey } from "./case-model";
import {
  bulkIneligibleText,
  DECISION_BLOCKED_BY_CONFLICT,
  CURRENT_VALUE_CHANGED_WARNING,
  PAST_REQUEST_HINT,
  STALE_REQUEST_NOTICE,
} from "./request-copy";

const logger = createLogger({ component: "RequestDecisionRow" });

function OpenRequestContent({
  request,
  onDecided,
  decisionDisabledReason,
  approveReasonRequired,
}: Readonly<{
  request: AggregatedOpenRequest;
  onDecided: (notice: string) => void;
  decisionDisabledReason?: string;
  approveReasonRequired: boolean;
}>) {
  switch (request.request_type) {
    case "enrollment":
      return <EnrollmentRequestItem row={request.data} view="open" grouped />;
    case "master_data":
      return (
        <MasterDataReviewItem
          row={request.data}
          onDecided={onDecided}
          grouped
          expectedVersion={request.expected_version}
          decisionDisabledReason={decisionDisabledReason}
          approveReasonRequired={approveReasonRequired}
        />
      );
    case "care_schedule":
      return (
        <CareRequestReviewItem
          row={request.data}
          onDecided={onDecided}
          grouped
          expectedVersion={request.expected_version}
          decisionDisabledReason={decisionDisabledReason}
          approveReasonRequired={approveReasonRequired}
        />
      );
    case "offering":
      return (
        <OfferingRequestReviewItem
          row={request.data}
          onDecided={onDecided}
          grouped
          expectedVersion={request.expected_version}
          decisionDisabledReason={decisionDisabledReason}
          approveReasonRequired={approveReasonRequired}
        />
      );
    case "excused":
      return (
        <ExcusedRequestReviewItem
          row={request.data}
          onDecided={onDecided}
          grouped
          expectedVersion={request.expected_version}
          currentStatusByDate={request.current_status_by_date}
          decisionDisabledReason={decisionDisabledReason}
          approveReasonRequired={approveReasonRequired}
        />
      );
  }
}

export function RequestDecisionRow({
  request,
  selected,
  position,
  total,
  inConflict,
  approveReasonRequired = false,
  onSelectionChange,
  onDecided,
  onStale,
}: Readonly<{
  request: AggregatedOpenRequest;
  selected: boolean;
  position: number;
  total: number;
  /** Steckt diese Anfrage in einer noch offenen Widerspruchsgruppe? */
  inConflict: boolean;
  /** Verlangt die Schule beim Freigeben eine Begründung? */
  approveReasonRequired?: boolean;
  onSelectionChange: (key: string, checked: boolean) => void;
  onDecided: (key: string, notice: string) => void;
  onStale: () => void;
}>) {
  const key = itemKey(request);
  const [markingDone, setMarkingDone] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!hasReviewMetadata(request)) {
    return (
      <OpenRequestContent
        request={request}
        onDecided={(notice) => onDecided(key, notice)}
        approveReasonRequired={approveReasonRequired}
      />
    );
  }

  const past = request.past === true;
  // Bei einer abgelaufenen Anfrage bleiben Ablehnen und Freigeben stehen: das
  // Backend weist ein Freigeben mit klarem Grund ab, und Ablehnen ist weiter
  // erlaubt. Der Hinweis dazu steht einmal oben, nicht zweimal.
  const disabledReason = inConflict ? DECISION_BLOCKED_BY_CONFLICT : undefined;

  const markDone = async () => {
    setMarkingDone(true);
    setError(null);
    try {
      await markRequestDone(
        request.request_type as ParentRequestKind,
        String(request.data.id),
        request.expected_version,
      );
      onDecided(key, "Die Anfrage wurde abgeschlossen.");
    } catch (err) {
      logger.warn("parent_request_mark_done_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      if (err instanceof ChangeRequestStaleError) {
        setError(STALE_REQUEST_NOTICE);
        onStale();
      } else {
        setError(
          err instanceof Error
            ? err.message
            : "Die Anfrage konnte nicht abgeschlossen werden.",
        );
      }
      setMarkingDone(false);
    }
  };

  return (
    <div className="border-t border-gray-100 first:border-t-0">
      <div className="flex flex-wrap items-center justify-between gap-2 px-4 pt-3 sm:px-5">
        <p className="text-xs font-medium text-gray-500">
          Anfrage {position} von {total}
        </p>
        {request.bulk_eligible ? (
          <label
            htmlFor={`bulk-request-${key}`}
            className="flex min-h-11 cursor-pointer items-center gap-2 text-xs font-medium text-gray-600 sm:min-h-8"
          >
            <Checkbox
              id={`bulk-request-${key}`}
              aria-label={`Gemeinsam freigeben: ${request.student_name}`}
              checked={selected}
              onChange={(event) => onSelectionChange(key, event.target.checked)}
            />
            <span>Gemeinsam freigeben</span>
          </label>
        ) : request.past ? null : (
          <p className="text-xs text-gray-600">
            Nur einzeln entscheiden:{" "}
            {bulkIneligibleText(
              request.bulk_ineligible_reason,
              request.bulk_ineligible_text,
            )}
          </p>
        )}
      </div>
      {request.current_value_changed === true && (
        <div className="px-4 pt-2 sm:px-5">
          <Alert type="warning" message={CURRENT_VALUE_CHANGED_WARNING} />
        </div>
      )}
      {past && (
        <div className="space-y-2 px-4 pt-2 sm:px-5">
          <p className="text-sm text-gray-600">{PAST_REQUEST_HINT}</p>
          <Button
            type="button"
            variant="outline"
            size="md"
            className="max-sm:min-h-11"
            disabled={markingDone}
            onClick={() => void markDone()}
          >
            Als erledigt markieren
          </Button>
        </div>
      )}
      {error && (
        <div className="px-4 pt-2 sm:px-5">
          <Alert type="warning" message={error} />
        </div>
      )}
      <div className="min-w-0">
        <OpenRequestContent
          request={request}
          onDecided={(notice) => onDecided(key, notice)}
          decisionDisabledReason={disabledReason}
          approveReasonRequired={approveReasonRequired}
        />
      </div>
    </div>
  );
}
