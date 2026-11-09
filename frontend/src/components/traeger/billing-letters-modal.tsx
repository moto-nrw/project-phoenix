"use client";

// Bestätigungsschreiben eines freigegebenen Laufs erstellen (#4669):
// Zahlungsfrist, Hinweise für die Eltern (Packliste, Krankheitsregel) und
// die Notfallnummern der ausrichtenden OGS. Die Vorschau zeigt das Schreiben
// der ersten Familie; erstellt werden alle Schreiben einmal. Ein Monatslauf
// (#4684) nimmt dasselbe Formular mit eigener Quelle, ohne Notfallnummern.

import { Plus, X } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Button } from "~/components/ui/button";
import { ISODateInput } from "~/components/ui/date-picker";
import { DataField } from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { toISODate, berlinTodayISO, parseISODate } from "~/lib/date-helpers";
import type { InvoicePayment } from "~/lib/traeger-billing-terms-api";
import { invoiceDueDatesText } from "./billing-payment-format";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  type BillingEmergencyContact,
  type BillingFile,
  type BillingLetters,
  type BillingLettersInput,
  createBillingLetters,
  previewBillingLetter,
} from "~/lib/traeger-billing-api";

/**
 * Where the letters of another kind of run come from (#4684): its own
 * create and preview, its own words, and no emergency numbers.
 */
export interface LetterSource<T> {
  readonly create: (input: {
    paymentDueOn: string;
    notes: string;
  }) => Promise<T>;
  readonly preview: (input: {
    paymentDueOn: string;
    notes: string;
  }) => Promise<BillingFile>;
  /** The sentence above the fields. */
  readonly intro: string;
  readonly notesLabel: string;
  readonly notesPlaceholder: string;
}

const logger = createLogger({ component: "BillingLettersModal" });

const MAX_CONTACTS = 10;

/** Two weeks after today, the Bissendorf payment term. */
function defaultDueDate(): string {
  const date = parseISODate(berlinTodayISO());
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
          controlSize="touch"
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
          controlSize="touch"
          inputMode="tel"
          placeholder="0170 1234567"
          value={contact.phone}
          maxLength={120}
          onChange={(event) =>
            onChange({ ...contact, phone: event.target.value })
          }
        />
      </div>
      <div className="flex sm:pt-8">
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

export function BillingLettersModal<T = BillingLetters>({
  runId,
  payments,
  isOpen,
  onClose,
  onCreated,
  source,
}: Readonly<{
  runId: string;
  payments: readonly (InvoicePayment | null | undefined)[];
  isOpen: boolean;
  onClose: () => void;
  onCreated: (letters: T) => void;
  /** Another kind of run; without it the holiday letters of `runId`. */
  source?: LetterSource<T>;
}>) {
  const withContacts = source === undefined;
  const deadlines = invoiceDueDatesText(payments);
  const hasLegacy = payments.some((payment) => !payment?.dueOn);
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
    setContacts(withContacts ? [{ name: "", phone: "" }] : []);
    clearErrors();
  }, [isOpen, clearErrors, withContacts]);

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
    if (hasLegacy && !dueDate)
      fields.payment_due_on = "Bitte geben Sie ein Datum an.";
    else if (hasLegacy && !dueDateValid)
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
      const file = source
        ? await source.preview(value)
        : await previewBillingLetter(runId, value);
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
      const letters = source
        ? await source.create(value)
        : ((await createBillingLetters(runId, value)) as T);
      success("Die Schreiben sind erstellt.");
      onCreated(letters);
    } catch (err) {
      logger.error("billing_letters_create_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "das Erstellen der Schreiben" });
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
        // On a phone Abbrechen and „Schreiben erstellen“ share one
        // full-width row and the preview sits full width below (#5758).
        <div className="flex w-full flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between">
          <Button
            type="button"
            variant="outline"
            size="md"
            disabled={previewing || saving}
            onClick={() => void preview()}
            className="w-full sm:w-auto"
          >
            {previewing ? "Vorschau wird erstellt …" : "Vorschau ansehen"}
          </Button>
          <EditActions
            onCancel={onClose}
            onSave={() => formRef.current?.requestSubmit()}
            saving={saving}
            saveLabel="Schreiben erstellen"
            stretchOnPhone
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
          {source?.intro ??
            "Jede Familie bekommt ein Schreiben mit ihrer Rechnung. Ihre Hinweise stehen in allen Schreiben. Danach lässt sich nichts mehr ändern."}
        </p>
        {deadlines ? (
          <DataField label="Fälligkeiten">
            {deadlines}
            <p className="mt-1 text-sm text-gray-600">
              Diese Daten stehen fest. Sie ändern sich hier nicht.
            </p>
          </DataField>
        ) : null}
        {hasLegacy ? (
          <ISODateInput
            id="payment_due_on"
            label={deadlines ? "Zahlen bis (ältere Rechnungen)" : "Zahlen bis"}
            value={dueDate}
            min={berlinTodayISO()}
            minDateError="Das Datum darf nicht in der Vergangenheit liegen."
            error={formErrors.fieldError("payment_due_on")}
            onChange={setDueDate}
            onValidityChange={setDueDateValid}
          />
        ) : null}
        <Textarea
          id="billing-letters-notes"
          name="notes"
          label={source?.notesLabel ?? "Hinweise für die Eltern (freiwillig)"}
          placeholder={
            source?.notesPlaceholder ??
            "Zum Beispiel: Bitte denken Sie an Frühstück, Wechselkleidung und Hausschuhe.\nKinder mit ansteckenden Krankheiten bleiben zu Hause."
          }
          rows={5}
          value={notes}
          maxLength={4000}
          error={formErrors.fieldError("notes") ?? undefined}
          onChange={(event) => setNotes(event.target.value)}
        />
        {withContacts ? (
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
                  setContacts((current) => [
                    ...current,
                    { name: "", phone: "" },
                  ])
                }
              >
                <Plus className="mr-1 h-4 w-4" aria-hidden="true" />
                Notfallnummer hinzufügen
              </Button>
            ) : null}
          </fieldset>
        ) : null}
      </form>
    </FormModal>
  );
}
