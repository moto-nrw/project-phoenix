"use client";

// Bestätigungsschreiben eines freigegebenen Laufs erstellen (#4669):
// Zahlungsfrist, Hinweise für die Eltern (Packliste, Krankheitsregel) und
// die Notfallnummern der ausrichtenden OGS. Die Vorschau zeigt das Schreiben
// der ersten Familie; erstellt werden alle Schreiben einmal.

import { Plus, X } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { ISODateInput } from "~/components/ui/date-picker";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { toISODate, todayISO } from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  type BillingEmergencyContact,
  type BillingLetters,
  type BillingLettersInput,
  createBillingLetters,
  previewBillingLetter,
} from "~/lib/traeger-billing-api";

const logger = createLogger({ component: "BillingLettersModal" });

const MAX_CONTACTS = 10;

/** Two weeks after today, the Bissendorf payment term. */
function defaultDueDate(): string {
  const date = new Date();
  date.setDate(date.getDate() + 14);
  return toISODate(date);
}

function ContactRow({
  index,
  contact,
  error,
  onChange,
  onRemove,
}: Readonly<{
  index: number;
  contact: BillingEmergencyContact;
  error: string | null;
  onChange: (contact: BillingEmergencyContact) => void;
  onRemove: () => void;
}>) {
  return (
    <div className="flex flex-col gap-2 sm:flex-row sm:items-start">
      <div className="flex-1">
        <Input
          id={`billing-letters-contact-${index}-name`}
          label="Betreuung"
          controlSize="compact"
          placeholder="Zum Beispiel: OGS Am Lindenhof"
          value={contact.name}
          maxLength={120}
          error={error ?? undefined}
          onChange={(event) =>
            onChange({ ...contact, name: event.target.value })
          }
        />
      </div>
      <div className="flex-1">
        <Input
          id={`billing-letters-contact-${index}-phone`}
          label="Telefon"
          controlSize="compact"
          inputMode="tel"
          placeholder="0170 1234567"
          value={contact.phone}
          maxLength={120}
          onChange={(event) =>
            onChange({ ...contact, phone: event.target.value })
          }
        />
      </div>
      <div className="flex sm:pt-7">
        {/* Icon only next to the row on wide screens; on a phone the row
            stacks, so the button names what it removes. */}
        <Button
          type="button"
          variant="ghost"
          size="compact"
          aria-label={`Notfallnummer ${contact.name || "ohne Namen"} entfernen`}
          onClick={onRemove}
        >
          <X className="h-4 w-4" aria-hidden="true" />
          <span className="sm:hidden">Notfallnummer entfernen</span>
        </Button>
      </div>
    </div>
  );
}

export function BillingLettersModal({
  runId,
  isOpen,
  onClose,
  onCreated,
}: Readonly<{
  runId: string;
  isOpen: boolean;
  onClose: () => void;
  onCreated: (letters: BillingLetters) => void;
}>) {
  const [dueDate, setDueDate] = useState("");
  // The date field keeps the last valid date while the typed one is not;
  // nothing goes out until the field holds a valid date again.
  const [dueDateValid, setDueDateValid] = useState(true);
  const [notes, setNotes] = useState("");
  const [contacts, setContacts] = useState<BillingEmergencyContact[]>([]);
  const [saving, setSaving] = useState(false);
  const [previewing, setPreviewing] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearErrors } = formErrors;
  const { success } = useToast();

  useEffect(() => {
    if (!isOpen) return;
    setDueDate(defaultDueDate());
    setDueDateValid(true);
    setNotes("");
    setContacts([{ name: "", phone: "" }]);
    clearErrors();
  }, [isOpen, clearErrors]);

  // An empty row is no emergency number; a half-filled one is a mistake.
  const filled = contacts.filter(
    (contact) => contact.name.trim() || contact.phone.trim(),
  );
  const incomplete = contacts.findIndex(
    (contact) => Boolean(contact.name.trim()) !== Boolean(contact.phone.trim()),
  );

  const input = (): BillingLettersInput | null => {
    formErrors.clear();
    const fields: Record<string, string> = {};
    if (!dueDate) fields.payment_due_on = "Bitte geben Sie ein Datum an.";
    else if (!dueDateValid)
      fields.payment_due_on =
        "Bitte geben Sie ein gültiges Datum ab heute im Format TT.MM.JJJJ ein.";
    if (incomplete >= 0)
      fields.emergency_contacts =
        "Bitte geben Sie zu jeder Notfallnummer Betreuung und Telefon an.";
    if (Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie Ihre Angaben.", fields);
      return null;
    }
    return {
      paymentDueOn: dueDate,
      notes: notes.trim(),
      emergencyContacts: filled.map((contact) => ({
        name: contact.name.trim(),
        phone: contact.phone.trim(),
      })),
    };
  };

  const preview = async () => {
    const value = input();
    if (!value) return;
    // The tab opens inside the click, before the request; afterwards a popup
    // blocker would refuse it.
    const tab = globalThis.open("", "_blank");
    setPreviewing(true);
    try {
      const file = await previewBillingLetter(runId, value);
      const url = URL.createObjectURL(file.blob);
      if (tab) {
        tab.location.href = url;
        globalThis.setTimeout(() => URL.revokeObjectURL(url), 60_000);
      } else {
        // The browser refused the tab: the preview comes as a download.
        URL.revokeObjectURL(url);
        downloadBlob(file.blob, file.filename);
      }
    } catch (err) {
      tab?.close();
      logger.error("billing_letter_preview_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Vorschau" });
    } finally {
      setPreviewing(false);
    }
  };

  const submit = async () => {
    const value = input();
    if (!value) return;
    setSaving(true);
    try {
      const letters = await createBillingLetters(runId, value);
      success("Die Schreiben sind erstellt.");
      onCreated(letters);
    } catch (err) {
      logger.error("billing_letters_create_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Schreiben" });
    } finally {
      setSaving(false);
    }
  };

  const contactsError = formErrors.fieldError("emergency_contacts");
  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title="Schreiben erstellen"
      size="lg"
      error={formErrors.error}
      footer={
        <div className="flex flex-wrap items-center justify-between gap-2">
          <Button
            type="button"
            variant="outline"
            size="md"
            disabled={previewing || saving}
            onClick={() => void preview()}
          >
            {previewing ? "Vorschau wird erstellt …" : "Vorschau ansehen"}
          </Button>
          <EditActions
            onCancel={onClose}
            onSave={() => formRef.current?.requestSubmit()}
            saving={saving}
            saveLabel="Schreiben erstellen"
          />
        </div>
      }
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
        <p className="text-sm text-gray-700">
          Jede Familie bekommt ein Schreiben mit ihrer Rechnung. Was Sie hier
          eingeben, steht in allen Schreiben. Danach lässt sich nichts mehr
          ändern.
        </p>
        <ISODateInput
          id="payment_due_on"
          label="Zahlen bis"
          value={dueDate}
          min={todayISO()}
          minDateError="Das Datum darf nicht in der Vergangenheit liegen."
          error={formErrors.fieldError("payment_due_on")}
          onChange={setDueDate}
          onValidityChange={setDueDateValid}
        />
        <Textarea
          id="billing-letters-notes"
          name="notes"
          label="Hinweise für die Eltern (freiwillig)"
          placeholder={
            "Zum Beispiel: Bitte denken Sie an Frühstück, Wechselkleidung und Hausschuhe.\nKinder mit ansteckenden Krankheiten bleiben zu Hause."
          }
          rows={5}
          value={notes}
          maxLength={4000}
          error={formErrors.fieldError("notes") ?? undefined}
          onChange={(event) => setNotes(event.target.value)}
        />
        <fieldset className="space-y-3" id="emergency_contacts">
          <legend className="text-sm font-medium text-gray-700">
            Notfallnummern der Betreuung (freiwillig)
          </legend>
          <p className="text-sm text-gray-600">
            Unter diesen Nummern erreichen Eltern die Betreuung, wenn ihr Kind
            krank ist.
          </p>
          {contacts.map((contact, index) => (
            <ContactRow
              // Rows have no ID of their own; the position is the row.
              // oxlint-disable-next-line react/no-array-index-key
              key={index}
              index={index}
              contact={contact}
              error={index === incomplete ? (contactsError ?? null) : null}
              onChange={(next) =>
                setContacts((current) =>
                  current.map((row, at) => (at === index ? next : row)),
                )
              }
              onRemove={() =>
                setContacts((current) =>
                  current.filter((_, at) => at !== index),
                )
              }
            />
          ))}
          {contacts.length < MAX_CONTACTS ? (
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() =>
                setContacts((current) => [...current, { name: "", phone: "" }])
              }
            >
              <Plus className="mr-1 h-4 w-4" aria-hidden="true" />
              Notfallnummer hinzufügen
            </Button>
          ) : null}
        </fieldset>
      </form>
    </FormModal>
  );
}
