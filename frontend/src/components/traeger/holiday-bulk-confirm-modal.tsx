"use client";

// Mehrere Ferienanmeldungen auf einmal bestätigen (#6674): Der Dialog zeigt
// vorher, welche Anmeldungen bestätigt werden und welche mit welchem Grund
// offen bleiben, bestätigt dann eine nach der anderen über dieselbe
// Bestätigung wie die Seite der Anmeldung und zeigt am Ende das Ergebnis.

import { Check } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import {
  bulkConfirmOutcome,
  type HolidayBulkPlan,
  type HolidayBulkSkip,
  type HolidayBulkSkipReason,
  registrationCount,
} from "~/components/traeger/holiday-bulk-confirm-model";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { FramedList } from "~/components/ui/framed-list";
import { ListGroupHeader } from "~/components/ui/list-group-header";
import { ListRowLink } from "~/components/ui/list-row-link";
import { Modal } from "~/components/ui/modal";
import { ProgressBar } from "~/components/ui/progress-bar";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { presentError } from "~/lib/error-presentation";
import { createLogger } from "~/lib/logger";
import { confirmHolidayRegistration } from "~/lib/traeger-contributions-api";
import type { HolidayRegistrationSummary } from "~/lib/traeger-holiday-registrations-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "HolidayBulkConfirmModal" });

/** What each reason is called and what the office does next. */
const OPEN_REASONS: Readonly<
  Record<HolidayBulkSkipReason, { title: string; hint: string }>
> = {
  unclear: {
    title: "Ergebnis unklar",
    hint: "Die Antwort kam nicht an. Öffnen Sie die Anmeldung und prüfen Sie den Status.",
  },
  failed: {
    title: "Hat nicht geklappt",
    hint: "Öffnen Sie die Anmeldung und bestätigen Sie sie dort.",
  },
  changed: {
    title: "Inzwischen geändert",
    hint: "Die Anmeldung wurde gerade geändert. Öffnen Sie sie und prüfen Sie den Stand.",
  },
  interrupted: {
    title: "Noch nicht bestätigt",
    hint: "moto konnte nicht weiter bestätigen. Laden Sie die Seite neu und versuchen Sie es noch einmal.",
  },
  stopped: {
    title: "Angehalten",
    hint: "Sie haben das Bestätigen angehalten. Diese Anmeldungen sind noch nicht bestätigt.",
  },
  place_open: {
    title: "Platz noch offen",
    hint: "Entscheiden Sie zuerst im Reiter „Vergabe“ über den Platz.",
  },
  payer_open: {
    title: "Wer bezahlt, ist offen",
    hint: "Die Eltern geben Leistungsbezug an. Prüfen Sie den Nachweis und wählen Sie in der Anmeldung, wer bezahlt.",
  },
  duplicate: {
    title: "Vielleicht doppelt angemeldet",
    hint: "Prüfen Sie zuerst, ob ein Kind zweimal angemeldet ist.",
  },
  debtor_number_missing: {
    title: "Debitorennummer fehlt",
    hint: "Tragen Sie die Nummer in der Anmeldung ein und bestätigen Sie dort.",
  },
  not_received: {
    title: "Schon bestätigt oder storniert",
    hint: "Hier ist nichts mehr zu bestätigen.",
  },
};

/**
 * The order of the groups: what the run left unclear first, then what the
 * office decides before a place, a payer or a number, last what needs
 * nothing.
 */
const REASON_ORDER: readonly HolidayBulkSkipReason[] = [
  "unclear",
  "failed",
  "changed",
  "interrupted",
  "stopped",
  "place_open",
  "payer_open",
  "duplicate",
  "debtor_number_missing",
  "not_received",
];

type Phase =
  | { readonly kind: "review" }
  | { readonly kind: "running"; readonly done: number }
  | {
      readonly kind: "done";
      readonly confirmed: readonly HolidayRegistrationSummary[];
      readonly open: readonly HolidayBulkSkip[];
    };

interface GroupRow {
  readonly registration: HolidayRegistrationSummary;
  readonly note?: string;
}

/** One group of a framed list: its head, its hint once, then its rows. */
function RegistrationGroup({
  periodId,
  title,
  hint,
  rows,
}: Readonly<{
  periodId: string;
  title: string;
  hint?: string;
  rows: readonly GroupRow[];
}>) {
  return (
    <>
      <li>
        <ListGroupHeader
          title={title}
          meta={rows.length}
          description={hint}
          as="h3"
        />
      </li>
      {rows.map(({ registration, note }) => {
        const children = registration.children
          .map((child) => child.firstName)
          .join(", ");
        const subtitle = [`Nr. ${registration.id}`, note ?? children]
          .filter(Boolean)
          .join(" · ");
        return (
          <li key={registration.id}>
            <ListRowLink
              href={traegerPath(
                `/traeger/ferien/${periodId}/anmeldungen/${registration.id}`,
              )}
              title={`${registration.guardianFirstName} ${registration.guardianLastName}`}
              subtitle={subtitle}
            />
          </li>
        );
      })}
    </>
  );
}

/** What is (or was) confirmed, then every reason a registration stays open. */
function RegistrationGroups({
  periodId,
  confirmedTitle,
  confirmed,
  open,
}: Readonly<{
  periodId: string;
  confirmedTitle: string;
  confirmed: readonly HolidayRegistrationSummary[];
  open: readonly HolidayBulkSkip[];
}>) {
  return (
    <FramedList maxHeight="lg">
      {confirmed.length > 0 ? (
        <RegistrationGroup
          periodId={periodId}
          title={confirmedTitle}
          rows={confirmed.map((registration) => ({ registration }))}
        />
      ) : null}
      {REASON_ORDER.map((reason) => {
        const rows = open.filter((entry) => entry.reason === reason);
        return rows.length > 0 ? (
          <RegistrationGroup
            key={reason}
            periodId={periodId}
            title={OPEN_REASONS[reason].title}
            hint={OPEN_REASONS[reason].hint}
            rows={rows}
          />
        ) : null;
      })}
    </FramedList>
  );
}

export function HolidayBulkConfirmModal({
  periodId,
  plan,
  isOpen,
  onClose,
}: Readonly<{
  periodId: string;
  /** What the selection confirms and what stays open, decided on opening. */
  plan: HolidayBulkPlan;
  isOpen: boolean;
  /** `changed`: at least one registration was confirmed. */
  onClose: (changed: boolean) => void;
}>) {
  const [phase, setPhase] = useState<Phase>({ kind: "review" });
  // The loop reads the ref; the button shows the state.
  const stopRequested = useRef(false);
  const [stopping, setStopping] = useState(false);
  const { show: showActionError } = useApiErrorDisplay();
  const total = plan.confirmable.length;
  const running = phase.kind === "running";

  // Leaving the page ends the run after the confirmation in flight; nothing
  // goes on unseen.
  useEffect(
    () => () => {
      stopRequested.current = true;
    },
    [],
  );

  // Closing the tab asks first while confirmations run.
  useEffect(() => {
    if (!running) return;
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [running]);

  const run = async () => {
    stopRequested.current = false;
    setStopping(false);
    setPhase({ kind: "running", done: 0 });
    const confirmed: HolidayRegistrationSummary[] = [];
    const open: HolidayBulkSkip[] = [];
    let next = 0;
    let rest: "stopped" | "interrupted" = "stopped";
    // One after another: every confirmation holds the invoices of the
    // period, so they would wait for each other anyway, and a refusal stays
    // with its own registration.
    while (next < total && !stopRequested.current) {
      const registration = plan.confirmable[next]!;
      next += 1;
      try {
        await confirmHolidayRegistration(periodId, registration.id, false);
        confirmed.push(registration);
      } catch (err) {
        logger.warn("holiday_bulk_confirm_registration_failed", {
          registration_id: registration.id,
          error: err instanceof Error ? err.message : String(err),
        });
        const outcome = bulkConfirmOutcome(err);
        open.push({
          registration,
          reason: outcome.reason,
          note:
            outcome.reason === "failed"
              ? presentError(err, "die Anmeldung").message
              : undefined,
        });
        if (outcome.stop) {
          // No further confirmation can succeed.
          void showActionError(err, { object: "die Anmeldungen" });
          rest = "interrupted";
          break;
        }
      }
      setPhase({ kind: "running", done: next });
    }
    const notReached = plan.confirmable
      .slice(next)
      .map((registration) => ({ registration, reason: rest }));
    setPhase({
      kind: "done",
      confirmed,
      open: [...open, ...notReached, ...plan.skipped],
    });
  };

  const close = () => {
    if (running) return;
    onClose(phase.kind === "done" && phase.confirmed.length > 0);
    setPhase({ kind: "review" });
  };

  if (phase.kind === "running") {
    return (
      <Modal
        isOpen={isOpen}
        onClose={close}
        title="Anmeldungen werden bestätigt"
        description="Bitte lassen Sie diese Seite offen, bis alle bestätigt sind."
        isDismissDisabled
        busy
        mobileSheet
        footer={
          <Button
            type="button"
            variant="outline"
            size="md"
            disabled={stopping}
            onClick={() => {
              stopRequested.current = true;
              setStopping(true);
            }}
            className="flex-1"
          >
            {stopping ? "Hält an …" : "Anhalten"}
          </Button>
        }
      >
        <div className="space-y-3 py-2">
          <p
            className="text-sm font-medium text-gray-900 tabular-nums"
            aria-live="polite"
          >
            {phase.done} von {total} bestätigt
          </p>
          <ProgressBar
            value={phase.done}
            max={total}
            label={`${phase.done} von ${total} Anmeldungen bestätigt`}
          />
        </div>
      </Modal>
    );
  }

  if (phase.kind === "done") {
    const count = phase.confirmed.length;
    return (
      <Modal
        isOpen={isOpen}
        onClose={close}
        title={
          count > 0
            ? `${registrationCount(count)} bestätigt`
            : "Keine Anmeldung bestätigt"
        }
        description={count > 0 ? "Die Eltern bekommen eine E-Mail." : undefined}
        mobileSheet
        footer={
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={close}
            className="flex-1"
          >
            Fertig
          </Button>
        }
      >
        {phase.open.length === 0 ? (
          <EmptyState
            tone="success"
            icon={<Check />}
            description="Alle ausgewählten Anmeldungen sind bestätigt."
            className="py-6"
          />
        ) : (
          <div className="space-y-3">
            <p className="text-sm text-gray-700">
              {phase.open.length === 1
                ? "1 Anmeldung bleibt offen. Bestätigen Sie sie einzeln."
                : `${registrationCount(phase.open.length)} bleiben offen. Bestätigen Sie sie einzeln.`}
            </p>
            <RegistrationGroups
              periodId={periodId}
              confirmedTitle="Bestätigt"
              confirmed={phase.confirmed}
              open={phase.open}
            />
          </div>
        )}
      </Modal>
    );
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={close}
      title={
        total > 0
          ? `${registrationCount(total)} bestätigen?`
          : "Keine Anmeldung lässt sich hier bestätigen"
      }
      description={
        total > 0
          ? "Die Familie bezahlt den Ferienbeitrag. Die Eltern bekommen eine E-Mail mit den gebuchten Wochen."
          : "Bestätigen Sie diese Anmeldungen einzeln in der Anmeldung."
      }
      mobileSheet
      footer={
        total > 0 ? (
          <>
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={close}
              className="flex-1"
            >
              Abbrechen
            </Button>
            <Button
              type="button"
              variant="primary"
              size="md"
              onClick={() => void run()}
              className="flex-1"
            >
              {total === 1 ? "Bestätigen" : `${total} bestätigen`}
            </Button>
          </>
        ) : (
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={close}
            className="flex-1"
          >
            Schließen
          </Button>
        )
      }
    >
      <RegistrationGroups
        periodId={periodId}
        confirmedTitle="Wird bestätigt"
        confirmed={plan.confirmable}
        open={plan.skipped}
      />
    </Modal>
  );
}
