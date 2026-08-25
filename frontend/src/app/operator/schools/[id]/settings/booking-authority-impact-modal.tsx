import type { BookingAuthorityImpact } from "~/lib/operator/operator-settings-api";
import { Alert } from "~/components/ui/alert";
import { ConfirmationModal } from "~/components/ui/modal";
import { formatDate } from "~/lib/date-helpers";

interface Props {
  readonly impact: BookingAuthorityImpact | null;
  readonly isOpen: boolean;
  readonly isLoading: boolean;
  readonly isSaving: boolean;
  readonly error: string | null;
  readonly onClose: () => void;
  readonly onConfirm: () => void;
}

export function BookingAuthorityImpactModal(props: Props) {
  const blockers = props.impact?.blockingChildren ?? [];
  return (
    <ConfirmationModal
      isOpen={props.isOpen}
      onClose={props.onClose}
      onConfirm={props.onConfirm}
      title="Buchungsmodus aktivieren?"
      confirmText="Buchungsmodus aktivieren"
      cancelText="Abbrechen"
      isConfirmLoading={props.isSaving}
      isConfirmDisabled={
        props.isLoading ||
        props.impact === null ||
        blockers.length > 0 ||
        props.error !== null
      }
    >
      <ImpactDetails {...props} />
    </ConfirmationModal>
  );
}

function ImpactDetails(props: Props) {
  const blockers = props.impact?.blockingChildren ?? [];
  return (
    <div className="space-y-4 text-sm text-gray-700">
      <p>
        Nach dem Aktivieren bestimmen die Buchungen, an welchen Tagen die Kinder
        betreut werden.
      </p>
      {props.isLoading ? <p>Auswirkungen werden geprüft …</p> : null}
      {props.error ? <Alert type="error" message={props.error} /> : null}
      {blockers.length > 0 && props.impact ? (
        <BlockingChildren impact={props.impact} />
      ) : null}
      {props.impact && blockers.length === 0 ? (
        <p className="font-medium text-gray-900">
          Alle aktuell betreuten Kinder haben mindestens einen gebuchten
          Betreuungstag.
        </p>
      ) : null}
      {props.impact ? <PlannedCompletions impact={props.impact} /> : null}
    </div>
  );
}

function BlockingChildren({ impact }: { impact: BookingAuthorityImpact }) {
  const children = impact.blockingChildren;
  return (
    <div>
      <p className="font-medium text-gray-900">
        Aktivieren nicht möglich: Für {children.length}
        {children.length === 1
          ? " Kind ist aktuell keine Betreuung gebucht."
          : " Kinder sind aktuell keine Betreuung gebucht."}
      </p>
      <ul className="mt-2 list-disc space-y-1 pl-5">
        {children.map((child) => (
          <li key={child.studentId}>
            {child.firstName} {child.lastName}
            {child.schoolClass ? " · " + child.schoolClass : ""}
          </li>
        ))}
      </ul>
      <p className="mt-2">
        Bitte klären Sie diese Buchungen zuerst. Das Aktivieren kann nicht
        übersprungen werden.
      </p>
    </div>
  );
}

function PlannedCompletions({ impact }: { impact: BookingAuthorityImpact }) {
  const count = impact.plannedCompletions.length;
  return (
    <div>
      <p className="font-medium text-gray-900">Geplante Abschlüsse: {count}</p>
      {count === 0 ? (
        <p className="mt-1">Es werden keine Abschlüsse geplant.</p>
      ) : (
        <PlannedCompletionList impact={impact} />
      )}
    </div>
  );
}

function PlannedCompletionList({ impact }: { impact: BookingAuthorityImpact }) {
  return (
    <>
      <p className="mt-1">
        moto legt diese Abschlüsse sofort an. Die Kinder bleiben bis zum letzten
        gebuchten Betreuungstag in den Arbeitslisten.
      </p>
      <ul className="mt-2 list-disc space-y-1 pl-5">
        {impact.plannedCompletions.map((child) => (
          <li key={child.studentId}>
            {child.firstName} {child.lastName}
            {child.firstBookinglessDay
              ? " · ab " + formatDate(child.firstBookinglessDay)
              : ""}
          </li>
        ))}
      </ul>
    </>
  );
}
