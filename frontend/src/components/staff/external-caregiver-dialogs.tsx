"use client";

import type { KeyboardEvent } from "react";
import { useEffect, useId, useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { staffService, type Staff } from "~/lib/staff-api";

import {
  EXTERNAL_CAREGIVER_HINT,
  EXTERNAL_FIELDS,
  ExternalCaregiverFields,
  emailFieldHint,
  showExternalFailure,
  useExternalCaregiverForm,
  type ExistingCaregiver,
  type ExternalCaregiverResult,
} from "./external-caregiver-entry";

const logger = createLogger({ component: "ExternalCaregiverDialogs" });

/**
 * „Externe Person eintragen“ in the Personalverwaltung (#4573): the fields of
 * the inline entry in a dialog. A name match without account invites that
 * person instead of recording a second one.
 */
export function ExternalCaregiverCreateModal({
  isOpen,
  onClose,
  existing,
  onSaved,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly existing: readonly ExistingCaregiver[];
  readonly onSaved: (result: ExternalCaregiverResult) => void;
}) {
  const bodyRef = useRef<HTMLDivElement>(null);
  const form = useExternalCaregiverForm({
    existing,
    onAdded: onSaved,
    containerRef: bodyRef,
  });
  // A reopened dialog starts empty: a successful save resets the form
  // itself, closing without saving resets it here.
  function close() {
    form.reset();
    onClose();
  }

  async function save() {
    if (await form.submit()) onClose();
  }

  function handleKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key !== "Enter") return;
    event.preventDefault();
    void save();
  }

  return (
    <FormModal
      isOpen={isOpen}
      onClose={close}
      title="Externe Person eintragen"
      size="md"
      closeDisabled={form.isSaving}
      isBackdropDismissDisabled
      error={form.error}
      footer={
        <>
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={close}
            disabled={form.isSaving}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => void save()}
            isLoading={form.isSaving}
            loadingText="Wird eingetragen ..."
          >
            Eintragen
          </Button>
        </>
      }
    >
      <div ref={bodyRef} className="space-y-3">
        <p className="text-sm text-gray-600">{EXTERNAL_CAREGIVER_HINT}</p>
        <ExternalCaregiverFields
          form={form}
          onKeyDown={handleKeyDown}
          autoFocus
        />
      </div>
    </FormModal>
  );
}

/** The person the invitation dialog is for. */
export interface ExternalInvitationTarget {
  readonly id: string;
  readonly name: string;
  /** An open invitation is replaced; the wording says so. */
  readonly invitationPending: boolean;
}

/**
 * „Konto einladen“ for an external person without account (#4573). With an
 * open invitation the same dialog sends a new one, for example to a
 * corrected address.
 */
export function ExternalInvitationModal({
  target,
  onClose,
  onInvited,
}: {
  readonly target: ExternalInvitationTarget | null;
  readonly onClose: () => void;
  readonly onInvited: (staff: Staff) => void;
}) {
  const [email, setEmail] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const bodyRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(bodyRef);
  const { clear } = formErrors;
  const inputId = useId();
  const targetId = target?.id;

  useEffect(() => {
    setEmail("");
    clear();
  }, [targetId, clear]);

  async function invite() {
    if (!target || isSaving) return;
    const hint = emailFieldHint(email);
    if (hint) {
      formErrors.invalid("Bitte prüfen Sie die E-Mail-Adresse.", {
        [EXTERNAL_FIELDS.email]: hint,
      });
      return;
    }
    formErrors.clear();
    setIsSaving(true);
    try {
      const staff = await staffService.inviteExternal(target.id, email);
      onInvited(staff);
      onClose();
    } catch (cause) {
      logger.error("external_caregiver_invite_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      await showExternalFailure(formErrors, cause, {
        object: `die Einladung für ${target.name}`,
        retry: () => void invite(),
      });
    } finally {
      setIsSaving(false);
    }
  }

  const pending = target?.invitationPending === true;
  return (
    <FormModal
      isOpen={target !== null}
      onClose={onClose}
      title={pending ? "Einladung erneut senden" : "Konto einladen"}
      size="md"
      closeDisabled={isSaving}
      error={formErrors.error}
      footer={
        <>
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={isSaving}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => void invite()}
            isLoading={isSaving}
            loadingText="Wird gesendet ..."
          >
            {pending ? "Erneut senden" : "Einladen"}
          </Button>
        </>
      }
    >
      <div ref={bodyRef} className="space-y-3">
        <p className="text-sm text-gray-600">
          {target?.name} bekommt eine Einladung per E-Mail. Damit meldet sich
          die Person selbst bei moto an.
          {pending ? " Die alte Einladung gilt dann nicht mehr." : null}
        </p>
        <Input
          id={`${inputId}-email`}
          name={EXTERNAL_FIELDS.email}
          type="email"
          label="E-Mail-Adresse"
          controlSize="compact"
          value={email}
          onChange={(event) => setEmail(event.target.value)}
          onKeyDown={(event) => {
            if (event.key !== "Enter") return;
            event.preventDefault();
            void invite();
          }}
          error={formErrors.fieldError(EXTERNAL_FIELDS.email)}
          aria-required="true"
          autoComplete="off"
          maxLength={254}
          autoFocus
        />
      </div>
    </FormModal>
  );
}
