"use client";

import { Plus } from "lucide-react";
import type { KeyboardEvent, RefObject } from "react";
import { useId, useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { staffService, type Staff } from "~/lib/staff-api";

const logger = createLogger({ component: "ExternalCaregiverEntry" });

/** Control names equal the wire fields, so server field errors find them. */
export const EXTERNAL_FIELDS = {
  firstName: "first_name",
  lastName: "last_name",
  email: "email",
  organization: "organization",
} as const;

/** Shown above the fields wherever an external person is recorded. */
export const EXTERNAL_CAREGIVER_HINT =
  "Zum Beispiel die AG-Leitung der Musikschule. Die Person bekommt eine Einladung per E-Mail. Damit meldet sie sich selbst bei moto an. Sie können sie sofort einplanen.";

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
}

function validate(
  values: Values,
): { message: string; fields: Record<string, string> } | null {
  const fields: Record<string, string> = {};
  if (!values.firstName.trim())
    fields[EXTERNAL_FIELDS.firstName] = "Bitte Vornamen eingeben.";
  if (!values.lastName.trim())
    fields[EXTERNAL_FIELDS.lastName] = "Bitte Nachnamen eingeben.";
  const emailHint = emailFieldHint(values.email);
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
 * State and submit of the external caregiver fields (#3823, #4573). The
 * inline entry and the dialog of the Personalverwaltung share it. A submit
 * always records a new person: a name says too little to pick an existing
 * one, and an existing person without account is invited only through
 * „Konto einladen“ in the Personalverwaltung. The backend refuses an address
 * that already signs in at the school, at the email field.
 */
export function useExternalCaregiverForm({
  onAdded,
  containerRef,
}: {
  readonly onAdded: (staff: Staff) => void;
  readonly containerRef?: RefObject<HTMLElement | null>;
}) {
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [email, setEmail] = useState("");
  const [organization, setOrganization] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const formErrors = useApiFormError(containerRef);

  function reset() {
    setFirstName("");
    setLastName("");
    setEmail("");
    setOrganization("");
    formErrors.clear();
  }

  /** Resolves true once the person is recorded and invited. */
  async function submit(): Promise<boolean> {
    if (isSaving) return false;
    const invalid = validate({ firstName, lastName, email });
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
        email,
        organization,
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
    values: { firstName, lastName, email, organization },
    setFirstName,
    setLastName,
    setEmail,
    setOrganization,
    isSaving,
    error: formErrors.error,
    fieldError: formErrors.fieldError,
    submit,
    reset,
  };
}

type ExternalCaregiverForm = ReturnType<typeof useExternalCaregiverForm>;

/** Vorname, Nachname, E-Mail-Adresse and Organisation as kit inputs. */
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
        id={`${uid}-email`}
        name={EXTERNAL_FIELDS.email}
        type="email"
        label="E-Mail-Adresse"
        controlSize="compact"
        value={form.values.email}
        onChange={(event) => form.setEmail(event.target.value)}
        onKeyDown={onKeyDown}
        error={form.fieldError(EXTERNAL_FIELDS.email)}
        // No native `required`: the entry may sit inside another form, which
        // must still submit while this panel is open and empty.
        aria-required="true"
        autoComplete="off"
        maxLength={254}
      />
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
    </>
  );
}

interface ExternalCaregiverEntryProps {
  /** Called with the new, invited entry. */
  readonly onAdded: (staff: Staff) => void;
  readonly disabled?: boolean;
}

/**
 * Records and invites an external caregiver (#3823, #4573) right where a
 * supervision is planned. It renders no `<form>`: the caller's dialog may be
 * a form already, so Enter is handled here instead of submitting the dialog.
 */
export function ExternalCaregiverEntry({
  onAdded,
  disabled = false,
}: ExternalCaregiverEntryProps) {
  const [isOpen, setIsOpen] = useState(false);
  const panelRef = useRef<HTMLDivElement>(null);
  const form = useExternalCaregiverForm({ onAdded, containerRef: panelRef });

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
