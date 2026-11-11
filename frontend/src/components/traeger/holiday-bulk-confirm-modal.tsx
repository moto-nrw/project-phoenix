"use client";

// Mehrere Ferienanmeldungen auf einmal bestätigen (#6674): Der Dialog zeigt
// vorher, welche Anmeldungen bestätigt werden und welche mit welchem Grund
// offen bleiben, bestätigt dann eine nach der anderen über dieselbe
// Bestätigung wie die Seite der Anmeldung und zeigt am Ende das Ergebnis.

import { Check } from "lucide-react";
import { useRef, useState } from "react";
import {
  bulkConfirmOutcome,
  type HolidayBulkPlan,
  type HolidayBulkSkip,
  type HolidayBulkSkipReason,
} from "~/components/traeger/holiday-bulk-confirm-model";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { FramedList } from "~/components/ui/framed-list";
import { ListGroupHeader } from "~/components/ui/list-group-header";
import { ListRowLink } from "~/components/ui/list-row-link";
import { Modal } from "~/components/ui/modal";
import { ProgressBar } from "~/components/ui/progress-bar";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { confirmHolidayRegistration } from "~/lib/traeger-contributions-api";
import type { HolidayRegistrationSummary } from "~/lib/traeger-holiday-registrations-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "HolidayBulkConfirmModal" });

/** Each reason a registration stays open, in the order the office works. */
const OPEN_REASONS: Readonly<
  Record<HolidayBulkSkipReason, { title: string; hint: string }>
> = {
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
  changed: {
    title: "Inzwischen geändert",
    hint: "Die Anmeldung wurde gerade geändert. Öffnen Sie sie und prüfen Sie den Stand.",
  },
  failed: {
    title: "Hat nicht geklappt",
    hint: "Öffnen Sie die Anmeldung und bestätigen Sie sie dort.",
  },
  stopped: {
    title: "Nicht mehr bestätigt",
    hint: "Das Bestätigen wurde angehalten. Wählen Sie diese Anmeldungen noch einmal aus.",
  },
  not_received: {
    title: "Schon bestätigt oder storniert",
    hint: "Hier ist nichts mehr zu bestätigen.",
  },
};

const REASON_ORDER = Object.keys(OPEN_REASONS) as HolidayBulkSkipReason[];

function anmeldungen(count: number): string {
  return count === 1 ? "1 Anmeldung" : `${count} Anmeldungen`;
}

type Phase =
  | { readonly kind: "review" }
  | { readonly kind: "running"; readonly done: number }
  | {
      readonly kind: "done";
      readonly confirmed: readonly HolidayRegistrationSummary[];
      readonly open: readonly HolidayBulkSkip[];
    };

function RegistrationRow({
  periodId,
  registration,
}: Readonly<{
  periodId: string;
  registration: HolidayRegistrationSummary;
}>) {
  const children = registration.children
    .map((child) => child.firstName)
    .join(", ");
  return (
    <li>
      <ListRowLink
        href={traegerPath(
          `/traeger/ferien/${periodId}/anmeldungen/${registration.id}`,
        )}
        title={`${registration.guardianFirstName} ${registration.guardianLastName}`}
        subtitle={
          children
            ? `Nr. ${registration.id} · ${children}`
            : `Nr. ${registration.id}`
        }
      />
    </li>
  );
}

/**
 * One framed list in groups: what is (or was) confirmed, then every reason a
 * registration stays open, its hint once above its rows.
 */
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
  const groups = REASON_ORDER.map((reason) => ({
    reason,
    rows: open.filter((entry) => entry.reason === reason),
  })).filter((group) => group.rows.length > 0);
  return (
    <FramedList maxHeight="lg">
      {confirmed.length > 0 ? (
        <>
          <li>
            <ListGroupHeader
              title={confirmedTitle}
              meta={confirmed.length}
              as="h3"
            />
          </li>
          {confirmed.map((registration) => (
            <RegistrationRow
              key={registration.id}
              periodId={periodId}
              registration={registration}
            />
          ))}
        </>
      ) : null}
      {groups.map((group) => (
        <GroupRows
          key={group.reason}
          periodId={periodId}
          reason={group.reason}
          rows={group.rows}
        />
      ))}
    </FramedList>
  );
}

function GroupRows({
  periodId,
  reason,
  rows,
}: Readonly<{
  periodId: string;
  reason: HolidayBulkSkipReason;
  rows: readonly HolidayBulkSkip[];
}>) {
  const { title, hint } = OPEN_REASONS[reason];
  return (
    <>
      <li className="bg-gray-50">
        <ListGroupHeader title={title} meta={rows.length} as="h3" />
        <p className="px-4 pb-2 text-xs text-gray-600">{hint}</p>
      </li>
      {rows.map((entry) => (
        <RegistrationRow
          key={entry.registration.id}
          periodId={periodId}
          registration={entry.registration}
        />
      ))}
    </>
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

  const run = async () => {
    stopRequested.current = false;
    setStopping(false);
    setPhase({ kind: "running", done: 0 });
    const confirmed: HolidayRegistrationSummary[] = [];
    const open: HolidayBulkSkip[] = [];
    let next = 0;
    // One after another: every confirmation holds the invoices of the
    // period, so they would wait for each other anyway, and a refusal stays
    // with its own registration.
    while (next < total && !stopRequested.current) {
      const registration = plan.confirmable[next]!;
      try {
        await confirmHolidayRegistration(periodId, registration.id, false);
        confirmed.push(registration);
      } catch (err) {
        const outcome = bulkConfirmOutcome(err);
        logger.warn("holiday_bulk_confirm_registration_failed", {
          registration_id: registration.id,
          error: err instanceof Error ? err.message : String(err),
        });
        if (outcome === "stop") {
          // No further confirmation can succeed; this one is unknown.
          void showActionError(err, { object: "die Anmeldungen" });
          break;
        }
        open.push({ registration, reason: outcome.reason });
      }
      next += 1;
      setPhase({ kind: "running", done: next });
    }
    const stopped = plan.confirmable
      .slice(next)
      .map((registration) => ({ registration, reason: "stopped" as const }));
    setPhase({
      kind: "done",
      confirmed,
      open: [...open, ...plan.skipped, ...stopped],
    });
  };

  const close = () => {
    if (phase.kind === "running") return;
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
            ? `${anmeldungen(count)} bestätigt`
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
                : `${phase.open.length} Anmeldungen bleiben offen. Bestätigen Sie sie einzeln.`}
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
          ? `${anmeldungen(total)} bestätigen?`
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
