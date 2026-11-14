"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { FieldLabel } from "~/components/ui/field-label";
import { FormModal } from "~/components/ui/form-modal";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import { WeekdayToggleGroup } from "~/components/ui/weekday-toggle-group";
import { useApiFormError } from "~/contexts/ToastContext";
import { berlinTodayISO } from "~/lib/date-helpers";
import { AUDIENCE_LABELS } from "~/lib/staff-notices-api";
import type {
  StaffNotice,
  StaffNoticeAudience,
  StaffNoticeInput,
  StaffNoticePriority,
  StaffNoticeWeekPattern,
} from "~/lib/staff-notices-api";

const WEEKDAYS = [
  { value: 1, label: "Mo" },
  { value: 2, label: "Di" },
  { value: 3, label: "Mi" },
  { value: 4, label: "Do" },
  { value: 5, label: "Fr" },
  { value: 6, label: "Sa" },
  { value: 7, label: "So" },
] as const;

const PRIORITY_OPTIONS = [
  { value: "info", label: "Information" },
  { value: "important", label: "Wichtig" },
];

// Zielgruppe (#2208): drei feste Werte. Die Erklärung darunter sagt, wo der
// Hinweis ankommt — die Leitung sieht ihn selbst nur im OGS-Portal.
const AUDIENCE_OPTIONS: readonly {
  value: StaffNoticeAudience;
  label: string;
}[] = [
  { value: "all", label: AUDIENCE_LABELS.all },
  { value: "staff", label: AUDIENCE_LABELS.staff },
  { value: "lehrkraft", label: AUDIENCE_LABELS.lehrkraft },
];

const AUDIENCE_HINTS: Readonly<Record<StaffNoticeAudience, string>> = {
  all: "Erscheint bei der Betreuung im OGS-Portal und bei Lehrkräften in moto schule.",
  staff: "Erscheint nur bei der Betreuung im OGS-Portal.",
  lehrkraft: "Erscheint nur bei Lehrkräften in moto schule.",
};

const WEEK_PATTERN_OPTIONS = [
  { value: "0", label: "Jede Woche" },
  { value: "1", label: "Nur Woche A" },
  { value: "2", label: "Nur Woche B" },
];

interface FormState {
  title: string;
  body: string;
  priority: StaffNoticePriority;
  audience: StaffNoticeAudience;
  validFrom: string;
  validUntil: string;
  weekdays: number[];
  weekPattern: StaffNoticeWeekPattern;
  requiresAcknowledgement: boolean;
  active: boolean;
}

function initialState(notice: StaffNotice | null): FormState {
  if (!notice) {
    return {
      title: "",
      body: "",
      priority: "info",
      audience: "all",
      validFrom: berlinTodayISO(),
      validUntil: "",
      weekdays: [],
      weekPattern: 0,
      requiresAcknowledgement: false,
      active: true,
    };
  }
  return {
    title: notice.title,
    body: notice.body,
    priority: notice.priority,
    audience: notice.audience,
    validFrom: notice.valid_from,
    validUntil: notice.valid_until ?? "",
    weekdays: [...notice.weekdays],
    weekPattern: notice.week_pattern,
    requiresAcknowledgement: notice.requires_acknowledgement,
    active: notice.active,
  };
}

/**
 * Anlegen und Bearbeiten einer Tagesinformation (#2180).
 *
 * Die Wiederholung ist bewusst schlicht: Zeitraum, Wochentage, Woche A/B —
 * dieselben drei Begriffe wie im Stundenplan und im Dienstplan. Wer keine
 * Wochentage anhakt, meint jeden Tag des Zeitraums; das steht auch so an der
 * Auswahl, damit niemand raten muss.
 */
export function StaffNoticeModal({
  isOpen,
  notice,
  onClose,
  onSubmit,
}: {
  readonly isOpen: boolean;
  readonly notice: StaffNotice | null;
  readonly onClose: () => void;
  readonly onSubmit: (input: StaffNoticeInput) => Promise<void>;
}) {
  const [form, setForm] = useState<FormState>(() => initialState(notice));
  const save = useFormModalPending(isOpen);
  const saving = save.pending;
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  const clearErrors = formErrors.clear;
  const latestSubmitRef = useRef<() => void>(() => undefined);

  // Beim Öffnen mit den Werten des gewählten Hinweises starten (oder leer für
  // einen neuen), damit ein zweiter Aufruf nicht die vorige Eingabe zeigt.
  useEffect(() => {
    if (isOpen) {
      setForm(initialState(notice));
      clearErrors();
    }
  }, [isOpen, notice, clearErrors]);

  const toggleWeekday = (day: number) => {
    setForm((prev) => ({
      ...prev,
      weekdays: prev.weekdays.includes(day)
        ? prev.weekdays.filter((d) => d !== day)
        : [...prev.weekdays, day].sort((a, b) => a - b),
    }));
  };

  const handleSubmit = async () => {
    const fields = noticeFieldErrors(form);
    if (Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie die markierten Felder.", fields);
      return;
    }

    await save.run(async (op) => {
      formErrors.clear();
      try {
        await onSubmit({
          title: form.title.trim(),
          body: form.body.trim(),
          priority: form.priority,
          audience: form.audience,
          valid_from: form.validFrom,
          valid_until: form.validUntil || null,
          weekdays: form.weekdays,
          week_pattern: form.weekPattern,
          requires_acknowledgement: form.requiresAcknowledgement,
          active: form.active,
        });
        if (op.isCurrent()) onClose();
      } catch (err) {
        if (!op.isCurrent()) return;
        // Protokolliert hat die Seite schon. Der Fehler bleibt im Dialog.
        await formErrors.show(err, {
          object: "das Speichern der Tagesinformation",
          retry: () => latestSubmitRef.current(),
        });
      }
    });
  };
  // „Wiederholen“ sendet den Stand, der dann im Formular steht.
  useLayoutEffect(() => {
    latestSubmitRef.current = () => void handleSubmit();
  });

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      pending={save}
      title={notice ? "Tagesinformation bearbeiten" : "Neue Tagesinformation"}
      error={formErrors.error}
      footer={
        <div className="flex justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="md"
            noTooltip="Abbrechen schließt den Dialog und speichert nichts."
            onClick={onClose}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            disabled={saving}
            // Tagesinformationen schicken keine Benachrichtigung; sie stehen
            // im gewählten Zeitraum bei der Zielgruppe (timetable staff_notices).
            tooltip="Speichert die Tagesinformation für die Zielgruppe, ohne Benachrichtigung."
            onClick={() => void handleSubmit()}
          >
            {saving ? "Wird gespeichert …" : "Speichern"}
          </Button>
        </div>
      }
    >
      <div ref={formRef} className="space-y-4">
        <Input
          label="Titel"
          name="title"
          value={form.title}
          error={formErrors.fieldError("title")}
          maxLength={200}
          onChange={(e) => setForm({ ...form, title: e.target.value })}
        />

        <Textarea
          label="Hinweis"
          name="body"
          rows={4}
          value={form.body}
          error={formErrors.fieldError("body")}
          onChange={(e) => setForm({ ...form, body: e.target.value })}
        />

        <CustomSelect
          label="Für wen"
          hint={AUDIENCE_HINTS[form.audience]}
          value={form.audience}
          options={AUDIENCE_OPTIONS}
          onChange={(value) =>
            setForm({ ...form, audience: value as StaffNoticeAudience })
          }
        />

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <CustomSelect
            label="Wichtigkeit"
            value={form.priority}
            options={PRIORITY_OPTIONS}
            onChange={(value) =>
              setForm({ ...form, priority: value as StaffNoticePriority })
            }
          />
          <CustomSelect
            label="Woche"
            value={String(form.weekPattern)}
            options={WEEK_PATTERN_OPTIONS}
            onChange={(value) =>
              setForm({
                ...form,
                weekPattern: Number(value) as StaffNoticeWeekPattern,
              })
            }
          />
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <ISODatePicker
            id="notice-valid-from"
            name="valid_from"
            label="Gilt ab"
            value={form.validFrom}
            hideClearButton
            calendarLayout="popover"
            error={formErrors.fieldError("valid_from")}
            onChange={(value) => {
              if (value) setForm({ ...form, validFrom: value });
            }}
          />
          <ISODatePicker
            id="notice-valid-until"
            name="valid_until"
            label="Gilt bis (optional)"
            value={form.validUntil}
            calendarLayout="popover"
            error={formErrors.fieldError("valid_until")}
            onChange={(value) => setForm({ ...form, validUntil: value })}
          />
        </div>

        <div>
          <FieldLabel>Wochentage</FieldLabel>
          <WeekdayToggleGroup
            ariaLabel="Wochentage"
            days={WEEKDAYS}
            value={form.weekdays}
            onToggle={toggleWeekday}
            hint="Ohne Auswahl gilt der Hinweis an jedem Tag des Zeitraums."
          />
        </div>

        <label
          htmlFor="notice-requires-ack"
          className="flex cursor-pointer items-center gap-3"
        >
          <Checkbox
            id="notice-requires-ack"
            checked={form.requiresAcknowledgement}
            onChange={(e) =>
              setForm({ ...form, requiresAcknowledgement: e.target.checked })
            }
          />
          <span className="text-sm text-gray-700">Kenntnisnahme verlangen</span>
        </label>

        <label
          htmlFor="notice-active"
          className="flex cursor-pointer items-center gap-3"
        >
          <Checkbox
            id="notice-active"
            checked={form.active}
            onChange={(e) => setForm({ ...form, active: e.target.checked })}
          />
          <span className="text-sm text-gray-700">
            Aktiv (abgeschaltete Hinweise sieht das Team nicht)
          </span>
        </label>
      </div>
    </FormModal>
  );
}

/** Prüfung vor dem Senden. Schlüssel sind die Feldnamen des Backends. */
function noticeFieldErrors(form: FormState): Record<string, string> {
  const fields: Record<string, string> = {};
  if (!form.title.trim()) fields.title = "Bitte geben Sie einen Titel an.";
  if (form.validUntil && form.validUntil < form.validFrom)
    fields.valid_until = "Das Ende darf nicht vor dem Beginn liegen.";
  return fields;
}
