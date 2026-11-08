"use client";

import { useCallback, useEffect, useState } from "react";
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

export function RequestMailStatus({
  mails,
  loadError,
}: Readonly<{
  mails: readonly AdminRequestMail[];
  loadError: ReturnType<typeof useRequestMails>["loadError"];
}>) {
  return (
    <>
      <LoadErrorAlert error={loadError} className="mb-4" />
      {/* Stays away while no mail exists: a card that always reads „keine
          Mail“ tells the school nothing. */}
      {mails.length > 0 && (
        <SectionCard
          title="E-Mails an die Familie"
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
                    {REQUEST_MAIL_LABELS[mail.kind] ?? "E-Mail"}
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
