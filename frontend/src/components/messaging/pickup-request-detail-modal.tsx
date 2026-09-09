"use client";

/**
 * Lese-Ansicht einer Tages-Abholzeit-Anfrage, geöffnet aus dem Nachrichten-
 * Verlauf (#3135). Zeigt, was die Eltern für welchen Tag beantragt haben und
 * wie die OGS entschieden hat; auch nach Freigabe, Ablehnung oder Rücknahme.
 * Hier wird nichts entschieden: eine offene Anfrage verweist auf `Anfragen`.
 */

import { useEffect, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import {
  DataField,
  DataFieldSkeleton,
  DataGrid,
  DetailIcons,
  InfoSection,
  InfoText,
} from "~/components/ui/detail-modal-components";
import { Modal } from "~/components/ui/modal";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import { HISTORY_STATUS_META } from "~/components/students/request-review-card";
import {
  CareRequestApiError,
  fetchCareScheduleChangeRequest,
  type StaffCareRequestDetail,
} from "~/lib/care-request-review-api";
import { formatChatDateTime, formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "PickupRequestDetailModal" });

// Die Historie kennt nur entschiedene Anfragen; hier kommt der offene Status
// dazu, mit demselben Wort wie in der Arbeitsliste.
const PENDING_META: { label: string; tone: StatusBadgeTone } = {
  label: "Offen",
  tone: "blue",
};

type LoadState =
  | { readonly kind: "loading" }
  | { readonly kind: "loaded"; readonly detail: StaffCareRequestDetail }
  | { readonly kind: "gone" }
  | { readonly kind: "forbidden" }
  | { readonly kind: "failed" };

function clockLabel(time: string): string {
  return `${time} Uhr`;
}

export function PickupRequestDetailModal({
  requestId,
  onClose,
  onOpenQueue,
}: Readonly<{
  /** Zeilen-ID der Anfrage; `null` hält das Fenster geschlossen. */
  requestId: string | null;
  onClose: () => void;
  /**
   * Weg zur Seite, auf der entschieden wird. Nur für eine noch offene
   * Anfrage angeboten; wer nicht entscheiden darf, übergibt nichts.
   */
  onOpenQueue?: () => void;
}>) {
  const [state, setState] = useState<LoadState>({ kind: "loading" });

  useEffect(() => {
    if (!requestId) return;
    let cancelled = false;
    setState({ kind: "loading" });
    fetchCareScheduleChangeRequest(requestId)
      .then((detail) => {
        if (!cancelled) setState({ kind: "loaded", detail });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        const status =
          err instanceof CareRequestApiError ? err.status : undefined;
        if (status === 404) {
          setState({ kind: "gone" });
          return;
        }
        if (status === 403) {
          setState({ kind: "forbidden" });
          return;
        }
        logger.error("pickup_request_detail_load_failed", {
          error: err instanceof Error ? err.message : String(err),
          request_id: requestId,
        });
        setState({ kind: "failed" });
      });
    return () => {
      cancelled = true;
    };
  }, [requestId]);

  const pending = state.kind === "loaded" && state.detail.status === "pending";

  return (
    <Modal
      isOpen={requestId !== null}
      onClose={onClose}
      title="Anfrage: Abholzeit"
      mobileSheet
      footer={
        <div className="flex flex-wrap justify-end gap-2">
          <Button type="button" variant="secondary" size="md" onClick={onClose}>
            Schließen
          </Button>
          {pending && onOpenQueue && (
            <Button
              type="button"
              variant="primary"
              size="md"
              onClick={onOpenQueue}
            >
              Zu den Anfragen
            </Button>
          )}
        </div>
      }
    >
      {state.kind === "loading" && (
        <InfoSection title="Beantragt" icon={DetailIcons.document}>
          <DataGrid>
            <DataFieldSkeleton />
            <DataFieldSkeleton />
            <DataFieldSkeleton />
            <DataFieldSkeleton fullWidth />
          </DataGrid>
        </InfoSection>
      )}
      {state.kind === "gone" && (
        <Alert
          type="info"
          message="Diese Anfrage gibt es nicht mehr. Der Hinweis im Verlauf bleibt erhalten."
        />
      )}
      {state.kind === "forbidden" && (
        <Alert
          type="info"
          message="Für dieses Kind dürfen Sie keine Anfragen ansehen."
        />
      )}
      {state.kind === "failed" && (
        <Alert
          type="error"
          message="Die Anfrage konnte nicht geladen werden. Bitte versuchen Sie es noch einmal."
        />
      )}
      {state.kind === "loaded" && <DetailBody detail={state.detail} />}
    </Modal>
  );
}

function DetailBody({ detail }: Readonly<{ detail: StaffCareRequestDetail }>) {
  const meta = HISTORY_STATUS_META[detail.status] ?? PENDING_META;
  // Nur Abholzeit-Hinweise öffnen dieses Fenster; ein Antrag ohne lesbaren
  // Tag zeigt „Keine Angabe" statt eines erfundenen Werts.
  const pickup = detail.pickup_change;
  return (
    <div className="space-y-3">
      <InfoSection title="Beantragt" icon={DetailIcons.document}>
        <DataGrid>
          <DataField label="Kind">
            {detail.first_name} {detail.last_name}
          </DataField>
          <DataField label="Tag">
            {pickup ? formatDate(pickup.date) : "Keine Angabe"}
          </DataField>
          <DataField label="Gewünschte Abholzeit">
            {pickup ? clockLabel(pickup.pickup_time) : "Keine Angabe"}
          </DataField>
          <DataField label="Abholzeit vorher">
            {pickup?.previous_pickup_time
              ? clockLabel(pickup.previous_pickup_time)
              : "Keine Angabe"}
          </DataField>
          <DataField label="Grund der Eltern" fullWidth>
            {detail.request_reason ?? "Keine Angabe"}
          </DataField>
          <DataField label="Eingegangen">
            {formatChatDateTime(detail.created_at)}
          </DataField>
        </DataGrid>
      </InfoSection>

      <InfoSection title="Entscheidung" icon={DetailIcons.notes}>
        <div className="mb-2">
          <StatusBadge label={meta.label} tone={meta.tone} />
        </div>
        {detail.status === "pending" && (
          <InfoText>
            Noch nicht entschieden. Entschieden wird unter Anfragen.
          </InfoText>
        )}
        {detail.status === "withdrawn" && (
          <InfoText>
            Die Eltern haben die Anfrage zurückgezogen
            {detail.decided_at
              ? ` (${formatChatDateTime(detail.decided_at)})`
              : ""}
            .
          </InfoText>
        )}
        {(detail.status === "approved" || detail.status === "rejected") && (
          <DataGrid>
            <DataField label="Entschieden am">
              {detail.decided_at
                ? formatChatDateTime(detail.decided_at)
                : "Keine Angabe"}
            </DataField>
            <DataField label="Entschieden von">
              {detail.decided_by_name ?? "Keine Angabe"}
            </DataField>
            <DataField label="Begründung der OGS" fullWidth>
              {detail.decision_reason ?? "Keine Angabe"}
            </DataField>
          </DataGrid>
        )}
      </InfoSection>
    </div>
  );
}
