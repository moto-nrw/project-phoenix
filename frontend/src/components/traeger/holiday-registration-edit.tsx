"use client";

// Die Angaben einer Ferienanmeldung ändern (#5629): Eltern, ein Kind,
// Notfallkontakte und Abholung, die allgemeinen Antworten. Jede Karte der
// Einzelansicht wechselt an Ort und Stelle in diesen Bearbeiten-Zustand.
// Die Felder sind die Bausteine des öffentlichen Formulars; die Feldpfade
// sind die des Backends, so landet ein Fehler an seinem Feld. Wochen ändert
// nur „Umbuchen“, Gesundheitsangaben ändern sich hier nicht.

import { type FormEvent, type ReactNode, useRef, useState } from "react";
import { CustomFieldInput } from "~/components/enrollment/custom-field-input";
import {
  DateOfBirthPicker,
  type DateOfBirthTranslator,
} from "~/components/enrollment/date-of-birth-picker";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FieldLabel } from "~/components/ui/field-label";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { useApiFormError } from "~/contexts/ToastContext";
import {
  buildFieldsByKey,
  type ConditionContext,
  isFieldVisible,
} from "~/lib/enrollment-field-visibility";
import type { HolidayFormField } from "~/lib/holiday-form-field";
import { createLogger } from "~/lib/logger";
import type { TraegerSchool } from "~/lib/traeger-api";
import {
  editHolidayRegistration,
  type HolidayRegistrationChild,
  type HolidayRegistrationContactEdit,
  type HolidayRegistrationDetail,
  type HolidayRegistrationEdit,
  type HolidayRegistrationPersonEdit,
} from "~/lib/traeger-holiday-registrations-api";
import {
  EDIT_HINT,
  editableQuestions,
} from "./holiday-registration-edit-fields";

const logger = createLogger({ component: "HolidayRegistrationEdit" });

// Limits of the public form; the backend checks them as well.
const MAX_EMERGENCY_CONTACTS = 3;
const MAX_PICKUP_PERSONS = 5;
const GUEST = "guest";

const MONTHS = [
  "Januar",
  "Februar",
  "März",
  "April",
  "Mai",
  "Juni",
  "Juli",
  "August",
  "September",
  "Oktober",
  "November",
  "Dezember",
];
const FIELD_TEXT: Readonly<Record<string, string>> = {
  "structured.yes": "Ja",
  "structured.no": "Nein",
  "structured.day": "Tag",
  "structured.month": "Monat",
  "structured.year": "Jahr",
};

/**
 * The texts the shared field blocks ask for. The Träger portal speaks German
 * only and has no translation provider.
 */
const germanText: DateOfBirthTranslator = Object.assign(
  (key: string) => FIELD_TEXT[key] ?? key,
  { raw: (key: string): unknown => (key === "months" ? MONTHS : undefined) },
);

export interface HolidayRegistrationEditorProps {
  readonly periodId: string;
  readonly registration: HolidayRegistrationDetail;
  readonly onSaved: (registration: HolidayRegistrationDetail) => void;
  readonly onCancel: () => void;
}

/** One edit state: the hint, the error at the top, the fields, the footer. */
function EditForm({
  periodId,
  registration,
  onSaved,
  onCancel,
  edit,
  children,
}: HolidayRegistrationEditorProps &
  Readonly<{
    edit: () => HolidayRegistrationEdit;
    children: (fieldError: (name: string) => string | undefined) => ReactNode;
  }>) {
  const formRef = useRef<HTMLFormElement>(null);
  const errors = useApiFormError(formRef);
  const [saving, setSaving] = useState(false);
  const fieldError = (name: string) => errors.fieldError(name) ?? undefined;

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setSaving(true);
    errors.clear();
    try {
      onSaved(await editHolidayRegistration(periodId, registration.id, edit()));
    } catch (err) {
      logger.error("holiday_registration_edit_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void errors.show(err, { object: "die Angaben" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <form
      ref={formRef}
      noValidate
      className="space-y-4"
      onSubmit={(event) => void submit(event)}
    >
      <FormErrorAlert message={errors.error} />
      <p className="text-sm text-gray-700">{EDIT_HINT}</p>
      {children(fieldError)}
      <EditActions
        onCancel={() => {
          errors.clear();
          onCancel();
        }}
        saving={saving}
        saveLabel="Änderung speichern"
      />
    </form>
  );
}

function PersonFields({
  prefix,
  person,
  optionalContact,
  fieldError,
  onChange,
}: Readonly<{
  prefix: string;
  person: HolidayRegistrationPersonEdit;
  /** E-mail and phone are optional for the second guardian. */
  optionalContact: boolean;
  fieldError: (name: string) => string | undefined;
  onChange: (person: HolidayRegistrationPersonEdit) => void;
}>) {
  const field = (
    key: keyof HolidayRegistrationPersonEdit,
    path: string,
    label: string,
    props: Readonly<{ type?: string; required: boolean; maxLength: number }>,
  ) => (
    <Input
      name={`${prefix}.${path}`}
      label={label}
      type={props.type}
      required={props.required}
      maxLength={props.maxLength}
      value={person[key]}
      error={fieldError(`${prefix}.${path}`)}
      onChange={(event) => onChange({ ...person, [key]: event.target.value })}
    />
  );
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
      {field("firstName", "first_name", "Vorname", {
        required: true,
        maxLength: 100,
      })}
      {field("lastName", "last_name", "Nachname", {
        required: true,
        maxLength: 100,
      })}
      {field(
        "email",
        "email",
        optionalContact ? "E-Mail (freiwillig)" : "E-Mail",
        { type: "email", required: !optionalContact, maxLength: 254 },
      )}
      {field(
        "phone",
        "phone",
        optionalContact ? "Telefon (freiwillig)" : "Telefon",
        { type: "tel", required: !optionalContact, maxLength: 40 },
      )}
    </div>
  );
}

const EMPTY_PERSON: HolidayRegistrationPersonEdit = {
  firstName: "",
  lastName: "",
  email: "",
  phone: "",
};

/** The parent who registered and the optional second guardian. */
export function GuardiansEditor(props: HolidayRegistrationEditorProps) {
  const { registration } = props;
  const [guardian, setGuardian] = useState<HolidayRegistrationPersonEdit>({
    firstName: registration.guardianFirstName,
    lastName: registration.guardianLastName,
    email: registration.guardianEmail,
    phone: registration.guardianPhone,
  });
  const [second, setSecond] = useState<HolidayRegistrationPersonEdit | null>(
    registration.secondGuardian,
  );
  return (
    <EditForm
      {...props}
      edit={() => ({ guardians: { guardian, secondGuardian: second } })}
    >
      {(fieldError) => (
        <>
          <fieldset className="space-y-3">
            <legend className="text-sm font-semibold text-gray-900">
              Elternteil
            </legend>
            <PersonFields
              prefix="guardian"
              person={guardian}
              optionalContact={false}
              fieldError={fieldError}
              onChange={setGuardian}
            />
          </fieldset>
          <fieldset className="space-y-3">
            <legend className="text-sm font-semibold text-gray-900">
              Zweite sorgeberechtigte Person
            </legend>
            {second ? (
              <>
                <PersonFields
                  prefix="second_guardian"
                  person={second}
                  optionalContact
                  fieldError={fieldError}
                  onChange={setSecond}
                />
                <Button
                  type="button"
                  variant="outline_danger"
                  size="md"
                  onClick={() => setSecond(null)}
                >
                  Zweite Person entfernen
                </Button>
              </>
            ) : (
              <Button
                type="button"
                variant="outline"
                size="md"
                onClick={() => setSecond(EMPTY_PERSON)}
              >
                Zweite Person hinzufügen
              </Button>
            )}
          </fieldset>
        </>
      )}
    </EditForm>
  );
}

interface ContactDraft extends HolidayRegistrationContactEdit {
  readonly key: number;
}

function ContactFields({
  title,
  label,
  prefix,
  contacts,
  max,
  phoneRequired,
  fieldError,
  onChange,
  nextKey,
}: Readonly<{
  title: string;
  /** The name of one entry, „Notfallkontakt“ or „Abholperson“. */
  label: string;
  prefix: string;
  contacts: readonly ContactDraft[];
  max: number;
  phoneRequired: boolean;
  fieldError: (name: string) => string | undefined;
  onChange: (contacts: readonly ContactDraft[]) => void;
  nextKey: () => number;
}>) {
  const set = (index: number, patch: Partial<ContactDraft>) =>
    onChange(
      contacts.map((contact, i) =>
        i === index ? { ...contact, ...patch } : contact,
      ),
    );
  return (
    <fieldset className="space-y-3">
      <legend className="text-sm font-semibold text-gray-900">{title}</legend>
      <p className="text-sm text-gray-700">
        Einen Eintrag entfernen Sie, indem Sie alle seine Felder leeren.
      </p>
      {/* A missing emergency contact names the list, not one entry. */}
      <FieldErrorHint id={`${prefix}-error`} message={fieldError(prefix)} />
      {contacts.map((contact, index) => {
        const path = `${prefix}.${index}`;
        return (
          <div
            key={contact.key}
            className="grid grid-cols-1 gap-3 sm:grid-cols-3"
          >
            <Input
              name={`${path}.name`}
              label="Name"
              required
              maxLength={120}
              value={contact.name}
              error={fieldError(`${path}.name`)}
              onChange={(event) => set(index, { name: event.target.value })}
            />
            <Input
              name={`${path}.phone`}
              type="tel"
              label={phoneRequired ? "Telefon" : "Telefon (freiwillig)"}
              required={phoneRequired}
              maxLength={40}
              value={contact.phone}
              error={fieldError(`${path}.phone`)}
              onChange={(event) => set(index, { phone: event.target.value })}
            />
            <Input
              name={`${path}.relationship`}
              label="Beziehung (freiwillig)"
              maxLength={60}
              value={contact.relationship}
              error={fieldError(`${path}.relationship`)}
              onChange={(event) =>
                set(index, { relationship: event.target.value })
              }
            />
          </div>
        );
      })}
      {contacts.length < max ? (
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={() =>
            onChange([
              ...contacts,
              { key: nextKey(), name: "", phone: "", relationship: "" },
            ])
          }
        >
          {label} hinzufügen
        </Button>
      ) : null}
    </fieldset>
  );
}

/** The emergency contacts and the persons allowed to collect the children. */
export function ContactsEditor(props: HolidayRegistrationEditorProps) {
  const counter = useRef(0);
  const nextKey = () => ++counter.current;
  const drafts = (kind: "emergency" | "pickup") =>
    props.registration.contacts
      .filter((contact) => contact.kind === kind)
      .map((contact) => ({
        key: nextKey(),
        name: contact.name,
        phone: contact.phone,
        relationship: contact.relationship,
      }));
  const [emergency, setEmergency] = useState<readonly ContactDraft[]>(() =>
    drafts("emergency"),
  );
  const [pickup, setPickup] = useState<readonly ContactDraft[]>(() =>
    drafts("pickup"),
  );
  // An entry with every field empty is removed. The form keeps the
  // remaining entries, so a refused field lands at its row.
  const filled = (contacts: readonly ContactDraft[]) =>
    contacts.filter(
      (contact) =>
        contact.name.trim() !== "" ||
        contact.phone.trim() !== "" ||
        contact.relationship.trim() !== "",
    );
  const plain = (contacts: readonly ContactDraft[]) =>
    contacts.map(({ name, phone, relationship }) => ({
      name,
      phone,
      relationship,
    }));
  return (
    <EditForm
      {...props}
      edit={() => {
        const keptEmergency = filled(emergency);
        const keptPickup = filled(pickup);
        setEmergency(keptEmergency);
        setPickup(keptPickup);
        return {
          contacts: {
            emergency: plain(keptEmergency),
            pickup: plain(keptPickup),
          },
        };
      }}
    >
      {(fieldError) => (
        <>
          <ContactFields
            title="Notfallkontakte"
            label="Notfallkontakt"
            prefix="emergency_contacts"
            contacts={emergency}
            max={MAX_EMERGENCY_CONTACTS}
            phoneRequired
            fieldError={fieldError}
            onChange={setEmergency}
            nextKey={nextKey}
          />
          <ContactFields
            title="Abholberechtigte"
            label="Abholperson"
            prefix="pickup_persons"
            contacts={pickup}
            max={MAX_PICKUP_PERSONS}
            phoneRequired={false}
            fieldError={fieldError}
            onChange={setPickup}
            nextKey={nextKey}
          />
        </>
      )}
    </EditForm>
  );
}

function Questions({
  questions,
  answers,
  context,
  prefix,
  fieldError,
  onChange,
}: Readonly<{
  questions: readonly HolidayFormField[];
  answers: Readonly<Record<string, unknown>>;
  context: ConditionContext;
  prefix: string;
  fieldError: (name: string) => string | undefined;
  onChange: (answers: Record<string, unknown>) => void;
}>) {
  const visible = questions.filter((field) => isFieldVisible(field, context));
  if (visible.length === 0) return null;
  const set = (key: string, value: unknown) =>
    onChange({ ...answers, [key]: value });
  return (
    <div className="space-y-4">
      {visible.map((field) => {
        const name = `${prefix}custom_data.${field.key}`;
        return (
          <CustomFieldInput
            key={field.key}
            field={field}
            inputName={name}
            value={answers[field.key]}
            onChange={(value) => set(field.key, value)}
            error={fieldError(name)}
            tr={germanText}
            renderStructured={() =>
              field.type === "date" ? (
                // The native date field of the kit Input: the calendar of
                // the date picker would double the page's bundle growth
                // for a rarely used question type.
                <Input
                  type="date"
                  name={name}
                  label={field.label}
                  required={field.required}
                  value={
                    typeof answers[field.key] === "string"
                      ? (answers[field.key] as string)
                      : ""
                  }
                  error={fieldError(name)}
                  onChange={(event) => set(field.key, event.target.value)}
                />
              ) : null
            }
          />
        );
      })}
    </div>
  );
}

/** The family's answers to the questions without health information. */
export function FamilyAnswersEditor(props: HolidayRegistrationEditorProps) {
  const { registration } = props;
  const fields = registration.form.fields;
  const [answers, setAnswers] = useState<Record<string, unknown>>({
    ...registration.answers,
  });
  return (
    <EditForm {...props} edit={() => ({ answers })}>
      {(fieldError) => (
        <Questions
          questions={editableQuestions(fields, false)}
          answers={answers}
          context={{
            guardianAnswers: answers,
            fieldsByKey: buildFieldsByKey(fields),
          }}
          prefix=""
          fieldError={fieldError}
          onChange={setAnswers}
        />
      )}
    </EditForm>
  );
}

/** One child: names, date of birth, school and its answers. */
export function ChildEditor(
  props: HolidayRegistrationEditorProps &
    Readonly<{
      child: HolidayRegistrationChild;
      schools: readonly TraegerSchool[];
    }>,
) {
  const { registration, child, schools } = props;
  const index = registration.children.findIndex(
    (entry) => entry.id === child.id,
  );
  const prefix = `children.${index}`;
  const fields = registration.form.fields;
  const [draft, setDraft] = useState({
    firstName: child.firstName,
    lastName: child.lastName,
    birthDate: child.birthDate,
    school: child.homeSchoolId ?? GUEST,
    guestSchoolName: child.guestSchoolName,
  });
  const [answers, setAnswers] = useState<Record<string, unknown>>({
    ...registration.childAnswers[child.id],
  });
  const set = (patch: Partial<typeof draft>) =>
    setDraft((current) => ({ ...current, ...patch }));
  const guest = draft.school === GUEST;
  return (
    <EditForm
      {...props}
      edit={() => ({
        child: {
          id: child.id,
          firstName: draft.firstName,
          lastName: draft.lastName,
          birthDate: draft.birthDate,
          homeSchoolId: guest ? null : draft.school,
          guestSchoolName: guest ? draft.guestSchoolName : "",
          answers,
        },
      })}
    >
      {(fieldError) => (
        <>
          <p className="text-sm text-gray-700">
            Die Wochen ändern Sie mit „Umbuchen“. Gesundheitsangaben lassen sich
            hier nicht ändern.
          </p>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Input
              name={`${prefix}.first_name`}
              label="Vorname"
              required
              maxLength={100}
              value={draft.firstName}
              error={fieldError(`${prefix}.first_name`)}
              onChange={(event) => set({ firstName: event.target.value })}
            />
            <Input
              name={`${prefix}.last_name`}
              label="Nachname"
              required
              maxLength={100}
              value={draft.lastName}
              error={fieldError(`${prefix}.last_name`)}
              onChange={(event) => set({ lastName: event.target.value })}
            />
          </div>
          <div className="space-y-2">
            <FieldLabel id={`${prefix}-birth-date-label`} required>
              Geburtsdatum
            </FieldLabel>
            <DateOfBirthPicker
              value={draft.birthDate}
              onChange={(birthDate) => set({ birthDate })}
              error={fieldError(`${prefix}.birth_date`)}
              tr={germanText}
              name={`${prefix}.birth_date`}
              errorId={`${prefix}-birth-date-error`}
            />
          </div>
          <div className="space-y-2">
            <FieldLabel id={`${prefix}-school-label`} required>
              Schule
            </FieldLabel>
            <CustomSelect
              name={`${prefix}.home_school_id`}
              ariaLabelledBy={`${prefix}-school-label`}
              value={draft.school}
              placeholder="Schule wählen"
              invalid={Boolean(fieldError(`${prefix}.home_school_id`))}
              ariaDescribedBy={`${prefix}-school-error`}
              options={[
                ...schools.map((school) => ({
                  value: school.id,
                  label: school.name,
                })),
                { value: GUEST, label: "Andere Schule (Feriengast)" },
              ]}
              onChange={(school) => set({ school })}
            />
            <FieldErrorHint
              id={`${prefix}-school-error`}
              message={fieldError(`${prefix}.home_school_id`)}
            />
            {guest ? (
              <Input
                name={`${prefix}.guest_school_name`}
                label="Name der eigenen Schule (freiwillig)"
                maxLength={120}
                value={draft.guestSchoolName}
                error={fieldError(`${prefix}.guest_school_name`)}
                onChange={(event) =>
                  set({ guestSchoolName: event.target.value })
                }
              />
            ) : null}
          </div>
          <Questions
            questions={editableQuestions(fields, true)}
            answers={answers}
            context={{
              guardianAnswers: { ...registration.answers },
              childAnswers: answers,
              fieldsByKey: buildFieldsByKey(fields),
            }}
            prefix={`${prefix}.`}
            fieldError={fieldError}
            onChange={setAnswers}
          />
        </>
      )}
    </EditForm>
  );
}
