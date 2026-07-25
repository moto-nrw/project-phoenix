"use client";

import { useEffect, useState } from "react";
import { Mail, MailWarning } from "lucide-react";
import { Modal } from "~/components/ui/modal";
import { Alert } from "~/components/ui/alert";
import { Loading } from "~/components/ui/loading";
import { StatusDotBadge } from "~/components/ui/status-dot-badge";
import {
  DataField,
  DataGrid,
  InfoSection,
} from "~/components/ui/detail-modal-components";
import { fetchInvitationDelivery } from "~/lib/guardian-api";
import type {
  InvitationDelivery,
  InvitationDeliveryAttempt,
} from "~/lib/guardian-delivery-helpers";
import { presentDeliveryAttempt } from "~/lib/guardian-delivery-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "InvitationDeliveryModal" });

interface InvitationDeliveryModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly invitationId: string;
  readonly recipientEmail?: string;
  readonly guardianName: string;
}

/**
 * Detail view for what actually happened to a guardian's invitation emails.
 *
 * Shows one card per dispatch attempt (a resend adds another), each with the
 * plain-German state, the school's next action, and the mail server's own
 * reports. Technical detail stays subordinate: staff need to know whether to
 * wait, resend, or fix the address.
 */
export function InvitationDeliveryModal({
  isOpen,
  onClose,
  invitationId,
  recipientEmail,
  guardianName,
}: InvitationDeliveryModalProps) {
  const [delivery, setDelivery] = useState<InvitationDelivery | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isOpen) return;

    let cancelled = false;
    setLoading(true);
    setError(null);

    fetchInvitationDelivery(invitationId)
      .then((result) => {
        if (!cancelled) setDelivery(result);
      })
      .catch((err: unknown) => {
        logger.error("invitation_delivery_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!cancelled) setError("Zustellstatus konnte nicht geladen werden.");
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [isOpen, invitationId]);

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Zustellstatus der Einladung"
      widthClass="mx-4 w-[calc(100%-2rem)] max-w-2xl"
    >
      <div className="space-y-4">
        <DataGrid>
          <DataField label="Erziehungsberechtigte Person">
            {guardianName}
          </DataField>
          <DataField label="E-Mail-Adresse">
            {recipientEmail ?? "Nicht angegeben"}
          </DataField>
        </DataGrid>

        {loading && <Loading />}

        {error && <Alert type="error" message={error} />}

        {!loading && !error && delivery?.attempts.length === 0 && (
          <Alert
            type="info"
            message="Für diese Einladung liegt noch kein Versandvorgang vor."
          />
        )}

        {!loading &&
          !error &&
          delivery?.attempts.map((attempt, index) => (
            <AttemptCard
              key={attempt.outboxId}
              attempt={attempt}
              // The list is newest first, so index 0 is the current attempt.
              ordinal={delivery.attempts.length - index}
              isLatest={index === 0}
            />
          ))}
      </div>
    </Modal>
  );
}

function AttemptCard({
  attempt,
  ordinal,
  isLatest,
}: Readonly<{
  attempt: InvitationDeliveryAttempt;
  ordinal: number;
  isLatest: boolean;
}>) {
  const presentation = presentDeliveryAttempt(attempt);

  return (
    <InfoSection
      title={isLatest ? `Versand ${ordinal} (aktuell)` : `Versand ${ordinal}`}
      icon={
        presentation.needsAddressFix ? (
          <MailWarning className="h-full w-full" />
        ) : (
          <Mail className="h-full w-full" />
        )
      }
      accentColor="gray"
    >
      <div className="space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <StatusDotBadge
            label={presentation.label}
            color={presentation.color}
          />
          {attempt.attempts > 1 && (
            <span className="text-xs text-gray-500">
              {attempt.attempts} Versuche
            </span>
          )}
        </div>

        <p className="text-sm text-gray-700">{presentation.hint}</p>

        <DataGrid>
          <DataField label="Eingeplant">
            {formatTimestamp(attempt.queuedAt)}
          </DataField>
          {attempt.sentAt && (
            <DataField label="An Mailserver übergeben">
              {formatTimestamp(attempt.sentAt)}
            </DataField>
          )}
          {attempt.deliveryStatusAt && (
            <DataField label="Letzte Rückmeldung">
              {formatTimestamp(attempt.deliveryStatusAt)}
            </DataField>
          )}
          {attempt.nextRetryAt && (
            <DataField label="Nächster Versuch">
              {formatTimestamp(attempt.nextRetryAt)}
            </DataField>
          )}
        </DataGrid>

        {attempt.events.length > 0 && (
          <div>
            <p className="mb-1 text-xs text-gray-500">
              Rückmeldungen des Mailservers
            </p>
            <ul className="space-y-1">
              {attempt.events.map((event) => (
                <li
                  key={`${event.type}-${event.occurredAt}`}
                  className="text-xs text-gray-600"
                >
                  <span className="font-medium">
                    {formatTimestamp(event.occurredAt)}
                  </span>
                  {" · "}
                  {event.reasonCode ? `${event.reasonCode} ` : ""}
                  {event.reason ?? event.type}
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </InfoSection>
  );
}

/** Timestamps here are instants, so a plain German date+time is correct. */
function formatTimestamp(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Unbekannt";
  return `${date.toLocaleDateString("de-DE")} ${date.toLocaleTimeString(
    "de-DE",
    {
      hour: "2-digit",
      minute: "2-digit",
    },
  )} Uhr`;
}
