"use client";

import { useEffect, useRef, useState } from "react";
import { EditActions } from "~/components/ui/edit-actions";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { saveTraegerContact, type TraegerContact } from "~/lib/traeger-api";

const logger = createLogger({ component: "TraegerContactCard" });

function sameContact(a: TraegerContact, b: TraegerContact): boolean {
  return a.name === b.name && a.phone === b.phone && a.email === b.email;
}

/**
 * The contact person of the Träger (#4653): who parents and schools reach
 * about a letter. Empty fields are left off the letter.
 */
export function TraegerContactCard({
  contact,
  canEdit,
  onSaved,
}: Readonly<{
  contact: TraegerContact;
  canEdit: boolean;
  onSaved: () => Promise<unknown>;
}>) {
  const [draft, setDraft] = useState<TraegerContact>(contact);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { success } = useToast();

  // A refresh after another change on the page brings a new contact
  // object; an open draft stays until it is saved or cancelled.
  const shown = useRef(contact);
  const [stored, setStored] = useState(contact);
  useEffect(() => {
    const before = shown.current;
    shown.current = contact;
    setStored(contact);
    setDraft((current) => (sameContact(current, before) ? contact : current));
  }, [contact]);

  const changed = !sameContact(draft, stored);

  const submit = async () => {
    formErrors.clear();
    setSaving(true);
    const submitted = draft;
    try {
      const saved = await saveTraegerContact(submitted);
      // The form shows what the backend keeps (trimmed, e-mail in lower
      // case), unless the person typed on while it saved.
      shown.current = saved;
      setStored(saved);
      setDraft((current) =>
        sameContact(current, submitted) ? saved : current,
      );
      success("Der Ansprechpartner ist gespeichert.");
      await onSaved();
    } catch (err) {
      logger.error("traeger_contact_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "der Ansprechpartner" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <SectionCard
      title="Ansprechpartner"
      description="An diese Person wenden sich Eltern und Schulen bei Fragen zu einem Brief. Leere Felder stehen nicht auf dem Brief."
    >
      <form
        ref={formRef}
        noValidate
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <FormErrorAlert message={formErrors.error} />
        <div className="grid gap-4 md:grid-cols-3">
          <Input
            id="traeger-contact-name"
            label="Name"
            name="name"
            value={draft.name}
            onChange={(e) => setDraft({ ...draft, name: e.target.value })}
            error={formErrors.fieldError("name")}
            maxLength={120}
            disabled={!canEdit}
          />
          <Input
            id="traeger-contact-phone"
            label="Telefon"
            name="phone"
            type="tel"
            value={draft.phone}
            onChange={(e) => setDraft({ ...draft, phone: e.target.value })}
            error={formErrors.fieldError("phone")}
            maxLength={40}
            disabled={!canEdit}
          />
          <Input
            id="traeger-contact-email"
            label="E-Mail-Adresse"
            name="email"
            type="email"
            value={draft.email}
            onChange={(e) => setDraft({ ...draft, email: e.target.value })}
            error={formErrors.fieldError("email")}
            maxLength={254}
            disabled={!canEdit}
          />
        </div>
        {canEdit && changed ? (
          <EditActions
            onCancel={() => {
              setDraft(stored);
              formErrors.clear();
            }}
            saving={saving}
            saveType="submit"
          />
        ) : null}
      </form>
    </SectionCard>
  );
}
