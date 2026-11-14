"use client";

import { type ReactNode, useCallback, useEffect, useState } from "react";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiLoadError } from "~/contexts/ToastContext";
import {
  CHECK_ADDRESS_AT_GUARDIAN,
  GUARDIAN_STOPPED_COPY,
  mailStatusCopy,
} from "~/lib/email-delivery-status";
import {
  type AdminRequestMail,
  getAdminRequestMails,
} from "~/lib/enrollment-admin-api";
import { formatBerlinDate, formatTime } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "RequestMailStatus" });

const REQUEST_MAIL_LABELS: Record<string, string> = {
  enrollment_submitted: "Eingangsbestätigung",
  enrollment_approved: "Zusage",
  enrollment_waitlisted: "Warteliste",
  enrollment_rejected: "Absage",
  enrollment_rollover_opt_in: "Verlängerung",
  enrollment_rollover_opt_out: "Verlängerung",
  enrollment_change_request_question: "Rückfrage zur Änderung",
  enrollment_change_request_approved: "Änderung angenommen",
  enrollment_change_request_rejected: "Änderung abgelehnt",
};

// A request can come before any child exists at the school.
const CHECK_REQUEST_ADDRESS = `Ist das Kind schon aufgenommen: ${CHECK_ADDRESS_AT_GUARDIAN}`;

// What became of the mails to the family (#5934).
export function useRequestMails(requestId: string) {
  const [mails, setMails] = useState<AdminRequestMail[]>([]);
  const load = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = load;
  const refresh = useCallback(async () => {
    try {
      setMails(await getAdminRequestMails(requestId));
      clearLoadError();
    } catch (err) {
      logger.error("enrollment_request_mails_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showLoadError(err, {
        object: "der Stand der E-Mails",
        retry: () => void refresh(),
      });
    }
  }, [requestId, showLoadError, clearLoadError]);
  useEffect(() => {
    void refresh();
  }, [refresh]);
  useEffect(() => {
    const timer = setInterval(() => {
      if (document.visibilityState === "visible") void refresh();
    }, 15_000);
    return () => clearInterval(timer);
  }, [refresh]);
  return { mails, loadError: load.error, refresh };
}

/**
 * The mails to a family with their status. The defaults name the mails of an
 * enrollment request; the holiday registrations of a Träger pass their own
 * title, names and symbol (#6976, #6999). Their next step stands in the notice
 * above, so it does not repeat here. A mail with `created_at` names when it
 * came about, so two mails of one kind stay apart.
 */
export function RequestMailStatus({
  mails,
  loadError,
  title = "E-Mails an die Familie",
  labels = REQUEST_MAIL_LABELS,
  leading,
}: Readonly<{
  mails: readonly AdminRequestMail[];
  loadError: ReturnType<typeof useRequestMails>["loadError"];
  title?: string;
  labels?: Readonly<Record<string, string>>;
  /** The symbol of the card, as the other cards of the page carry one. */
  leading?: ReactNode;
}>) {
  return (
    <>
      <LoadErrorAlert error={loadError} className="mb-4" />
      {/* Stays away while no mail exists: a card that always reads „keine
          Mail“ tells the school nothing. */}
      {mails.length > 0 && (
        <SectionCard
          title={title}
          leading={leading}
          description="Hier sehen Sie, ob die Mails zu dieser Anmeldung angekommen sind."
          className="mb-4"
        >
          <ul className="divide-y divide-gray-100">
            {mails.map((mail, index) => {
              const copy = mailStatusCopy(
                mail.outcome,
                mail.address_mark,
                mail.stopped,
              );
              return (
                <li
                  key={`${mail.kind}-${index}`}
                  className="flex flex-wrap items-center justify-between gap-2 py-2"
                >
                  <span className="text-sm text-gray-900">
                    {labels[mail.kind] ?? "E-Mail"}
                    {mail.created_at && (
                      <span className="block text-xs text-gray-500">
                        am {formatBerlinDate(mail.created_at)},{" "}
                        {formatTime(mail.created_at)} Uhr
                      </span>
                    )}
                  </span>
                  {copy && (
                    <StatusBadge label={copy.label} tone={copy.tone} compact />
                  )}
                </li>
              );
            })}
          </ul>
          {mails.some((mail) => mail.address_mark) && (
            <p className="mt-2 text-sm text-gray-600">
              An die Adresse dieser Anmeldung geht keine Mail raus. Bitte
              sprechen Sie die Familie direkt an. {CHECK_REQUEST_ADDRESS}
            </p>
          )}
          {mails.some((mail) => mail.stopped && !mail.address_mark) && (
            <p className="mt-2 text-sm text-gray-600">
              {GUARDIAN_STOPPED_COPY.explanation} Bitte sprechen Sie die Familie
              direkt an.
            </p>
          )}
        </SectionCard>
      )}
    </>
  );
}
