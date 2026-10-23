"use client";

import { Plus } from "lucide-react";
import type { KeyboardEvent, RefObject } from "react";
import { useId, useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { Radio } from "~/components/ui/radio";
import { useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { staffService, type ExternalAccess, type Staff } from "~/lib/staff-api";

const logger = createLogger({ component: "ExternalCaregiverEntry" });

/** Control names equal the wire fields, so server field errors find them. */
export const EXTERNAL_FIELDS = {
  firstName: "first_name",
  lastName: "last_name",
  email: "email",
  organization: "organization",
  access: "access",
} as const;

/** Shown above the fields wherever an external person is recorded. */
export const EXTERNAL_CAREGIVER_HINT =
  "Zum Beispiel jemand von der Musikschule oder vom Sportverein. Sie können die Person sofort einplanen.";

/** Shown at the email field while the chosen access sends an invitation. */
const EXTERNAL_INVITATION_HINT =
  "Die Person bekommt eine Einladung per E-Mail. Damit meldet sie sich selbst bei moto an.";

/**
 * The three access levels of ADR 0091, each with one line on what the person
 * can do. The order is the order on screen.
 */
const EXTERNAL_ACCESS_OPTIONS: Readonly<
  Record<
    ExternalAccess,
    { readonly label: string; readonly description: string }
  >
> = {
  none: {
    label: "Kein Zugang",
    description: "Nur für die Planung. Eine OGS-Kraft startet die Termine.",
  },
  view: {
    label: "Nur ansehen",
    description:
      "Sieht Kinder, Räume und AGs. Keine Elterndaten, keine Änderungen.",
  },
  supervise: {
    label: "Eigene Termine betreuen",
    description:
      "Startet und beendet eigene Termine. Checkt Kinder ein und aus.",
  },
};

/**
 * The levels this session may grant. „Nur ansehen“ needs `users:manage`
 * (`canGrantExternalViewAccess`); without it the backend refuses the role.
 */
function grantableExternalAccess(
  canGrantView: boolean,
): readonly ExternalAccess[] {
  return canGrantView ? ["none", "view", "supervise"] : ["none", "supervise"];
}

// Deliberately loose: the backend decides; this only catches typing slips.
const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

/** Email check shared by the entry and the invitation dialog. */
export function emailFieldHint(email: string): string | null {
  const trimmed = email.trim();
  if (!trimmed) return "Bitte E-Mail-Adresse eingeben.";
  if (!EMAIL_PATTERN.test(trimmed))
    return "Bitte eine gültige E-Mail-Adresse eingeben.";
  return null;
}

interface Values {
  readonly firstName: string;
  readonly lastName: string;
  readonly email: string;
  readonly access: ExternalAccess;
}

function validate(
  values: Values,
): { message: string; fields: Record<string, string> } | null {
  const fields: Record<string, string> = {};
  if (!values.firstName.trim())
    fields[EXTERNAL_FIELDS.firstName] = "Bitte Vornamen eingeben.";
  if (!values.lastName.trim())
    fields[EXTERNAL_FIELDS.lastName] = "Bitte Nachnamen eingeben.";
  // Kein Zugang needs no address; the field is hidden then.
  const emailHint =
    values.access === "none" ? null : emailFieldHint(values.email);
  if (emailHint) fields[EXTERNAL_FIELDS.email] = emailHint;
  const missingName =
    EXTERNAL_FIELDS.firstName in fields || EXTERNAL_FIELDS.lastName in fields;
  if (missingName && emailHint)
    return { message: "Bitte füllen Sie die markierten Felder aus.", fields };
  if (missingName)
    return { message: "Bitte geben Sie Vor- und Nachnamen ein.", fields };
  if (emailHint)
    return { message: "Bitte prüfen Sie die E-Mail-Adresse.", fields };
  return null;
}

/**
 * State and submit of the external caregiver fields (#3823, ADR 0091). The
 * inline entry and the dialog of the Personalverwaltung share it. A submit
 * always records a new person: a name says too little to pick an existing
 * one, and an existing person without account is invited only through
 * „Konto einladen“ in the Personalverwaltung. The access starts at „Kein
 * Zugang“; the other two levels invite by email. The backend refuses an
 * address that already signs in at the school, at the email field.
 */
export function useExternalCaregiverForm({
  onAdded,
  containerRef,
  canGrantView = false,
}: {
  readonly onAdded: (staff: Staff) => void;
  readonly containerRef?: RefObject<HTMLElement | null>;
  /** Offer „Nur ansehen“ (`canGrantExternalViewAccess`). */
  readonly canGrantView?: boolean;
}) {
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [email, setEmail] = useState("");
  const [organization, setOrganization] = useState("");
  const [access, setAccess] = useState<ExternalAccess>("none");
  const [isSaving, setIsSaving] = useState(false);
  const formErrors = useApiFormError(containerRef);

  function reset() {
    setFirstName("");
    setLastName("");
    setEmail("");
    setOrganization("");
    setAccess("none");
    formErrors.clear();
  }

  /** Resolves true once the person is recorded (and invited, if chosen). */
  async function submit(): Promise<boolean> {
    if (isSaving) return false;
    const invalid = validate({ firstName, lastName, email, access });
    if (invalid) {
      formErrors.invalid(invalid.message, invalid.fields);
      return false;
    }
    formErrors.clear();
    setIsSaving(true);
    try {
      const staff = await staffService.createExternal({
        firstName: firstName.trim(),
        lastName: lastName.trim(),
        organization,
        access,
        ...(access === "none" ? {} : { email }),
      });
      onAdded(staff);
      reset();
      return true;
    } catch (cause) {
      logger.error("external_caregiver_create_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void formErrors.show(cause, {
        object: "das Eintragen der externen Person",
        retry: () => void submit(),
      });
      return false;
    } finally {
      setIsSaving(false);
    }
  }

  return {
    values: { firstName, lastName, email, organization, access },
    accessOptions: grantableExternalAccess(canGrantView),
    setFirstName,
    setLastName,
    setEmail,
    setOrganization,
    setAccess,
    isSaving,
    error: formErrors.error,
    fieldError: formErrors.fieldError,
    submit,
    reset,
  };
}

type ExternalCaregiverForm = ReturnType<typeof useExternalCaregiverForm>;

/**
 * The access choice of ADR 0091 as radio tiles: each level with the one line
 * that says what the person can do. A single level (the invitation without
 * `users:manage`) is stated as text, not offered as a one-option choice.
 * The radios carry the wire field name, so a server field error finds them.
 */
export function ExternalAccessChoice<T extends ExternalAccess>({
  options,
  value,
  onChange,
  error,
  onKeyDown,
}: {
  readonly options: readonly T[];
  readonly value: T;
  readonly onChange: (next: T) => void;
  readonly error?: string;
  /** Enter and Escape of the inline entry, which renders no form. */
  readonly onKeyDown?: (event: KeyboardEvent<HTMLInputElement>) => void;
}) {
  const uid = useId();
  const errorId = `${uid}-access-error`;
  const only = options.length === 1 ? options[0] : undefined;
  if (only !== undefined) {
    const option = EXTERNAL_ACCESS_OPTIONS[only];
    return (
      <div className="text-sm">
        <p className="font-medium text-gray-700">Zugang</p>
        <p className="mt-1 text-gray-900">{option.label}</p>
        <p className="text-gray-600">{option.description}</p>
        <FieldErrorHint id={errorId} message={error} />
      </div>
    );
  }
  return (
    <fieldset className="min-w-0">
      <legend className="mb-1 text-sm font-medium text-gray-700">Zugang</legend>
      <div className="grid gap-2">
        {options.map((option) => {
          const { label, description } = EXTERNAL_ACCESS_OPTIONS[option];
          const id = `${uid}-access-${option}`;
          return (
            <ChoiceTile
              key={option}
              selected={value === option}
              className="items-start"
            >
              <Radio
                id={id}
                name={EXTERNAL_FIELDS.access}
                value={option}
                checked={value === option}
                onChange={() => onChange(option)}
                onKeyDown={onKeyDown}
                aria-invalid={error ? true : undefined}
                aria-describedby={error ? errorId : undefined}
                className="mt-0.5"
              />
              <span className="min-w-0">
                <span className="block text-gray-900">{label}</span>
                <span className="block font-normal text-gray-600">
                  {description}
                </span>
              </span>
            </ChoiceTile>
          );
        })}
      </div>
      <FieldErrorHint id={errorId} message={error} />
    </fieldset>
  );
}

/**
 * Vorname, Nachname, Organisation, the access choice and, for an access
 * that invites, the E-Mail-Adresse with the invitation hint.
 */
export function ExternalCaregiverFields({
  form,
  onKeyDown,
  autoFocus = false,
}: {
  readonly form: ExternalCaregiverForm;
  readonly onKeyDown?: (event: KeyboardEvent<HTMLInputElement>) => void;
  readonly autoFocus?: boolean;
}) {
  // Two entries can be mounted at once (dialog over a page), so the ids are
  // unique while the names stay the wire field names.
  const uid = useId();
  const invites = form.values.access !== "none";
  return (
    <>
      <div className="grid gap-3 sm:grid-cols-2">
        <Input
          id={`${uid}-first-name`}
          name={EXTERNAL_FIELDS.firstName}
          label="Vorname"
          controlSize="compact"
          value={form.values.firstName}
          onChange={(event) => form.setFirstName(event.target.value)}
          onKeyDown={onKeyDown}
          error={form.fieldError(EXTERNAL_FIELDS.firstName)}
          autoComplete="off"
          maxLength={100}
          autoFocus={autoFocus}
        />
        <Input
          id={`${uid}-last-name`}
          name={EXTERNAL_FIELDS.lastName}
          label="Nachname"
          controlSize="compact"
          value={form.values.lastName}
          onChange={(event) => form.setLastName(event.target.value)}
          onKeyDown={onKeyDown}
          error={form.fieldError(EXTERNAL_FIELDS.lastName)}
          autoComplete="off"
          maxLength={100}
        />
      </div>
      <Input
        id={`${uid}-organization`}
        name={EXTERNAL_FIELDS.organization}
        label="Organisation (freiwillig)"
        controlSize="compact"
        value={form.values.organization}
        onChange={(event) => form.setOrganization(event.target.value)}
        onKeyDown={onKeyDown}
        error={form.fieldError(EXTERNAL_FIELDS.organization)}
        placeholder="zum Beispiel Musikschule"
        autoComplete="off"
        maxLength={150}
      />
      <ExternalAccessChoice
        options={form.accessOptions}
        value={form.values.access}
        onChange={form.setAccess}
        error={form.fieldError(EXTERNAL_FIELDS.access)}
        onKeyDown={onKeyDown}
      />
      {invites ? (
        <div className="space-y-1">
          <Input
            id={`${uid}-email`}
            name={EXTERNAL_FIELDS.email}
            type="email"
            label="E-Mail-Adresse"
            controlSize="compact"
            value={form.values.email}
            onChange={(event) => form.setEmail(event.target.value)}
            onKeyDown={onKeyDown}
            error={form.fieldError(EXTERNAL_FIELDS.email)}
            // No native `required`: the entry may sit inside another form,
            // which must still submit while this panel is open and empty.
            aria-required="true"
            autoComplete="off"
            maxLength={254}
          />
          <p className="text-sm text-gray-600">{EXTERNAL_INVITATION_HINT}</p>
        </div>
      ) : null}
    </>
  );
}

interface ExternalCaregiverEntryProps {
  /** Called with the new entry, with or without invitation. */
  readonly onAdded: (staff: Staff) => void;
  readonly disabled?: boolean;
  /** Offer „Nur ansehen“ (`canGrantExternalViewAccess`). */
  readonly canGrantView?: boolean;
}

/**
 * Records an external caregiver (#3823, ADR 0091) right where a supervision
 * is planned, with the chosen access. It renders no `<form>`: the caller's
 * dialog may be a form already, so Enter is handled here instead of
 * submitting the dialog.
 */
export function ExternalCaregiverEntry({
  onAdded,
  disabled = false,
  canGrantView = false,
}: ExternalCaregiverEntryProps) {
  const [isOpen, setIsOpen] = useState(false);
  const panelRef = useRef<HTMLDivElement>(null);
  const form = useExternalCaregiverForm({
    onAdded,
    containerRef: panelRef,
    canGrantView,
  });

  function close() {
    form.reset();
    setIsOpen(false);
  }

  async function submitAndClose() {
    if (await form.submit()) setIsOpen(false);
  }

  function handleKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "Escape") {
      // Close only this panel. The dialog listens for Escape on the same
      // document node, so plain stopPropagation would not keep it open.
      event.preventDefault();
      event.nativeEvent.stopImmediatePropagation();
      close();
      return;
    }
    if (event.key !== "Enter") return;
    event.preventDefault();
    void submitAndClose();
  }

  if (!isOpen) {
    return (
      <Button
        type="button"
        variant="ghost"
        size="compact"
        onClick={() => setIsOpen(true)}
        disabled={disabled}
      >
        <Plus className="h-4 w-4" aria-hidden="true" />
        Externe Person eintragen
      </Button>
    );
  }

  return (
    <div
      ref={panelRef}
      role="group"
      aria-label="Externe Person eintragen"
      className="space-y-3 rounded-lg border border-gray-200 bg-gray-50 p-3"
    >
      <p className="text-sm text-gray-600">{EXTERNAL_CAREGIVER_HINT}</p>
      <FormErrorAlert message={form.error} />
      <ExternalCaregiverFields
        form={form}
        onKeyDown={handleKeyDown}
        autoFocus
      />
      <div className="flex justify-end gap-2">
        {/* Not "Abbrechen": the dialog below already has one for itself. */}
        <Button
          type="button"
          variant="ghost"
          size="md"
          onClick={close}
          disabled={form.isSaving}
        >
          Nicht eintragen
        </Button>
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={() => void submitAndClose()}
          isLoading={form.isSaving}
          loadingText="Wird eingetragen ..."
        >
          Eintragen
        </Button>
      </div>
    </div>
  );
}
