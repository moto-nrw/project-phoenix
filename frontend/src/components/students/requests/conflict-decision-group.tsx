"use client";

/**
 * Widersprüchliche Wünsche zu EINER Sache (#2267). Zwei Anfragen zum selben
 * Tag oder Wochentag lassen sich nicht nacheinander entscheiden: die zweite
 * würde die erste stillschweigend überschreiben. Deshalb legt die OGS hier
 * EIN Ergebnis fest, und alle anderen Wünsche werden abgelehnt.
 *
 * Eine echte Radio-Gruppe (ein `name`, ein `fieldset`, eine `legend`), damit
 * mit der Tastatur klar ist, dass genau eine Antwort möglich ist.
 */

import { useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { Input } from "~/components/ui/input";
import { ConfirmationModal } from "~/components/ui/modal";
import { Radio } from "~/components/ui/radio";
import { Textarea } from "~/components/ui/textarea";
import { TimeField } from "~/components/ui/time-field";
import { formatDate } from "~/lib/date-helpers";
import { DEPARTURE_WEEKDAYS } from "~/lib/student-helpers";
import { createLogger } from "~/lib/logger";
import {
  ChangeRequestStaleError,
  conflictResolutionKind,
  resolveRequestConflict,
  type ParentRequestKind,
} from "~/lib/change-request-list-api";
import type { ConflictGroup, ReviewItem } from "./case-model";
import {
  ABSENCE_STATUS_OPTIONS,
  conflictOfferingID,
  OFFERING_NO_DAY_HINT,
  STALE_REQUEST_NOTICE,
  type StaffValueInput,
} from "./request-copy";

const logger = createLogger({ component: "ConflictDecisionGroup" });

const STAFF_CHOICE = "staff";
const NONE_CHOICE = "none";

const STAFF_VALUE_LABEL = "Eigener Wert";

/** Der eigene Wert, solange er getippt wird. Je Art ist ein Teil davon aktiv. */
interface StaffDraft {
  /** Stand, Uhrzeit oder Feldwert. */
  readonly text: string;
  /** Nur beim Angebot: ab wann der eigene Wert gilt. */
  readonly date: string;
  /** Nur beim Angebot: die Wochentage. Leer = Abmeldung. */
  readonly days: readonly string[];
}

const EMPTY_DRAFT: StaffDraft = { text: "", date: "", days: [] };

/** Das Eingabefeld für den eigenen Wert, passend zur Art des Widerspruchs. */
function StaffValueField({
  kind,
  draft,
  onChange,
}: Readonly<{
  kind: StaffValueInput;
  draft: StaffDraft;
  onChange: (next: StaffDraft) => void;
}>) {
  if (kind === "status") {
    return (
      <CustomSelect
        ariaLabel={STAFF_VALUE_LABEL}
        value={draft.text}
        options={[...ABSENCE_STATUS_OPTIONS]}
        onChange={(text) => onChange({ ...draft, text })}
        placeholder="Stand wählen"
      />
    );
  }
  if (kind === "time") {
    return (
      <TimeField
        label={STAFF_VALUE_LABEL}
        hint="Uhrzeit im Format 15:30"
        placeholder="15:30"
        value={draft.text}
        onChange={(text) => onChange({ ...draft, text })}
      />
    );
  }
  if (kind === "offering") {
    return (
      <div className="space-y-2">
        <ISODatePicker
          label="Gilt ab"
          value={draft.date}
          onChange={(date) => onChange({ ...draft, date })}
        />
        <fieldset>
          <legend className="mb-1 text-sm font-medium text-gray-800">
            Tage
          </legend>
          <div className="flex flex-wrap gap-x-4 gap-y-2">
            {DEPARTURE_WEEKDAYS.map((day) => (
              <label
                key={day.key}
                htmlFor={`staff-day-${day.key}`}
                className="flex min-h-11 cursor-pointer items-center gap-2 text-sm text-gray-800"
              >
                <Checkbox
                  id={`staff-day-${day.key}`}
                  checked={draft.days.includes(day.key)}
                  onChange={(event) =>
                    onChange({
                      ...draft,
                      days: event.target.checked
                        ? [...draft.days, day.key]
                        : draft.days.filter((value) => value !== day.key),
                    })
                  }
                />
                <span>{day.label}</span>
              </label>
            ))}
          </div>
          <p className="mt-1 text-xs text-gray-600">{OFFERING_NO_DAY_HINT}</p>
        </fieldset>
      </div>
    );
  }
  return (
    <Input
      aria-label={STAFF_VALUE_LABEL}
      controlSize="compact"
      value={draft.text}
      onChange={(event) => onChange({ ...draft, text: event.target.value })}
    />
  );
}

/** Ist der eigene Wert vollständig genug zum Speichern? */
function staffDraftReady(kind: StaffValueInput, draft: StaffDraft): boolean {
  // Beim Angebot reicht das Datum: keine Tage heißt bewusst „abgemeldet".
  if (kind === "offering") return draft.date !== "";
  return draft.text.trim() !== "";
}

/** Was an das Backend geht, in der Form der jeweiligen Art. */
function staffValuePayload(
  kind: StaffValueInput,
  draft: StaffDraft,
  offeringID: string,
): unknown {
  if (kind !== "offering") return draft.text.trim();
  return {
    effective_from: draft.date,
    selections: [{ offering_id: offeringID, selected_days: [...draft.days] }],
  };
}

/** Wie der eigene Wert in der Bestätigung gelesen wird. */
function staffValueText(kind: StaffValueInput, draft: StaffDraft): string {
  if (kind === "status") {
    return (
      ABSENCE_STATUS_OPTIONS.find((option) => option.value === draft.text)
        ?.label ?? draft.text
    );
  }
  if (kind === "offering") {
    if (draft.date === "") return "";
    const days = DEPARTURE_WEEKDAYS.filter((day) =>
      draft.days.includes(day.key),
    ).map((day) => day.label);
    return `ab ${formatDate(draft.date)}: ${
      days.length > 0 ? days.join(", ") : "Abmeldung von diesem Angebot"
    }`;
  }
  return draft.text.trim();
}

/** Kurzfassung eines Wunsches, damit die Auswahl ohne Aufklappen lesbar ist. */
function requestChoiceLabel(item: ReviewItem): string {
  switch (item.request_type) {
    case "excused":
      return `${item.data.absence_status === "sick" ? "Krank" : "Entschuldigt"}: ${item.data.dates.join(", ")}`;
    case "master_data":
      return String(item.data.new_value ?? "");
    case "care_schedule":
      return (item.data.diff ?? []).map((line) => line.new).join(", ");
    case "offering":
      return (item.data.diff ?? []).map((line) => line.new).join(", ");
  }
}

export function ConflictDecisionGroup({
  group,
  onResolved,
  onStale,
}: Readonly<{
  group: ConflictGroup;
  onResolved: (notice: string) => void;
  onStale: () => void;
}>) {
  const [choice, setChoice] = useState<string | null>(null);
  const [staffDraft, setStaffDraft] = useState<StaffDraft>(EMPTY_DRAFT);
  const [reason, setReason] = useState("");
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Die Art, unter der aufgelöst wird, hängt am Schlüssel: eine Abholzeit
  // reist als care_schedule durch die Liste, wird aber als pickup_change
  // aufgelöst.
  const requestType = group.items[0]?.request_type as
    ParentRequestKind | undefined;
  const kind = requestType
    ? conflictResolutionKind(group.key, requestType)
    : undefined;
  const chosenItem = group.items.find(
    (item) => String(item.data.id) === choice,
  );
  const resultText =
    choice === STAFF_CHOICE
      ? staffValueText(group.staffValueInput, staffDraft)
      : choice === NONE_CHOICE
        ? "Keine Änderung"
        : chosenItem
          ? requestChoiceLabel(chosenItem)
          : "";

  // Fehlt noch ein Wunsch aus einer nicht geladenen Seite, darf hier nichts
  // entschieden werden: sonst bliebe genau dieser Wunsch offen stehen und der
  // Widerspruch wäre nur scheinbar geklärt.
  const missing = Math.max(0, group.expectedCount - group.items.length);
  // Eine Begründung ist hier immer Pflicht, unabhängig von der Einstellung
  // der Schule: das Ergebnis lehnt Wünsche ab, die Eltern gestellt haben.
  const ready =
    group.complete &&
    choice !== null &&
    (choice !== STAFF_CHOICE ||
      staffDraftReady(group.staffValueInput, staffDraft)) &&
    resultText !== "" &&
    reason.trim() !== "";

  const save = async () => {
    if (!kind || choice === null) return;
    setBusy(true);
    setError(null);
    try {
      await resolveRequestConflict({
        kind,
        conflictKey: group.key,
        requestIDs: group.items.map((item) => String(item.data.id)),
        expectedVersions: group.items.map((item) => item.expected_version),
        ...(choice === STAFF_CHOICE
          ? {
              staffValue: staffValuePayload(
                group.staffValueInput,
                staffDraft,
                conflictOfferingID(group.key),
              ),
            }
          : choice === NONE_CHOICE
            ? { none: true }
            : { chosenRequestID: choice }),
        reason: reason.trim(),
      });
      setConfirming(false);
      onResolved("Das Ergebnis wurde gespeichert.");
    } catch (err) {
      logger.warn("parent_request_conflict_resolve_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setConfirming(false);
      if (err instanceof ChangeRequestStaleError) {
        setError(STALE_REQUEST_NOTICE);
        onStale();
      } else {
        setError(
          err instanceof Error
            ? err.message
            : "Das Ergebnis konnte nicht gespeichert werden.",
        );
      }
      setBusy(false);
    }
  };

  const name = `conflict-${group.key}`;
  return (
    <section className="moto-content-surface rounded-2xl border p-4 shadow-sm">
      <fieldset className="space-y-3">
        <legend className="text-sm font-semibold text-gray-900">
          {group.expectedCount} Wünsche für {group.label}
        </legend>
        <p className="text-sm text-gray-600">
          Diese Wünsche widersprechen sich. Legen Sie fest, was gelten soll.
          Alle anderen Wünsche werden abgelehnt.
        </p>
        {missing > 0 && (
          <Alert
            type="warning"
            message={
              missing === 1
                ? "1 weiterer Wunsch ist noch nicht geladen. Laden Sie zuerst weitere Einträge."
                : `${missing} weitere Wünsche sind noch nicht geladen. Laden Sie zuerst weitere Einträge.`
            }
          />
        )}
        {group.items.map((item) => {
          const id = String(item.data.id);
          return (
            <label
              key={id}
              htmlFor={`${name}-${id}`}
              className="flex min-h-11 cursor-pointer items-center gap-3 text-sm text-gray-800"
            >
              <Radio
                id={`${name}-${id}`}
                name={name}
                value={id}
                checked={choice === id}
                onChange={() => setChoice(id)}
              />
              <span>{requestChoiceLabel(item)}</span>
            </label>
          );
        })}
        <div className="space-y-2">
          <label
            htmlFor={`${name}-${STAFF_CHOICE}`}
            className="flex min-h-11 cursor-pointer items-center gap-3 text-sm text-gray-800"
          >
            <Radio
              id={`${name}-${STAFF_CHOICE}`}
              name={name}
              value={STAFF_CHOICE}
              checked={choice === STAFF_CHOICE}
              onChange={() => setChoice(STAFF_CHOICE)}
            />
            <span>Eigenen Wert eintragen</span>
          </label>
          {choice === STAFF_CHOICE && (
            <StaffValueField
              kind={group.staffValueInput}
              draft={staffDraft}
              onChange={setStaffDraft}
            />
          )}
        </div>
        <label
          htmlFor={`${name}-${NONE_CHOICE}`}
          className="flex min-h-11 cursor-pointer items-center gap-3 text-sm text-gray-800"
        >
          <Radio
            id={`${name}-${NONE_CHOICE}`}
            name={name}
            value={NONE_CHOICE}
            checked={choice === NONE_CHOICE}
            onChange={() => setChoice(NONE_CHOICE)}
          />
          <span>Keine Änderung</span>
        </label>
        <label
          htmlFor={`${name}-reason`}
          className="block space-y-1 text-sm font-medium text-gray-800"
        >
          <span>Begründung</span>
          <Textarea
            id={`${name}-reason`}
            rows={2}
            value={reason}
            onChange={(event) => setReason(event.target.value)}
          />
        </label>
        {error && <Alert type="warning" message={error} />}
        <Button
          type="button"
          variant="primary"
          size="md"
          className="max-sm:min-h-11"
          disabled={!ready || busy}
          onClick={() => setConfirming(true)}
        >
          Ergebnis festlegen
        </Button>
      </fieldset>
      {confirming && (
        <ConfirmationModal
          isOpen
          onClose={() => setConfirming(false)}
          onConfirm={() => void save()}
          title="Ergebnis festlegen?"
          confirmText="Ergebnis speichern"
          cancelText="Zurück"
          isConfirmLoading={busy}
          isDismissDisabled={busy}
          mobileSheet
        >
          <div className="space-y-2 text-sm text-gray-700">
            <p>So wird es gespeichert: {resultText}</p>
            <p>Die anderen Wünsche werden abgelehnt.</p>
          </div>
        </ConfirmationModal>
      )}
    </section>
  );
}
