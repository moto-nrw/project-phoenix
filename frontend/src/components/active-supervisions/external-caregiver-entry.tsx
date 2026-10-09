"use client";

import { Plus } from "lucide-react";
import type { KeyboardEvent } from "react";
import { useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { staffService, type Staff } from "~/lib/staff-api";

const logger = createLogger({ component: "ExternalCaregiverEntry" });

/** A person already in the picker, matched by name before a new entry. */
interface ExistingCaregiver {
  readonly id: string;
  readonly fullName: string;
}

interface ExternalCaregiverEntryProps {
  /** Everyone the picker already offers; a name match selects that entry. */
  readonly existing: readonly ExistingCaregiver[];
  /** Called with the new entry, or with the id of the matching entry. */
  readonly onAdded: (result: ExternalCaregiverResult) => void;
  readonly disabled?: boolean;
}

export type ExternalCaregiverResult =
  | { readonly kind: "created"; readonly staff: Staff }
  | { readonly kind: "existing"; readonly id: string };

const FIRST_NAME = "external-first-name";
const LAST_NAME = "external-last-name";

function normalizeName(value: string): string {
  return value.trim().replace(/\s+/g, " ").toLocaleLowerCase("de");
}

/**
 * Records an external caregiver without a moto account (#3823) right where a
 * supervision is planned. It renders no `<form>`: the caller's dialog may be
 * a form already, so Enter is handled here instead of submitting the dialog.
 */
export function ExternalCaregiverEntry({
  existing,
  onAdded,
  disabled = false,
}: ExternalCaregiverEntryProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [organization, setOrganization] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const panelRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(panelRef);

  function reset() {
    setIsOpen(false);
    setFirstName("");
    setLastName("");
    setOrganization("");
    formErrors.clear();
  }

  async function submitExternal() {
    if (isSaving) return;
    const first = firstName.trim();
    const last = lastName.trim();
    if (!first || !last) {
      formErrors.invalid("Bitte geben Sie Vor- und Nachnamen ein.", {
        ...(first ? {} : { [FIRST_NAME]: "Bitte Vornamen eingeben." }),
        ...(last ? {} : { [LAST_NAME]: "Bitte Nachnamen eingeben." }),
      });
      return;
    }
    const match = existing.find(
      (person) =>
        normalizeName(person.fullName) === normalizeName(`${first} ${last}`),
    );
    if (match) {
      onAdded({ kind: "existing", id: match.id });
      reset();
      return;
    }
    formErrors.clear();
    setIsSaving(true);
    try {
      const staff = await staffService.createExternal({
        firstName: first,
        lastName: last,
        organization,
      });
      onAdded({ kind: "created", staff });
      reset();
    } catch (cause) {
      logger.error("external_caregiver_create_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void formErrors.show(cause, {
        object: "der Eintrag für die externe Person",
        retry: () => void submitExternal(),
      });
    } finally {
      setIsSaving(false);
    }
  }

  function handleKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "Escape") {
      // Close only this panel. The dialog listens for Escape on the same
      // document node, so plain stopPropagation would not keep it open.
      event.preventDefault();
      event.nativeEvent.stopImmediatePropagation();
      reset();
      return;
    }
    if (event.key !== "Enter") return;
    event.preventDefault();
    void submitExternal();
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
      <p className="text-sm text-gray-600">
        Für Personen ohne moto-Konto, zum Beispiel die AG-Leitung der
        Musikschule. Danach können Sie die Person immer wieder auswählen.
      </p>
      <FormErrorAlert message={formErrors.error} />
      <div className="grid gap-3 sm:grid-cols-2">
        <Input
          name={FIRST_NAME}
          label="Vorname"
          controlSize="compact"
          value={firstName}
          onChange={(event) => setFirstName(event.target.value)}
          onKeyDown={handleKeyDown}
          error={formErrors.fieldError(FIRST_NAME)}
          autoComplete="off"
          maxLength={100}
          autoFocus
        />
        <Input
          name={LAST_NAME}
          label="Nachname"
          controlSize="compact"
          value={lastName}
          onChange={(event) => setLastName(event.target.value)}
          onKeyDown={handleKeyDown}
          error={formErrors.fieldError(LAST_NAME)}
          autoComplete="off"
          maxLength={100}
        />
      </div>
      <Input
        name="external-organization"
        label="Organisation (freiwillig)"
        controlSize="compact"
        value={organization}
        onChange={(event) => setOrganization(event.target.value)}
        onKeyDown={handleKeyDown}
        placeholder="zum Beispiel Musikschule"
        autoComplete="off"
        maxLength={150}
      />
      <div className="flex justify-end gap-2">
        {/* Not "Abbrechen": the dialog below already has one for itself. */}
        <Button
          type="button"
          variant="ghost"
          size="md"
          onClick={reset}
          disabled={isSaving}
        >
          Nicht eintragen
        </Button>
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={() => void submitExternal()}
          isLoading={isSaving}
          loadingText="Wird eingetragen ..."
        >
          Eintragen
        </Button>
      </div>
    </div>
  );
}
