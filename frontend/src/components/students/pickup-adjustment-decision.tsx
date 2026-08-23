"use client";

import type { RefObject } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ISODatePicker } from "~/components/ui/date-picker";
import { Input } from "~/components/ui/input";
import {
  DataField,
  DataGrid,
  InfoSection,
} from "~/components/ui/detail-modal-components";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { formatDate } from "~/lib/date-helpers";
import type {
  PickupAdjustmentCatalogItem,
  PickupAdjustmentMatch,
  PickupAdjustmentPreview,
} from "~/lib/pickup-schedule-api";

interface PickupAdjustmentDecisionProps {
  preview: PickupAdjustmentPreview;
  headingRef: RefObject<HTMLHeadingElement | null>;
  reason: string;
  selectedOfferingId: string | null;
  effectiveFrom: string;
  canSaveException: boolean;
  confirmed: boolean;
  busy: boolean;
  onReasonChange: (value: string) => void;
  onSelectOffering: (offeringId: string) => void;
  onEffectiveFromChange: (value: string) => void;
  onConfirmedChange: (value: boolean) => void;
  onSaveOffering: () => void;
  onSaveException: () => void;
}

export function PickupAdjustmentDecision(props: PickupAdjustmentDecisionProps) {
  return (
    <section
      aria-label="Angebot oder dauerhafte Ausnahme wählen"
      className="space-y-5"
    >
      <DecisionNotice preview={props.preview} headingRef={props.headingRef} />
      <PlanDiff preview={props.preview} />
      <ReasonField value={props.reason} onChange={props.onReasonChange} />
      <MatchingOfferings {...props} />
      {props.selectedOfferingId ? <OfferingDetails {...props} /> : null}
      <ExceptionAction
        busy={props.busy}
        canSave={props.canSaveException}
        onSave={props.onSaveException}
      />
    </section>
  );
}

function DecisionNotice({
  preview,
  headingRef,
}: Pick<PickupAdjustmentDecisionProps, "preview" | "headingRef">) {
  const message =
    preview.matching_offerings.length > 0
      ? "Gehzeiten passen zu einem anderen Angebot"
      : "Gehzeiten passen zu keinem Angebot";
  return (
    <div>
      <h3 ref={headingRef} tabIndex={-1} className="sr-only">
        Angebot oder dauerhafte Ausnahme wählen
      </h3>
      <Alert type="warning" message={message} />
      <p className="mt-2 text-sm leading-6 text-gray-700">
        Wählen Sie ein passendes Angebot. Oder speichern Sie die Zeiten als
        dauerhafte Ausnahme.
      </p>
    </div>
  );
}

function PlanDiff({ preview }: { readonly preview: PickupAdjustmentPreview }) {
  return (
    <DataGrid>
      <DataField label="Vorher">{preview.current_plan}</DataField>
      <DataField label="Nachher">{preview.proposed_plan}</DataField>
    </DataGrid>
  );
}

function ReasonField({
  value,
  onChange,
}: {
  readonly value: string;
  readonly onChange: (value: string) => void;
}) {
  return (
    <label className="block" htmlFor="pickup-adjustment-reason">
      <span className="mb-1 block text-sm font-medium text-gray-700">
        Grund (optional)
      </span>
      <Input
        id="pickup-adjustment-reason"
        type="text"
        controlSize="compact"
        maxLength={255}
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />
    </label>
  );
}

function MatchingOfferings(props: PickupAdjustmentDecisionProps) {
  if (props.preview.matching_offerings.length === 0) return null;
  return (
    <div className="space-y-3">
      <p className="text-sm font-semibold text-gray-900">Passendes Angebot</p>
      <div className="grid gap-2 sm:grid-cols-2">
        {props.preview.matching_offerings.map((offering) => (
          <OfferingButton
            key={offering.offering_id}
            offering={offering}
            {...props}
          />
        ))}
      </div>
    </div>
  );
}

function OfferingButton(
  props: PickupAdjustmentDecisionProps & {
    readonly offering: PickupAdjustmentMatch;
  },
) {
  const isFull = offeringHasNoFreeSlot(props.offering, props.preview);
  return (
    <div>
      <Button
        type="button"
        variant={
          props.selectedOfferingId === props.offering.offering_id
            ? "primary"
            : "outline"
        }
        size="md"
        className="w-full"
        disabled={props.busy || isFull}
        onClick={() => props.onSelectOffering(props.offering.offering_id)}
      >
        Auf „{props.offering.name}“ umbuchen
      </Button>
      {isFull ? (
        <p className="text-moto-red-strong mt-1 text-xs">
          Dieses Angebot hat keinen freien Platz.
        </p>
      ) : null}
    </div>
  );
}

function OfferingDetails(props: PickupAdjustmentDecisionProps) {
  return (
    <InfoSection
      title="Angebot ändern"
      icon={<MotoConceptIcon concept="pickup" size={16} />}
    >
      <div className="space-y-4">
        <EffectiveDateField {...props} />
        <PlanningConflicts preview={props.preview} />
        <BookingConsequences preview={props.preview} />
        <OfferingConfirmation {...props} />
      </div>
    </InfoSection>
  );
}

function EffectiveDateField(props: PickupAdjustmentDecisionProps) {
  return (
    <ISODatePicker
      id="pickup-offering-effective-from"
      label="Gilt ab"
      ariaLabel="Gilt ab"
      controlSize="md"
      value={props.effectiveFrom}
      min={props.preview.offering_catalog?.earliest_effective_from}
      max={props.preview.offering_catalog?.latest_effective_from}
      required
      hideClearButton
      disabled={props.busy}
      onChange={props.onEffectiveFromChange}
    />
  );
}

function PlanningConflicts({
  preview,
}: {
  readonly preview: PickupAdjustmentPreview;
}) {
  const conflicts =
    preview.offering_consequences?.manual_planning_conflicts ?? [];
  if (conflicts.length === 0) return null;
  return (
    <div className="space-y-2">
      <Alert
        type="warning"
        message="Diese Gruppen passen nicht zu den neuen Betreuungstagen. moto ändert sie nicht automatisch."
      />
      <p className="font-semibold">Betroffene Gruppen:</p>
      <ul className="mt-2 list-disc space-y-1 pl-5">
        {conflicts.map((conflict) => (
          <li key={conflict.activity_group_id}>
            {conflict.activity_group_name}: {formatOfferingDays(conflict.days)}
          </li>
        ))}
      </ul>
      <p className="text-sm text-gray-700">
        Nach dem Speichern: Öffnen Sie den Betreuungsplan. Entfernen Sie das
        Kind an diesen Tagen aus den Gruppen oder ändern Sie die Gruppentage.
      </p>
    </div>
  );
}

function BookingConsequences({
  preview,
}: {
  readonly preview: PickupAdjustmentPreview;
}) {
  const consequences = preview.offering_consequences;
  if (!consequences) return null;
  return (
    <div className="text-sm text-gray-800">
      <p className="font-semibold">Änderung der Buchung</p>
      <ul className="mt-2 space-y-1.5">
        {consequences.selections.map((selection) => (
          <li key={selection.offering_id}>
            <span className="font-medium">
              {catalogItem(preview, selection.offering_id)?.name ?? "Angebot"}:
            </span>{" "}
            {offeringSelectionChangeLabel(
              catalogItem(preview, selection.offering_id),
              selection.state,
              selection.days,
            )}
          </li>
        ))}
      </ul>
      {consequences.arrival_expectations_follow_bookings ? (
        <p className="mt-2 text-xs leading-5 text-gray-600">
          Die Betreuungstage folgen danach automatisch den gebuchten Angeboten.
        </p>
      ) : null}
      {(preview.removed_manual_notes?.length ?? 0) > 0 ? (
        <div className="mt-3 border-t border-gray-200 pt-3">
          <p className="font-semibold">
            Diese Gehzeit-Notizen werden entfernt:
          </p>
          <ul className="mt-1 list-disc space-y-1 pl-5">
            {preview.removed_manual_notes?.map((note) => (
              <li key={`${note.weekday}-${note.note}`}>
                {weekdayLabel(note.weekday)}: {note.note}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}

function OfferingConfirmation(props: PickupAdjustmentDecisionProps) {
  return (
    <>
      <label
        htmlFor="pickup-offering-confirmation"
        className="flex cursor-pointer items-start gap-3 text-sm text-gray-800"
      >
        <Checkbox
          id="pickup-offering-confirmation"
          checked={props.confirmed}
          onChange={(event) => props.onConfirmedChange(event.target.checked)}
        />
        <span>
          Ich bestätige: Das Angebot gilt ab {formatDate(props.effectiveFrom)}.
          Die Gehzeiten folgen dem Angebot.
          {(props.preview.removed_manual_notes?.length ?? 0) > 0
            ? " Die oben genannten Gehzeit-Notizen werden entfernt."
            : ""}
        </span>
      </label>
      <Button
        type="button"
        size="md"
        className="w-full"
        disabled={!props.confirmed || props.busy}
        onClick={props.onSaveOffering}
      >
        Angebot ändern und speichern
      </Button>
    </>
  );
}

function ExceptionAction({
  busy,
  canSave,
  onSave,
}: {
  readonly busy: boolean;
  readonly canSave: boolean;
  readonly onSave: () => void;
}) {
  return (
    <div className="border-t border-gray-200 pt-4">
      <Button
        type="button"
        variant="outline"
        size="md"
        className="w-full"
        disabled={busy || !canSave}
        onClick={onSave}
      >
        Als dauerhafte Ausnahme speichern
      </Button>
      <p className="mt-2 text-xs leading-5 text-gray-500">
        {canSave
          ? "Das gebuchte Angebot bleibt unverändert. Bei einer Abweichung steht in der Kinderkartei „Andere Zeit als im Angebot“."
          : "Eine dauerhafte Ausnahme gilt ab heute. Bearbeiten Sie die aktuelle Woche."}
      </p>
    </div>
  );
}

const OFFERING_DAY_LABELS: Record<string, string> = {
  mon: "Montag",
  tue: "Dienstag",
  wed: "Mittwoch",
  thu: "Donnerstag",
  fri: "Freitag",
  sat: "Samstag",
  sun: "Sonntag",
};

function formatOfferingDays(days: readonly string[]): string {
  return days.map((day) => OFFERING_DAY_LABELS[day] ?? day).join(", ");
}

function catalogItem(preview: PickupAdjustmentPreview, id: string) {
  return preview.offering_catalog?.items.find(
    (item) => item.offering_id === id,
  );
}

function weekdayLabel(weekday: number): string {
  const key = Object.keys(OFFERING_DAY_LABELS)[weekday - 1];
  return (key && OFFERING_DAY_LABELS[key]) || `Tag ${weekday}`;
}

function offeringHasNoFreeSlot(
  match: PickupAdjustmentMatch,
  preview: PickupAdjustmentPreview,
): boolean {
  const item = catalogItem(preview, match.offering_id);
  return Boolean(
    item &&
    !item.selected &&
    item.capacity !== undefined &&
    item.free_slots === 0,
  );
}

function offeringSelectionChangeLabel(
  item: PickupAdjustmentCatalogItem | undefined,
  state: "booked" | "removed",
  days: readonly string[],
): string {
  if (state === "removed") return "wird entfernt";
  const dayLabel = formatOfferingDays(days);
  if (!item?.selected)
    return dayLabel ? `wird für ${dayLabel} gebucht` : "wird gebucht";
  if (item.selected_days.join(",") === days.join(",")) return "bleibt gebucht";
  return dayLabel
    ? `wird auf ${dayLabel} geändert`
    : "wird ohne ausgewählte Tage gebucht";
}
