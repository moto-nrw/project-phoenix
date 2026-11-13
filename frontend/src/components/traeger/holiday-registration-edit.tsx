"use client";

// Die Angaben einer Ferienanmeldung ändern (#5629): Eltern, ein Kind,
// Notfallkontakte und Abholung, die allgemeinen Antworten. Jede Karte der
// Einzelansicht wechselt an Ort und Stelle in diesen Bearbeiten-Zustand.
// Die Felder sind die Bausteine des öffentlichen Formulars; die Feldpfade
// sind die des Backends, so landet ein Fehler an seinem Feld. Wochen ändert
// nur „Umbuchen“, Gesundheitsangaben ändern sich hier nicht.

import {
  type FormEvent,
  type ReactNode,
  lazy,
  Suspense,
  useRef,
  useState,
} from "react";
import {
  DateOfBirthPicker,
  type DateOfBirthTranslator,
} from "~/components/enrollment/date-of-birth-picker";
import { CustomDateField } from "~/components/enrollment/custom-date-field";
import {
  type GuardianAddressDraft,
  GuardianAddressFields,
} from "~/components/holiday-registration/holiday-guardian-address-fields";
import { HolidayQuestionFields } from "~/components/holiday-registration/holiday-question-fields";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FieldLabel } from "~/components/ui/field-label";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { useApiFormError } from "~/contexts/ToastContext";
import {
  buildFieldsByKey,
  type ConditionContext,
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
  EDIT_TITLE,
} from "./holiday-registration-edit-fields";

const logger = createLogger({ component: "HolidayRegistrationEdit" });

// The calendar of the kit date picker loads only when an open edit form
// asks a date question; the German texts are its defaults.
const LazyDatePicker = lazy(() =>
  import("~/components/ui/date-picker").then((module) => ({
    default: module.ISODatePicker,
  })),
);

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
  "fields.choose": "Bitte wählen",
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
  description,
  edit,
  children,
}: HolidayRegistrationEditorProps &
  Readonly<{
    /** Der Bereich, den dieser Bearbeiten-Zustand ändert. */
    description: string;
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
    <form ref={formRef} noValidate onSubmit={(event) => void submit(event)}>
      <SectionCard
        title={EDIT_TITLE}
        description={description}
        footer={
          <EditActions
            onCancel={() => {
              errors.clear();
              onCancel();
            }}
            saving={saving}
            saveLabel="Änderung speichern"
          />
        }
      >
        <div className="space-y-4">
          <FormErrorAlert message={errors.error} />
          <p className="text-sm text-gray-700">{EDIT_HINT}</p>
          {children(fieldError)}
        </div>
      </SectionCard>
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
      controlSize="touch"
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
const ADDRESS_LABELS = {
  street: "Straße und Hausnummer",
  postalCode: "PLZ",
  city: "Ort",
} as const;

type AddressDraft = GuardianAddressDraft;

/**
 * The postal address of the parent (#5625), when the form version of the
 * registration asks for it: the fields of the public form, and here the
 * notice that an issued invoice keeps its address.
 */
function AddressSection({
  address,
  required,
  invoiced,
  fieldError,
  onChange,
}: Readonly<{
  address: AddressDraft;
  required: boolean;
  /** An issued invoice keeps the address it was issued with. */
  invoiced: boolean;
  fieldError: (name: string) => string | undefined;
  onChange: (address: AddressDraft) => void;
}>) {
  return (
    <fieldset className="space-y-3">
      <legend className="text-sm font-semibold text-gray-900">
        {required ? "Anschrift" : "Anschrift (freiwillig)"}
      </legend>
      {required ? null : (
        <p className="text-sm text-gray-700">
          Mit Anschrift sind Straße, PLZ und Ort nötig.
        </p>
      )}
      {invoiced ? (
        <p className="text-sm text-gray-700">
          Für diese Anmeldung gibt es schon eine Rechnung. Die Rechnung behält
          die bisherige Anschrift.
        </p>
      ) : null}
      <GuardianAddressFields
        controlSize="compact"
        address={address}
        required={required}
        labels={ADDRESS_LABELS}
        fieldError={fieldError}
        onChange={onChange}
      />
    </fieldset>
  );
}

/** The parent who registered with the postal address, and the optional
 * second guardian. */
export function GuardiansEditor(
  props: HolidayRegistrationEditorProps & Readonly<{ invoiced?: boolean }>,
) {
  const { registration, invoiced = false } = props;
  const addressRequirement = registration.form.addressRequirement ?? "off";
  const [address, setAddress] = useState<AddressDraft>(
    registration.guardianAddress ?? { street: "", postalCode: "", city: "" },
  );
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
      description="Eltern"
      edit={() => ({
        guardians: {
          guardian,
          // An empty address is no address, as in the public form.
          address:
            addressRequirement === "off" ||
            (address.street.trim() === "" &&
              address.postalCode.trim() === "" &&
              address.city.trim() === "")
              ? null
              : address,
          secondGuardian: second,
        },
      })}
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
          {addressRequirement === "off" ? null : (
            <AddressSection
              address={address}
              required={addressRequirement === "required"}
              invoiced={invoiced}
              fieldError={fieldError}
              onChange={setAddress}
            />
          )}
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
              controlSize="touch"
              name={`${path}.name`}
              label="Name"
              required
              maxLength={120}
              value={contact.name}
              error={fieldError(`${path}.name`)}
              onChange={(event) => set(index, { name: event.target.value })}
            />
            <Input
              controlSize="touch"
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
              controlSize="touch"
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
      description="Notfallkontakte und Abholung"
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
          <p className="text-sm text-gray-700">
            Einen Eintrag entfernen Sie, indem Sie alle seine Felder leeren.
          </p>
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

/** The questions of one scope with the fields of the public form. */
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
  return (
    <HolidayQuestionFields
      fields={questions}
      answers={answers}
      context={context}
      prefix={prefix}
      fieldError={fieldError}
      onChange={(key, value) => onChange({ ...answers, [key]: value })}
      tr={germanText}
      renderDate={({ name, field, value, error, onChange: change }) => (
        <CustomDateField
          field={field}
          inputName={name}
          value={value}
          error={error}
          onChange={change}
          controlId={name.replaceAll(".", "-")}
          renderControl={(control) => (
            <Suspense fallback={null}>
              <LazyDatePicker
                id={control.id}
                name={control.name}
                controlSize="md"
                value={control.value}
                onChange={control.onChange}
                invalid={control.invalid}
                ariaDescribedBy={control.ariaDescribedBy}
                calendarLayout="popover"
                // A question can ask for any date, from a birthday to an
                // appointment: the year stays selectable, without bounds.
                monthYearNavigation
              />
            </Suspense>
          )}
        />
      )}
    />
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
    <EditForm
      {...props}
      description="Allgemeine Angaben"
      edit={() => ({ answers })}
    >
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
      description={`${props.child.firstName} ${props.child.lastName}`}
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
              controlSize="touch"
              name={`${prefix}.first_name`}
              label="Vorname"
              required
              maxLength={100}
              value={draft.firstName}
              error={fieldError(`${prefix}.first_name`)}
              onChange={(event) => set({ firstName: event.target.value })}
            />
            <Input
              controlSize="touch"
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
                ...(child.homeSchoolId !== null &&
                !schools.some((school) => school.id === child.homeSchoolId)
                  ? [
                      {
                        value: child.homeSchoolId,
                        label: "Bisherige Schule (gelöscht)",
                      },
                    ]
                  : []),
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
                controlSize="touch"
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
