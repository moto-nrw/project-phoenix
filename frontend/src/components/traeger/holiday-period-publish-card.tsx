"use client";

// Veröffentlichen eines Ferienzeitraums (#4659). Die Karte sagt, was fehlt,
// bevor der Knopf wirkt, und was danach gilt. Ein veröffentlichter Zeitraum
// zeigt den einen Link für Eltern und die Zahl der Anmeldungen (#4661).

import { useState } from "react";
import { PublicLinkCopyButton } from "~/components/enrollment/public-link-copy-button";
import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { parentsPortalUrl } from "~/lib/parent-url";
import { type HolidayPeriod, publishHolidayPeriod } from "~/lib/traeger-api";
import { holidayPeriodComplete } from "./holiday-period-model";

const logger = createLogger({ component: "HolidayPeriodPublishCard" });

function publishBlocker(
  period: HolidayPeriod,
  unsavedChanges: boolean,
): string | null {
  if (unsavedChanges) {
    return "Bitte speichern Sie zuerst Ihre Änderungen.";
  }
  if (!holidayPeriodComplete(period)) {
    return "Zum Veröffentlichen braucht der Ferienzeitraum mindestens eine Woche. Jede Woche braucht mindestens ein Zeitmodell.";
  }
  return null;
}

export function HolidayPeriodPublishCard({
  period,
  unsavedChanges,
  onPublished,
}: Readonly<{
  period: HolidayPeriod;
  unsavedChanges: boolean;
  onPublished: (period: HolidayPeriod) => void;
}>) {
  const [confirming, setConfirming] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const errorDisplay = useApiErrorDisplay();

  if (period.status === "published") {
    return (
      <SectionCard title="Veröffentlicht">
        <div className="space-y-4 text-sm text-gray-700">
          <p>
            {period.publishedAt
              ? `Seit ${formatDate(period.publishedAt)} für die Anmeldung freigegeben.`
              : "Für die Anmeldung freigegeben."}{" "}
            Änderungen gelten sofort nach dem Speichern.
          </p>
          <HolidayRegistrationLink period={period} />
          <p>{registrationCountText(period.registrationCount)}</p>
        </div>
      </SectionCard>
    );
  }

  const blocker = publishBlocker(period, unsavedChanges);
  const publish = async () => {
    setPublishing(true);
    try {
      onPublished(await publishHolidayPeriod(period.id));
      setConfirming(false);
    } catch (err) {
      logger.error("holiday_period_publish_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setConfirming(false);
      void errorDisplay.show(err, { object: "den Ferienzeitraum" });
    } finally {
      setPublishing(false);
    }
  };

  return (
    <SectionCard
      title="Entwurf"
      description="Ein Entwurf ist noch nicht für die Anmeldung freigegeben."
      action={
        <Button
          type="button"
          variant="primary"
          size="md"
          disabled={blocker !== null}
          aria-describedby={blocker ? "holiday-publish-blocker" : undefined}
          onClick={() => setConfirming(true)}
        >
          Veröffentlichen
        </Button>
      }
    >
      <div className="space-y-2 text-sm text-gray-700">
        {blocker ? <p id="holiday-publish-blocker">{blocker}</p> : null}
        <p>Den Link für Eltern gibt es nach dem Veröffentlichen.</p>
      </div>
      <ConfirmationModal
        isOpen={confirming}
        title="Ferienzeitraum veröffentlichen?"
        confirmText="Veröffentlichen"
        isConfirmLoading={publishing}
        loadingText="Wird veröffentlicht…"
        onClose={() => setConfirming(false)}
        onConfirm={() => void publish()}
      >
        <p className="text-sm text-gray-700">
          Danach ist der Ferienzeitraum für die Anmeldung freigegeben. Sie
          können ihn weiter ändern, aber nicht mehr zum Entwurf machen.
        </p>
      </ConfirmationModal>
    </SectionCard>
  );
}

function registrationCountText(count: number): string {
  if (count === 0) return "Es ist noch keine Anmeldung eingegangen.";
  if (count === 1) return "Bisher ist 1 Anmeldung eingegangen.";
  return `Bisher sind ${count} Anmeldungen eingegangen.`;
}

/** Der eine Link des Zeitraums für alle seine Schulen, im Elternportal. */
function HolidayRegistrationLink({
  period,
}: Readonly<{ period: HolidayPeriod }>) {
  if (!period.publicToken) return null;
  const url = parentsPortalUrl(
    `/ferienanmeldung/${encodeURIComponent(period.publicToken)}`,
  );
  return (
    <div className="space-y-1">
      <p className="font-semibold text-gray-900">Link für Eltern</p>
      <p>
        Über diesen Link melden Eltern ihre Kinder für alle Wochen an. Ein
        Elternkonto brauchen sie nicht.
      </p>
      <div className="flex items-center gap-2">
        <span className="min-w-0 flex-1 truncate font-mono text-xs text-gray-600 select-all">
          {url}
        </span>
        <PublicLinkCopyButton
          url={url}
          componentId={`holiday-registration-link-${period.id}`}
          label="Link kopieren"
        />
      </div>
    </div>
  );
}
