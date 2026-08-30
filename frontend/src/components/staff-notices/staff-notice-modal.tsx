"use client";

import { useEffect, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { CustomSelect } from "~/components/ui/custom-select";
import { DatePicker } from "~/components/ui/date-picker";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import { parseISODate, toISODate, todayISO } from "~/lib/date-helpers";
import type {
  StaffNotice,
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

const WEEK_PATTERN_OPTIONS = [
  { value: "0", label: "Jede Woche" },
  { value: "1", label: "Nur Woche A" },
  { value: "2", label: "Nur Woche B" },
];

interface FormState {
  title: string;
  body: string;
  priority: StaffNoticePriority;
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
      validFrom: todayISO(),
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
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);

  // Beim Öffnen mit den Werten des gewählten Hinweises starten (oder leer für
  // einen neuen), damit ein zweiter Aufruf nicht die vorige Eingabe zeigt.
  useEffect(() => {
    if (isOpen) {
      setForm(initialState(notice));
      setError("");
    }
  }, [isOpen, notice]);

  const toggleWeekday = (day: number) => {
    setForm((prev) => ({
      ...prev,
      weekdays: prev.weekdays.includes(day)
        ? prev.weekdays.filter((d) => d !== day)
        : [...prev.weekdays, day].sort((a, b) => a - b),
    }));
  };

  const handleSubmit = async () => {
    if (!form.title.trim()) {
      setError("Bitte einen Titel angeben.");
      return;
    }
    if (form.validUntil && form.validUntil < form.validFrom) {
      setError("Das Ende darf nicht vor dem Beginn liegen.");
      return;
    }

    setSaving(true);
    setError("");
    try {
      await onSubmit({
        title: form.title.trim(),
        body: form.body.trim(),
        priority: form.priority,
        valid_from: form.validFrom,
        valid_until: form.validUntil || null,
        weekdays: form.weekdays,
        week_pattern: form.weekPattern,
        requires_acknowledgement: form.requiresAcknowledgement,
        active: form.active,
      });
      onClose();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Die Tagesinformation konnte nicht gespeichert werden.",
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={notice ? "Tagesinformation bearbeiten" : "Neue Tagesinformation"}
      footer={
        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" size="md" onClick={onClose}>
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            disabled={saving}
            onClick={() => void handleSubmit()}
          >
            {saving ? "Wird gespeichert …" : "Speichern"}
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        {error && <Alert type="error" message={error} />}

        <Input
          label="Titel"
          value={form.title}
          maxLength={200}
          onChange={(e) => setForm({ ...form, title: e.target.value })}
        />

        <Textarea
          label="Hinweis"
          rows={4}
          value={form.body}
          onChange={(e) => setForm({ ...form, body: e.target.value })}
        />

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <span className="mb-1 block text-sm font-medium text-gray-700">
              Wichtigkeit
            </span>
            <CustomSelect
              value={form.priority}
              options={PRIORITY_OPTIONS}
              ariaLabel="Wichtigkeit"
              onChange={(value) =>
                setForm({ ...form, priority: value as StaffNoticePriority })
              }
            />
          </div>
          <div>
            <span className="mb-1 block text-sm font-medium text-gray-700">
              Woche
            </span>
            <CustomSelect
              value={String(form.weekPattern)}
              options={WEEK_PATTERN_OPTIONS}
              ariaLabel="Woche"
              onChange={(value) =>
                setForm({
                  ...form,
                  weekPattern: Number(value) as StaffNoticeWeekPattern,
                })
              }
            />
          </div>
        </div>

        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <span className="mb-1 block text-sm font-medium text-gray-700">
              Gilt ab
            </span>
            <DatePicker
              value={parseISODate(form.validFrom)}
              hideClearButton
              calendarLayout="popover"
              onChange={(date) => {
                if (date) setForm({ ...form, validFrom: toISODate(date) });
              }}
            />
          </div>
          <div>
            <span className="mb-1 block text-sm font-medium text-gray-700">
              Gilt bis (optional)
            </span>
            <DatePicker
              value={form.validUntil ? parseISODate(form.validUntil) : null}
              calendarLayout="popover"
              onChange={(date) =>
                setForm({ ...form, validUntil: date ? toISODate(date) : "" })
              }
            />
          </div>
        </div>

        <div>
          <span className="mb-1 block text-sm font-medium text-gray-700">
            Wochentage
          </span>
          <div className="flex flex-wrap gap-2">
            {WEEKDAYS.map((day) => {
              const selected = form.weekdays.includes(day.value);
              return (
                <Button
                  key={day.value}
                  type="button"
                  aria-pressed={selected}
                  onClick={() => toggleWeekday(day.value)}
                  variant={selected ? "primary" : "outline"}
                  size="compact"
                  className="h-9 w-11 px-0"
                >
                  {day.label}
                </Button>
              );
            })}
          </div>
          <p className="mt-1 text-xs text-gray-500">
            Ohne Auswahl gilt der Hinweis an jedem Tag des Zeitraums.
          </p>
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
