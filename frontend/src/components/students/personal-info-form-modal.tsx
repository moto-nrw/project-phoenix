"use client";

import { useCallback, useMemo, useState, useEffect } from "react";
import { ISODatePicker } from "~/components/ui/date-picker";
import { FormModal } from "~/components/ui/form-modal";
import { useToast } from "~/contexts/ToastContext";
import { todayISO } from "~/lib/date-helpers";
import type { ExtendedStudent } from "~/lib/hooks/use-student-data";
import { DepartureSection } from "./student-form-fields";
import {
  busDaysHaveAny,
  pickupDaysHaveAny,
  departureDaysFromLegacy,
  allowedDepartureModesFromDeparture,
  allowedDepartureToBusDays,
  allowedDepartureToDepartureDays,
  allowedDepartureToPickupDays,
  normalizeAllowedDepartureModes,
  allowedDepartureModesEqual,
  allowedDepartureModesFingerprint,
  accompaniedWeekdayKeys,
} from "~/lib/student-helpers";
import {
  companionsFingerprint,
  fetchStudentCompanions,
  mergeCompanionConfirmations,
  type CompanionExtensionConfirmation,
  type StudentCompanion,
} from "~/lib/student-companion-api";
import { useCompanionRemoteRefresh } from "~/lib/hooks/use-companion-remote-refresh";
import {
  companionDepartureMessage,
  CompanionPlanConflictError,
  companionsChangedMessage,
  isCompanionDepartureRefusal,
  isCompanionsChanged,
} from "~/lib/api";
import type { AllowedDepartureModes } from "~/lib/student-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "PersonalInfoFormModal" });

// The departure plan of a student in the one shape everything here compares
// and submits. Derived identically for the stored copy and the edited one, so
// a comparison between them sees plan changes and not shape noise.
function departureModesOf(
  source: Pick<
    ExtendedStudent,
    "allowed_departure_modes" | "departure_days" | "bus_days" | "pickup_days"
  >,
): AllowedDepartureModes {
  return normalizeAllowedDepartureModes(
    source.allowed_departure_modes ??
      allowedDepartureModesFromDeparture(
        source.departure_days ??
          departureDaysFromLegacy(source.bus_days, source.pickup_days),
      ),
  );
}

interface PersonalInfoFormModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly student: ExtendedStudent;
  readonly onSave: (student: ExtendedStudent) => Promise<void>;
}

export function PersonalInfoFormModal({
  isOpen,
  onClose,
  student,
  onSave,
}: PersonalInfoFormModalProps) {
  const toast = useToast();
  const [editedStudent, setEditedStudent] = useState<ExtendedStudent>(student);
  const [isSaving, setIsSaving] = useState(false);
  // Set when the backend refused because a linked child's own departure plan
  // does not allow the requested days. Answering yes re-sends the identical
  // payload with the confirmation flag.
  const [planConflict, setPlanConflict] = useState<string | null>(null);
  // WHAT the user confirmed, not just that they did: the backend widens a
  // companion's plan only for the children and weekdays listed here, so a
  // conflict that appeared while the question was on screen is refused again
  // instead of riding along on the earlier yes.
  const [confirmedExtensions, setConfirmedExtensions] = useState<
    CompanionExtensionConfirmation[]
  >([]);
  // The children and weekdays the OPEN question is about — not yet approved.
  // They only become confirmed extensions when the user clicks "Ergänzen und
  // speichern"; merging them on arrival would let "Abbrechen" leave a yes
  // behind that widens another child's Heimweg on the next ordinary save.
  const [pendingExtensions, setPendingExtensions] = useState<
    CompanionExtensionConfirmation[]
  >([]);
  // The submitted list REPLACES the stored one, so saving is blocked until the
  // stored links are known: an empty list from a pending or failed load would
  // silently delete the child's Laufgemeinschaft.
  const [companionsStatus, setCompanionsStatus] = useState<
    "loading" | "ready" | "error"
  >("loading");
  // The list as it was loaded, so an untouched one can be left out of the save
  // instead of overwriting whatever someone else changed in the meantime.
  const [loadedCompanions, setLoadedCompanions] = useState<StudentCompanion[]>(
    [],
  );
  const [reloadCompanions, setReloadCompanions] = useState(0);

  // Reset form when modal opens with new student data. The confirmation state
  // resets too — a yes given in an earlier session of this modal must not
  // carry over into a new edit.
  useEffect(() => {
    if (isOpen) {
      // A refresh (SWR/SSE) hands in a NEW object for the SAME child while the
      // modal is open. The companions live in their own table and are fetched
      // separately, so copying the incoming student verbatim would drop them —
      // and the fetch below would not run again, because student.id did not
      // change. Since the submitted list REPLACES the stored one, the next save
      // would delete the child's whole Laufgemeinschaft. Carry the loaded links
      // through a same-child reset; a different child starts from nothing and
      // the fetch below refills them.
      setEditedStudent((prev) =>
        prev.id === student.id
          ? { ...student, companions: prev.companions }
          : student,
      );
      setPlanConflict(null);
      setConfirmedExtensions([]);
      setPendingExtensions([]);
    }
  }, [isOpen, student]);

  // The Laufgemeinschaft lives in its own table, so it is fetched when the
  // modal opens and submitted together with the departure plan it belongs to.
  useEffect(() => {
    if (!isOpen || !student.id) return;
    let cancelled = false;
    setCompanionsStatus("loading");
    setLoadedCompanions([]);
    fetchStudentCompanions(student.id)
      .then((companions: StudentCompanion[]) => {
        if (cancelled) return;
        setEditedStudent((prev) => ({ ...prev, companions }));
        setLoadedCompanions(companions);
        setCompanionsStatus("ready");
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setCompanionsStatus("error");
        logger.error("failed to load companions", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen, student.id, reloadCompanions]);

  const companionsDirty =
    companionsStatus === "ready" &&
    companionsFingerprint(loadedCompanions) !==
      companionsFingerprint(editedStudent.companions ?? []);

  // A departure plan the USER changed is a companion-conflicting edit too, even
  // while the picker itself is untouched: the backend TRIMS the stored links to
  // the weekdays the submitted plan allows, so such a draft is a pending delete
  // of every link on the days it dropped.
  //
  // Without this, a remote companion write would be answered by a plain refetch
  // (nothing "dirty" to protect), and the refreshed list would silently become
  // the baseline this save claims: the fingerprint then describes the very edge
  // the other person just added on a removed weekday, the backend has nothing
  // left to refuse the save by, and the trim deletes their link. Treat it like
  // an edited list — keep the loaded baseline, flag the modal stale, make the
  // user reload and redo the narrowing deliberately.
  //
  // An UNTOUCHED plan stays out: the reset effect above keeps the draft equal
  // to the incoming student's plan, so this is false for it and unrelated saves
  // are not blocked by somebody else's legitimate companion change.
  const departurePlanDirty = useMemo(
    () =>
      !allowedDepartureModesEqual(
        departureModesOf(student),
        departureModesOf(editedStudent),
      ),
    [student, editedStudent],
  );

  // The modal can stay open while someone else edits the same child's links,
  // and the list it submits REPLACES the stored one. Reacting to companion
  // writes is what keeps an edit from saving on top of a snapshot that no
  // longer exists and deleting links this modal never saw.
  const reloadCompanionsFromRemote = useCallback(
    () => setReloadCompanions((count) => count + 1),
    [],
  );
  const { companionsStale, refreshFromRemote, withOwnWrite, markStale } =
    useCompanionRemoteRefresh({
      // Listening for the whole time the modal is open, including while the
      // first load is still in flight — that request can have been answered
      // before the remote write landed.
      active: isOpen,
      // Closing is the usual end of a stale flag here, but the modal also
      // reloads in place when it is handed another child — that new form must
      // not inherit the previous child's conflict warning.
      resetKey: student.id,
      hasUnsavedCompanionEdits: companionsDirty || departurePlanDirty,
      // Both halves of what a save submits about the Laufgemeinschaft: the list
      // itself and the plan the backend trims it to. The picker stays usable
      // while a save runs (only the button is disabled), so this is what tells
      // the hook that the draft has moved on from the one that was submitted.
      companionDraftKey: `${companionsFingerprint(editedStudent.companions ?? [])}#${allowedDepartureModesFingerprint(editedStudent.allowed_departure_modes)}`,
      onRefresh: reloadCompanionsFromRemote,
    });

  const updateField = <K extends keyof ExtendedStudent>(
    field: K,
    value: ExtendedStudent[K],
  ) => {
    setEditedStudent((prev) => ({ ...prev, [field]: value }));
  };

  const handleSave = async () => {
    // The edited list would replace links this modal never loaded. Refusing
    // here (instead of saving and hoping the backend's stranding check happens
    // to object) is the only reading that cannot lose someone else's work.
    if (companionsStale) {
      toast.error(
        "Die Laufgemeinschaft wurde zwischenzeitlich an anderer Stelle geändert. Bitte neu laden und die Änderung wiederholen.",
      );
      return;
    }
    const allowedDepartureModes = departureModesOf(editedStudent);
    // "Mit anderem Kind" needs to say with whom (#1694) — either a linked
    // child (better: structured, symmetric) or the free-text note for someone
    // who is not a child of this school. The cover is per weekday, exactly as
    // the backend checks it: a Monday link answers nothing for an accompanied
    // Tuesday, so every accompanied day must be covered by a link or the note.
    //
    // Skipped while the stored links are unknown: claiming "nobody is linked"
    // from a pending or failed load would block an unrelated edit with a wrong
    // reason. The backend re-checks the same rule against the stored links.
    const coveredDays = new Set(
      (editedStudent.companions ?? []).flatMap(
        (companion) => companion.weekdays,
      ),
    );
    if (
      companionsStatus === "ready" &&
      !editedStudent.departure_companion_note?.trim() &&
      accompaniedWeekdayKeys(allowedDepartureModes).some(
        (day) => !coveredDays.has(day),
      )
    ) {
      toast.error(
        "Bitte ein Kind verknüpfen oder angeben, mit welcher Person das Kind nach Hause geht",
      );
      return;
    }
    await submit(
      allowedDepartureModes,
      editedStudent.extend_companion_plans ?? false,
      confirmedExtensions,
    );
  };

  const submit = async (
    allowedDepartureModes: AllowedDepartureModes,
    extendCompanionPlans: boolean,
    confirmed: CompanionExtensionConfirmation[],
  ) => {
    setIsSaving(true);
    try {
      const busDays = allowedDepartureToBusDays(allowedDepartureModes);
      const pickupDays = allowedDepartureToPickupDays(allowedDepartureModes);
      // The plan this save replaces, derived from the incoming student the
      // same way the submitted one is derived from the edited copy, so the
      // comparison below sees plan changes and not shape noise.
      const storedDepartureModes = departureModesOf(student);
      // A plan the USER changed, as opposed to the untouched copy this modal
      // resubmits on every save. Only the former may remove links.
      const planEdited = !allowedDepartureModesEqual(
        storedDepartureModes,
        allowedDepartureModes,
      );
      // Whether the backend can answer this save with a
      // student_companions_changed echo. It broadcasts only when the write
      // actually changed links: an edited list travels with a differing
      // fingerprint, a confirmed extension widens a companion's plan, and a
      // changed departure plan can trim links off weekdays it no longer
      // allows (only possible when links exist — unknown while the load is
      // not "ready", so that case stays conservative). An edit that touches
      // neither list nor plan is never echoed; arming the grace for it would
      // swallow the next genuinely remote change instead.
      const mayAnnounceCompanions =
        companionsDirty ||
        confirmed.length > 0 ||
        (planEdited &&
          (companionsStatus !== "ready" || loadedCompanions.length > 0));
      // withOwnWrite: this save announces the companion change itself, and
      // reacting to our own announcement would flag the modal stale for a
      // change the user just made.
      await withOwnWrite(
        () =>
          onSave({
            ...editedStudent,
            // The submitted list REPLACES the stored one, so it must only travel
            // when the user actually edited it. A pending or failed load stays
            // undefined, which the page turns into "no companions key" — the
            // backend then leaves the child's Laufgemeinschaft untouched instead of
            // clearing it. An UNTOUCHED list stays out for the neighbouring reason:
            // re-sending the snapshot this modal loaded would overwrite links
            // someone else changed in the meantime, on a save that never meant to
            // touch them.
            companions: companionsDirty ? editedStudent.companions : undefined,
            // What this save claims the stored links are, so the backend can
            // refuse it instead of deleting a change someone else made since
            // this modal loaded.
            //
            // It travels with an edited LIST (which replaces the links) and with
            // an edited PLAN, because the backend trims the links to the
            // weekdays the submitted plan allows — a deliberate narrowing
            // removes links just as surely as an edited list does, and without
            // the claim the backend cannot tell it from a plan that went stale
            // while this modal was open. A save that touches neither claims
            // nothing: it removes nothing either. A pending or failed load makes
            // no claim — it has none to make.
            companions_fingerprint:
              companionsStatus === "ready" && (companionsDirty || planEdited)
                ? companionsFingerprint(loadedCompanions)
                : undefined,
            extend_companion_plans: extendCompanionPlans,
            confirmed_companion_extensions: confirmed,
            allowed_departure_modes: allowedDepartureModes,
            departure_days: allowedDepartureToDepartureDays(
              allowedDepartureModes,
            ),
            bus_days: busDays,
            buskind: busDaysHaveAny(busDays),
            pickup_days: pickupDays,
            pickup_status: pickupDaysHaveAny(pickupDays)
              ? "Wird abgeholt"
              : "Geht alleine nach Hause",
          }),
        mayAnnounceCompanions,
      );
      setPlanConflict(null);
      // One-shot: this save consumed the confirmation. The modal component
      // stays mounted across open/close, so a stale list would ride along
      // with a later save and re-widen a companion's plan unasked.
      setConfirmedExtensions([]);
      setPendingExtensions([]);
      onClose();
    } catch (err) {
      if (err instanceof CompanionPlanConflictError) {
        // Not a failure: ask, then repeat the same save confirming exactly the
        // children and weekdays the message named. The conflicts stay PENDING
        // until that yes — "Abbrechen" must leave nothing behind that a later
        // ordinary save could carry along and widen unasked.
        setPlanConflict(err.message);
        setPendingExtensions(err.conflicts);
        return;
      }
      logger.error("failed to save personal information", {
        error: err instanceof Error ? err.message : String(err),
      });
      // The stranded-companion refusal is expected and user-actionable: the
      // backend names the child whose Heimweg has to be filled in before this
      // link can go. Swallowing it into the generic text would leave the user
      // guessing what to change.
      if (isCompanionDepartureRefusal(err)) {
        toast.error(companionDepartureMessage(err));
        return;
      }
      // The backend saw what the announcement bus could not: another browser
      // replaced the links this list was built on. Nothing was written — flag
      // the modal stale so the user reloads and redoes the edit instead of
      // retrying the same save, which is exactly the write that was refused.
      if (isCompanionsChanged(err)) {
        markStale();
        toast.error(companionsChangedMessage(err));
        return;
      }
      toast.error("Fehler beim Speichern der persönlichen Informationen");
    } finally {
      setIsSaving(false);
    }
  };

  const handleCancel = () => {
    setEditedStudent(student);
    onClose();
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={handleCancel}
      title="Persönliche Infos"
      size="lg"
      mobilePosition="center"
      footer={
        <>
          <button
            type="button"
            onClick={handleCancel}
            disabled={isSaving}
            className="inline-flex items-center justify-center rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:bg-gray-50 disabled:opacity-50"
          >
            Abbrechen
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={isSaving}
            className="inline-flex items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:bg-gray-700 disabled:opacity-50"
          >
            {isSaving ? "Wird gespeichert..." : "Speichern"}
          </button>
        </>
      }
    >
      <div className="space-y-4">
        {companionsStale ? (
          <div className="rounded-lg border border-[#F78C10] bg-[#F78C10]/5 p-3">
            <p className="text-sm text-gray-900">
              Die Laufgemeinschaft dieses Kindes wurde zwischenzeitlich an
              anderer Stelle geändert. Speichern ist gesperrt, damit die fremde
              Änderung nicht überschrieben wird.
            </p>
            <p className="mt-1 text-xs text-gray-600">
              Neu laden verwirft die hier vorgenommenen Änderungen an der
              Laufgemeinschaft.
            </p>
            <div className="mt-2 flex justify-end">
              <button
                type="button"
                onClick={refreshFromRemote}
                className="inline-flex items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:bg-gray-700"
              >
                Neu laden
              </button>
            </div>
          </div>
        ) : null}
        {companionsStatus === "error" ? (
          <div className="rounded-lg border border-[#FF3130] bg-[#FF3130]/5 p-3">
            <p className="text-sm text-gray-900">
              Die Laufgemeinschaft konnte nicht geladen werden und wird unten
              nicht angezeigt. Andere Angaben lassen sich speichern, die
              bestehenden Verknüpfungen bleiben dabei unverändert.
            </p>
            <div className="mt-2 flex justify-end">
              <button
                type="button"
                onClick={() => setReloadCompanions((count) => count + 1)}
                className="inline-flex items-center justify-center rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:bg-gray-50"
              >
                Erneut laden
              </button>
            </div>
          </div>
        ) : null}
        {planConflict ? (
          <div className="rounded-lg border border-[#F78C10] bg-[#F78C10]/5 p-3">
            <p className="text-sm text-gray-900">{planConflict}</p>
            <p className="mt-1 text-xs text-gray-600">
              Soll „Anderes Kind“ im Heimweg des verknüpften Kindes ergänzt
              werden? Bestehende Heimwege bleiben erhalten.
            </p>
            <div className="mt-2 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => {
                  // Dismissing the question is a NO: drop the conflicts with
                  // it, so the next save asks again instead of widening the
                  // linked child's Heimweg on a yes that was never given.
                  setPlanConflict(null);
                  setPendingExtensions([]);
                }}
                className="inline-flex items-center justify-center rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:bg-gray-50"
              >
                Abbrechen
              </button>
              <button
                type="button"
                disabled={isSaving || companionsStale}
                onClick={() => {
                  // Same reason as in handleSave — the confirmed retry
                  // re-sends the very list that went stale.
                  if (companionsStale) return;
                  // The yes happens HERE: only now do the conflicts the
                  // question named become confirmed extensions.
                  const confirmed = mergeCompanionConfirmations(
                    confirmedExtensions,
                    pendingExtensions,
                  );
                  setConfirmedExtensions(confirmed);
                  setPendingExtensions([]);
                  updateField("extend_companion_plans", true);
                  void submit(departureModesOf(editedStudent), true, confirmed);
                }}
                className="inline-flex items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:bg-gray-700 disabled:opacity-50"
              >
                Ergänzen und speichern
              </button>
            </div>
          </div>
        ) : null}
        <TextInput
          id="modal-student-first-name"
          label="Vorname"
          value={editedStudent.first_name ?? ""}
          onChange={(value) => updateField("first_name", value)}
        />
        <TextInput
          id="modal-student-last-name"
          label="Nachname"
          value={editedStudent.second_name ?? ""}
          onChange={(value) => updateField("second_name", value)}
        />
        <TextInput
          id="modal-student-school-class"
          label="Klasse"
          value={editedStudent.school_class}
          onChange={(value) => updateField("school_class", value)}
        />
        <DateInput
          id="modal-student-birthday"
          label="Geburtsdatum"
          value={editedStudent.birthday}
          onChange={(value) => updateField("birthday", value)}
        />
        <TextInput
          id="modal-student-address-street"
          label="Straße und Hausnummer"
          value={editedStudent.address_street ?? ""}
          onChange={(value) => updateField("address_street", value)}
        />
        <TextInput
          id="modal-student-address-postal-code"
          label="PLZ"
          value={editedStudent.address_postal_code ?? ""}
          onChange={(value) => updateField("address_postal_code", value)}
        />
        <TextInput
          id="modal-student-address-city"
          label="Ort"
          value={editedStudent.address_city ?? ""}
          onChange={(value) => updateField("address_city", value)}
        />
        <DepartureSection
          companions={editedStudent.companions}
          // Editable ONLY once the stored links are known. While the fetch is
          // pending the picker would start from an empty list and be
          // overwritten the moment it resolves; after a failed load the edit is
          // discarded silently, because submit deliberately sends
          // `companions: undefined` rather than delete links it never read.
          onCompanionsChange={
            companionsStatus === "ready"
              ? (companions) => updateField("companions", companions)
              : undefined
          }
          companionStudentId={student.id}
          onCompanionExtensionConfirmed={(confirmation) => {
            updateField("extend_companion_plans", true);
            setConfirmedExtensions((current) =>
              mergeCompanionConfirmations(current, [confirmation]),
            );
          }}
          days={
            editedStudent.allowed_departure_modes ??
            allowedDepartureModesFromDeparture(
              editedStudent.departure_days ??
                departureDaysFromLegacy(
                  editedStudent.bus_days,
                  editedStudent.pickup_days,
                ),
            )
          }
          onChange={(value) => {
            const allowed = normalizeAllowedDepartureModes(value);
            const departure = allowedDepartureToDepartureDays(allowed);
            const busDays = allowedDepartureToBusDays(allowed);
            const pickupDays = allowedDepartureToPickupDays(allowed);
            setEditedStudent((prev) => ({
              ...prev,
              allowed_departure_modes: allowed,
              departure_days: departure,
              bus_days: busDays,
              buskind: busDaysHaveAny(busDays),
              pickup_days: pickupDays,
              pickup_status: pickupDaysHaveAny(pickupDays)
                ? "Wird abgeholt"
                : "Geht alleine nach Hause",
            }));
          }}
          companionNote={editedStudent.departure_companion_note}
          onCompanionNoteChange={(value) =>
            updateField("departure_companion_note", value)
          }
        />
        <TextAreaInput
          id="modal-student-health-info"
          label="Gesundheitsinformationen"
          value={editedStudent.health_info ?? ""}
          onChange={(value) => updateField("health_info", value)}
          placeholder="Allergien, Medikamente, wichtige medizinische Informationen"
          rows={3}
        />
        <TextAreaInput
          id="modal-student-supervisor-notes"
          label="Betreuernotizen"
          value={editedStudent.supervisor_notes ?? ""}
          onChange={(value) => updateField("supervisor_notes", value)}
          placeholder="Notizen für Betreuer"
          rows={3}
        />
        <TextAreaInput
          id="modal-student-extra-info"
          label="Elternnotizen"
          value={editedStudent.extra_info ?? ""}
          onChange={(value) => updateField("extra_info", value)}
          placeholder="Notizen der Eltern"
          rows={2}
        />
      </div>
    </FormModal>
  );
}

// =============================================================================
// FORM INPUT COMPONENTS
// =============================================================================

interface TextInputProps {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
}

function TextInput({ id, label, value, onChange }: Readonly<TextInputProps>) {
  return (
    <div>
      <label htmlFor={id} className="mb-1 block text-xs text-gray-500">
        {label}
      </label>
      <input
        id={id}
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
        maxLength={255}
      />
    </div>
  );
}

interface DateInputProps {
  id: string;
  label: string;
  value?: string;
  onChange: (value: string) => void;
}

function DateInput({ id, label, value, onChange }: Readonly<DateInputProps>) {
  return (
    <div>
      <label htmlFor={id} className="mb-1 block text-xs text-gray-500">
        {label}
      </label>
      <ISODatePicker
        id={id}
        // The API returns the birthday as a full timestamp here; ISODatePicker
        // takes the calendar day off it, which is what displayValue did.
        value={value ?? ""}
        onChange={onChange}
        monthYearNavigation
        max={todayISO()}
        calendarLayout="popover"
      />
    </div>
  );
}

interface TextAreaInputProps {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  rows?: number;
}

function TextAreaInput({
  id,
  label,
  value,
  onChange,
  placeholder,
  rows = 3,
}: Readonly<TextAreaInputProps>) {
  return (
    <div>
      <label htmlFor={id} className="mb-1 block text-xs text-gray-500">
        {label}
      </label>
      <textarea
        id={id}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="min-h-[80px] w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
        rows={rows}
        placeholder={placeholder}
        maxLength={2000}
      />
    </div>
  );
}
