import { Plus, RotateCcw } from "lucide-react";

import { getStatusLabel } from "~/lib/timetable-helpers";
import type {
  EnrichedInstance,
  InstanceStaffSummary,
} from "~/lib/timetable-types";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { CustomSelect } from "~/components/ui/custom-select";
import { Input } from "~/components/ui/input";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { Radio } from "~/components/ui/radio";

import {
  isEditableAppointment,
  type PersonForm,
} from "./substitution-deviation-input";

interface PersonCardProps {
  row: InstanceStaffSummary;
  person: PersonForm;
  name: string;
  appointments: readonly EnrichedInstance[];
  substituteOptions: Array<{ value: string; label: string }>;
  canEdit: boolean;
  unstaffed: boolean;
  scopeReady: boolean;
  substituteDisabled: boolean;
  staffLoadError: boolean;
  fullyCovered: boolean;
  onUpdate: (patch: Partial<PersonForm>) => void;
  onChooseScope: (scope: "all" | "selected") => void;
  onToggleAppointment: (instanceId: string) => void;
}

interface PersonHeaderProps {
  row: InstanceStaffSummary;
  person: PersonForm;
  name: string;
  canEdit: boolean;
  onUpdate: (patch: Partial<PersonForm>) => void;
}

function PersonIdentity({ row, person, name }: PersonHeaderProps) {
  return (
    <div className="min-w-0">
      <div
        className={`truncate text-sm font-semibold ${person.absent ? "text-gray-400 line-through" : "text-gray-900"}`}
      >
        {name}
      </div>
      <div className="mt-0.5 flex items-center gap-1.5 text-[11px]">
        <span
          className={
            person.absent
              ? "text-moto-red-strong font-semibold"
              : "text-gray-500"
          }
        >
          {person.absent ? "Abwesend" : "Anwesend"}
        </span>
        {row.isPrimary && <span className="text-gray-400">• Zuständig</span>}
      </div>
    </div>
  );
}

function MarkAbsentAction({ person, name, onUpdate }: PersonHeaderProps) {
  return (
    <Button
      type="button"
      variant="outline_danger"
      size="md"
      aria-label={`${name} als abwesend markieren`}
      onClick={() =>
        onUpdate({
          absent: true,
          scope: "",
          selectedInstanceIds: person.existingAbsentIds,
          allDayAbsence: false,
        })
      }
    >
      <MotoConceptIcon concept="substitution" size={16} className="mr-1.5" />
      Abwesend
    </Button>
  );
}

function UndoAbsenceAction({ person, name, onUpdate }: PersonHeaderProps) {
  return (
    <Button
      type="button"
      variant="ghost"
      size="md"
      aria-label={`${name} ${person.wasAbsent ? "anwesend melden" : "Änderung rückgängig machen"}`}
      onClick={() =>
        onUpdate({
          absent: false,
          substituteId: "",
          scope: "",
          selectedInstanceIds: [],
          allDayAbsence: false,
        })
      }
    >
      <RotateCcw className="mr-1.5 h-4 w-4" />
      {person.wasAbsent ? "Anwesend melden" : "Rückgängig"}
    </Button>
  );
}

function PersonAction(props: PersonHeaderProps) {
  if (!props.canEdit || (props.person.absent && props.person.sickLocked)) {
    return null;
  }
  return props.person.absent ? (
    <UndoAbsenceAction {...props} />
  ) : (
    <MarkAbsentAction {...props} />
  );
}

function PersonHeader(props: PersonHeaderProps) {
  return (
    <div className="flex items-center justify-between gap-2 p-3">
      <PersonIdentity {...props} />
      <div className="shrink-0">
        <PersonAction {...props} />
      </div>
    </div>
  );
}

interface ScopeChoiceProps {
  staffId: string;
  scope: "all" | "selected";
  selected: boolean;
  title: string;
  description: string;
  onChoose: () => void;
}

function ScopeChoice({
  staffId,
  scope,
  selected,
  title,
  description,
  onChoose,
}: ScopeChoiceProps) {
  const id = `vp-scope-${scope}-${staffId}`;
  return (
    <label
      htmlFor={id}
      className="flex items-start gap-2 rounded-lg border border-gray-200 bg-white p-3"
    >
      <Radio
        id={id}
        name={`vp-scope-${staffId}`}
        value={scope}
        checked={selected}
        onChange={onChoose}
        className="mt-0.5"
      />
      <span className="text-sm text-gray-800">
        {title}
        <span className="block text-xs text-gray-500">{description}</span>
      </span>
    </label>
  );
}

interface AppointmentChoiceProps {
  staffId: string;
  appointment: EnrichedInstance;
  person: PersonForm;
  onToggle: () => void;
}

function AppointmentChoice({
  staffId,
  appointment,
  person,
  onToggle,
}: AppointmentChoiceProps) {
  const { selectable, status } = appointmentState(appointment, person);
  const id = `vp-appointment-${staffId}-${appointment.id}`;
  return (
    <li>
      <label htmlFor={id} className="flex items-start gap-2">
        <Checkbox
          id={id}
          checked={person.selectedInstanceIds.includes(appointment.id)}
          disabled={!selectable}
          onChange={onToggle}
        />
        <span
          className={`text-sm ${selectable ? "text-gray-800" : "text-gray-400"}`}
        >
          {appointment.startTime}–{appointment.endTime} · {appointment.title}
          {status}
        </span>
      </label>
    </li>
  );
}

function appointmentState(appointment: EnrichedInstance, person: PersonForm) {
  const editable = isEditableAppointment(appointment);
  const selectable =
    editable &&
    (!person.sickLocked || person.existingAbsentIds.includes(appointment.id));
  const status = !editable
    ? ` · ${getStatusLabel(appointment.status)}`
    : person.sickLocked && !selectable
      ? " · Nicht abwesend"
      : "";
  return { selectable, status };
}

interface AppointmentListProps {
  staffId: string;
  person: PersonForm;
  appointments: readonly EnrichedInstance[];
  onUpdate: (patch: Partial<PersonForm>) => void;
  onToggleAppointment: (instanceId: string) => void;
}

function AppointmentList(props: AppointmentListProps) {
  const { staffId, person, appointments, onUpdate, onToggleAppointment } =
    props;
  return (
    <div className="space-y-2 rounded-lg border border-gray-200 bg-gray-50 p-3">
      <p className="text-xs font-semibold text-gray-700">Termine auswählen</p>
      <ul className="space-y-2">
        {appointments.map((appointment) => (
          <AppointmentChoice
            key={appointment.id}
            staffId={staffId}
            appointment={appointment}
            person={person}
            onToggle={() => onToggleAppointment(appointment.id)}
          />
        ))}
      </ul>
      {!person.sickLocked && (
        <label
          htmlFor={`vp-all-day-absence-${staffId}`}
          className="flex items-start gap-2 border-t border-gray-200 pt-3"
        >
          <Checkbox
            id={`vp-all-day-absence-${staffId}`}
            checked={person.allDayAbsence}
            onChange={(event) =>
              onUpdate({ allDayAbsence: event.target.checked })
            }
          />
          <span className="text-sm text-gray-800">
            Auch in allen anderen Terminen als abwesend markieren
          </span>
        </label>
      )}
    </div>
  );
}

function ScopeChoices(props: PersonCardProps) {
  const { row, person, name, onChooseScope } = props;
  const allDescription = person.sickLocked
    ? `Die Ersatzperson übernimmt alle offenen Termine. ${name} muss dort abwesend sein.`
    : `Die Änderung gilt für diesen Tag. Alle offenen Termine von ${name} werden geändert.`;
  const selectedDescription = person.sickLocked
    ? "Die Ersatzperson übernimmt nur die ausgewählten Termine."
    : "Nur die ausgewählten Termine werden geändert.";
  return (
    <>
      <ScopeChoice
        staffId={row.staffId}
        scope="all"
        selected={person.scope === "all"}
        title="Alle noch offenen Termine"
        description={allDescription}
        onChoose={() => onChooseScope("all")}
      />
      <ScopeChoice
        staffId={row.staffId}
        scope="selected"
        selected={person.scope === "selected"}
        title="Bestimmte Termine"
        description={selectedDescription}
        onChoose={() => onChooseScope("selected")}
      />
    </>
  );
}

function PersonScope(props: PersonCardProps) {
  const { person, row } = props;
  return (
    <fieldset className="space-y-2">
      <legend className="text-xs font-semibold text-gray-800">
        {person.sickLocked
          ? "Welche Termine soll die Ersatzperson übernehmen?"
          : "Welche Termine sollen geändert werden?"}
      </legend>
      <p className="text-xs text-gray-500">Wählen Sie vor dem Speichern.</p>
      <ScopeChoices {...props} />
      {person.scope === "selected" && (
        <AppointmentList {...props} staffId={row.staffId} />
      )}
    </fieldset>
  );
}

interface SubstituteHintProps {
  unstaffed: boolean;
  scopeReady: boolean;
  fullyCovered: boolean;
  staffLoadError: boolean;
  hasOptions: boolean;
}

function SubstituteHint(props: SubstituteHintProps) {
  if (props.unstaffed) {
    return (
      <p className="mt-1 text-[11px] text-gray-400">
        Der Termin ist bewusst unbesetzt. Deshalb ist keine Vertretung möglich.
      </p>
    );
  }
  if (!props.scopeReady) {
    return (
      <p className="mt-1 text-[11px] text-gray-500">
        Wählen Sie zuerst die Termine.
      </p>
    );
  }
  if (props.fullyCovered) {
    return (
      <p className="mt-1 text-[11px] text-gray-400">
        Der Termin ist bereits vollständig vertreten. Wählen Sie zuerst
        „Entfernen“.
      </p>
    );
  }
  if (props.staffLoadError && !props.hasOptions) {
    return (
      <p className="text-moto-red-strong mt-1 text-[11px]">
        Die Personalliste konnte nicht geladen werden. Laden Sie die Seite neu.
      </p>
    );
  }
  return null;
}

function SubstitutePicker(props: PersonCardProps) {
  const { person, name, canEdit, substituteOptions, onUpdate } = props;
  return (
    <div>
      <span className="mb-1 block text-[11px] font-semibold tracking-wide text-gray-500 uppercase">
        Vertretung für {name}
      </span>
      {canEdit ? (
        <CustomSelect
          value={person.substituteId}
          options={substituteOptions}
          ariaLabel={`Vertretung für ${name}`}
          placeholder="Ersatzperson wählen…"
          disabled={props.substituteDisabled}
          onChange={(value) => onUpdate({ substituteId: value })}
        />
      ) : (
        <span className="text-sm text-gray-500">—</span>
      )}
      <SubstituteHint
        unstaffed={props.unstaffed}
        scopeReady={props.scopeReady}
        fullyCovered={props.fullyCovered}
        staffLoadError={props.staffLoadError}
        hasOptions={substituteOptions.length > 0}
      />
    </div>
  );
}

function ReasonControl({ person, onUpdate }: PersonCardProps) {
  if (person.wasAbsent) {
    return person.reason ? (
      <p className="text-xs text-gray-500">Grund: {person.reason}</p>
    ) : null;
  }
  if (person.showReason) {
    return (
      <Input
        controlSize="compact"
        value={person.reason}
        maxLength={500}
        onChange={(event) => onUpdate({ reason: event.target.value })}
        placeholder="Grund (optional)"
      />
    );
  }
  return (
    <button
      type="button"
      onClick={() => onUpdate({ showReason: true })}
      className="inline-flex items-center gap-1 text-xs font-medium text-gray-500 hover:text-gray-700"
    >
      <Plus className="h-3.5 w-3.5" />
      Grund hinzufügen
    </button>
  );
}

function AbsentDetails(props: PersonCardProps) {
  return (
    <div className="border-moto-red/15 space-y-4 rounded-b-xl border-t bg-white/60 p-3">
      {props.person.sickLocked && (
        <Alert
          type="warning"
          message="Diese Abwesenheit kommt aus einer Krankmeldung. Sie können hier nur die Vertretung ändern."
        />
      )}
      <PersonScope {...props} />
      <SubstitutePicker {...props} />
      <ReasonControl {...props} />
    </div>
  );
}

export function SubstitutionPersonCard(props: PersonCardProps) {
  const { row, person } = props;
  return (
    <li
      className={`rounded-xl border shadow-sm ${person.absent ? "border-moto-red/25 bg-moto-red/5" : "border-gray-200 bg-white"}`}
    >
      <PersonHeader
        row={row}
        person={person}
        name={props.name}
        canEdit={props.canEdit}
        onUpdate={props.onUpdate}
      />
      {person.absent && <AbsentDetails {...props} />}
    </li>
  );
}
