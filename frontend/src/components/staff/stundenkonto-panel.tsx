"use client";

// Stundenkonto-Verwaltung (#1420): Auszahlung / Freizeitausgleich-Gewährung /
// Schuljahres-Reset als Admin-Aktionen auf dem Übersicht-Tab, plus die
// Historie aller Buchungen. Jede Buchung ist eine eigene Transaktion im
// Stundenkonto, kein Zeiteintrag — der Server rechnet sie live in die
// Monatskarten-Kette ein. Dazu der einmalige Eröffnungssaldo (#2132), der den
// Übernahme-Stand aus dem Altsystem setzt.

import { useState } from "react";
import { Banknote, Clock4, Flag, RotateCcw, Trash2 } from "lucide-react";

import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { ISODatePicker } from "~/components/ui/date-picker";
import { Modal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import { staffBalanceAdjustmentService } from "~/lib/staff-api";
import {
  balanceAdjustmentTypeLabel,
  type BalanceAdjustment,
} from "~/lib/time-tracking-helpers";

const DECIMAL_INPUT_PATTERN = /^[+-]?(?:\d+(?:[,.]\d+)?|[,.]\d+)$/;

function parseDecimalInput(value: string): number | null {
  const normalized = value.trim();
  if (!DECIMAL_INPUT_PATTERN.test(normalized)) return null;
  const parsed = Number(normalized.replace(",", "."));
  return Number.isFinite(parsed) ? parsed : null;
}

import { formatSignedDuration } from "./staff-time-views";

interface StundenkontoPanelProps {
  readonly staffId: string;
  /** Live Stundenkonto in Minuten (null solange es lädt). */
  readonly balanceMinutes: number | null;
  /** Kontostart ("YYYY-MM-DD") — Buchungen davor lehnt der Server ab. */
  readonly accountStartKey: string;
  /** Heutiges Datum in Berlin ("YYYY-MM-DD"). */
  readonly todayKey: string;
  readonly adjustments: readonly BalanceAdjustment[];
  readonly onChanged: () => void | Promise<unknown>;
}

export function StundenkontoPanel({
  staffId,
  balanceMinutes,
  accountStartKey,
  todayKey,
  adjustments,
  onChanged,
}: StundenkontoPanelProps) {
  const [activeModal, setActiveModal] = useState<
    "payout" | "comp_time" | "reset" | "opening" | null
  >(null);
  const [deleteTarget, setDeleteTarget] = useState<BalanceAdjustment | null>(
    null,
  );
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");
  const toast = useToast();
  // Reset und Eröffnungssaldo brauchen beide einen abgeschlossenen Kontotag:
  // der Stichtag muss vor heute liegen.
  const canResetClosedPeriod = accountStartKey < todayKey;
  const hasOpening = adjustments.some(
    (adjustment) => adjustment.type === "opening",
  );
  const openingDisabledHint = () => {
    if (hasOpening) {
      return "Es existiert bereits ein Eröffnungssaldo. Lösche zuerst die bestehende Buchung.";
    }
    if (!canResetClosedPeriod) {
      return "Ein Eröffnungssaldo ist erst nach dem ersten abgeschlossenen Kontotag möglich.";
    }
    return undefined;
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    setDeleteError("");
    try {
      await staffBalanceAdjustmentService.delete(staffId, deleteTarget.id);
      toast.success("Buchung gelöscht.");
      setDeleteTarget(null);
      await onChanged();
    } catch (err) {
      setDeleteError(
        err instanceof Error ? err.message : "Löschen fehlgeschlagen.",
      );
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div className="rounded-3xl border border-gray-100/50 bg-white/90 p-4 shadow-[0_8px_30px_rgb(0,0,0,0.12)] sm:p-6">
      <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
        <h3 className="text-sm font-semibold tracking-wide text-gray-400 uppercase">
          Stundenkonto-Verwaltung
        </h3>
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="outline"
            size="compact"
            onClick={() => setActiveModal("payout")}
          >
            <Banknote className="h-3.5 w-3.5" aria-hidden />
            Auszahlung
          </Button>
          <Button
            type="button"
            variant="outline"
            size="compact"
            onClick={() => setActiveModal("comp_time")}
          >
            <Clock4 className="h-3.5 w-3.5" aria-hidden />
            Freizeitausgleich
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={() => setActiveModal("reset")}
            disabled={!canResetClosedPeriod}
            title={
              canResetClosedPeriod
                ? undefined
                : "Ein Reset ist erst nach dem ersten abgeschlossenen Kontotag möglich."
            }
          >
            <RotateCcw className="h-3.5 w-3.5" aria-hidden />
            Zurücksetzen
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={() => setActiveModal("opening")}
            disabled={!canResetClosedPeriod || hasOpening}
            title={openingDisabledHint()}
          >
            <Flag className="h-3.5 w-3.5" aria-hidden />
            Eröffnungssaldo
          </Button>
        </div>
      </div>

      {adjustments.length === 0 ? (
        <p className="py-6 text-center text-sm text-gray-400">
          Noch keine Buchungen — Auszahlungen, Freizeitausgleich, Resets und
          Eröffnungssalden erscheinen hier.
        </p>
      ) : (
        <ul className="divide-y divide-gray-100">
          {adjustments.map((adjustment) => (
            <li
              key={adjustment.id}
              className="flex items-center justify-between gap-3 py-2.5"
            >
              <div className="min-w-0">
                <p className="text-sm text-gray-800">
                  <span className="font-medium">
                    {balanceAdjustmentTypeLabel(adjustment.type)}
                  </span>
                  <span className="ml-2 text-gray-500">
                    {formatDate(adjustment.effectiveDate)}
                  </span>
                </p>
                {adjustment.note && (
                  <p className="truncate text-xs text-gray-500">
                    {adjustment.note}
                  </p>
                )}
              </div>
              <div className="flex shrink-0 items-center gap-2">
                <span className="text-sm font-medium text-gray-900 tabular-nums">
                  {formatSignedDuration(adjustment.minutesDelta)}
                </span>
                <button
                  type="button"
                  onClick={() => setDeleteTarget(adjustment)}
                  aria-label={`Buchung ${balanceAdjustmentTypeLabel(adjustment.type)} vom ${formatDate(adjustment.effectiveDate)} löschen`}
                  title="Buchung löschen"
                  className="rounded-md p-1.5 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-600"
                >
                  <Trash2 className="h-4 w-4" aria-hidden />
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}

      {(activeModal === "payout" || activeModal === "comp_time") && (
        <AdjustmentModal
          staffId={staffId}
          accountStartKey={accountStartKey}
          todayKey={todayKey}
          type={activeModal}
          onClose={() => setActiveModal(null)}
          onSaved={async () => {
            setActiveModal(null);
            await onChanged();
          }}
        />
      )}
      {activeModal === "reset" && (
        <ResetModal
          staffId={staffId}
          accountStartKey={accountStartKey}
          todayKey={todayKey}
          balanceMinutes={balanceMinutes}
          onClose={() => setActiveModal(null)}
          onSaved={async () => {
            setActiveModal(null);
            await onChanged();
          }}
        />
      )}
      {activeModal === "opening" && (
        <OpeningModal
          staffId={staffId}
          accountStartKey={accountStartKey}
          todayKey={todayKey}
          balanceMinutes={balanceMinutes}
          onClose={() => setActiveModal(null)}
          onSaved={async () => {
            setActiveModal(null);
            await onChanged();
          }}
        />
      )}
      <ConfirmDeleteModal
        isOpen={deleteTarget !== null}
        title="Buchung löschen"
        description={
          deleteTarget ? (
            <>
              Die Buchung{" "}
              <strong>
                {balanceAdjustmentTypeLabel(deleteTarget.type)} (
                {formatSignedDuration(deleteTarget.minutesDelta)})
              </strong>{" "}
              vom {formatDate(deleteTarget.effectiveDate)} wird entfernt. Das
              Stundenkonto wird entsprechend neu berechnet.
            </>
          ) : (
            ""
          )
        }
        gate={{ mode: "twoStep" }}
        onConfirm={handleDelete}
        onClose={() => {
          if (!deleting) {
            setDeleteTarget(null);
            setDeleteError("");
          }
        }}
        loading={deleting}
        error={deleteError}
      />
    </div>
  );
}

const modalCopy = {
  payout: {
    title: "Plus-Stunden auszahlen",
    amountLabel: "Auszahlung (Stunden)",
    hint: "Die ausgezahlten Stunden werden vom Stundenkonto abgezogen und in der Historie als Auszahlung ausgewiesen.",
    success: "Auszahlung gebucht.",
    submit: "Auszahlen",
  },
  comp_time: {
    title: "Freizeitausgleich gewähren",
    amountLabel: "Freizeitausgleich (Stunden)",
    hint: "Pauschale Gutschrift-Verrechnung: die Stunden werden direkt vom Stundenkonto abgezogen. Für einen ganzen freien Tag stattdessen eine Abwesenheit vom Typ Freizeitausgleich anlegen — dann sinkt das Konto automatisch um das Tagessoll.",
    success: "Freizeitausgleich gebucht.",
    submit: "Gewähren",
  },
} as const;

function AdjustmentModal({
  staffId,
  accountStartKey,
  todayKey,
  type,
  onClose,
  onSaved,
}: {
  readonly staffId: string;
  readonly accountStartKey: string;
  readonly todayKey: string;
  readonly type: "payout" | "comp_time";
  readonly onClose: () => void;
  readonly onSaved: () => void | Promise<void>;
}) {
  const copy = modalCopy[type];
  const [hours, setHours] = useState("");
  const [effectiveDate, setEffectiveDate] = useState(todayKey);
  // Backend bound: the full calendar month 12 months ahead is supported.
  const horizon = parseISODate(todayKey);
  horizon.setFullYear(horizon.getFullYear() + 1, horizon.getMonth() + 1, 0);
  const maxEffectiveDateKey = toISODate(horizon);
  const [note, setNote] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const toast = useToast();

  const handleSubmit = async () => {
    const h = Number.parseFloat(hours.replace(",", "."));
    if (Number.isNaN(h) || h <= 0 || h > 1000) {
      toast.error("Stundenangabe ungültig.");
      return;
    }
    if (!effectiveDate) {
      toast.error("Datum fehlt.");
      return;
    }
    setSubmitting(true);
    try {
      await staffBalanceAdjustmentService.create(staffId, {
        type,
        minutesDelta: -Math.round(h * 60),
        effectiveDate,
        note: note.trim(),
      });
      toast.success(copy.success);
      await onSaved();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Buchung fehlgeschlagen.",
      );
    } finally {
      setSubmitting(false);
    }
  };

  const canSubmit = !submitting && hours.trim() !== "" && note.trim() !== "";

  return (
    <Modal
      isOpen
      onClose={() => !submitting && onClose()}
      title={copy.title}
      footer={
        <AdjustmentModalFooter
          submitting={submitting}
          canSubmit={canSubmit}
          submitLabel={copy.submit}
          onCancel={onClose}
          onSubmit={handleSubmit}
        />
      }
    >
      <div className="space-y-4">
        <p className="text-sm text-gray-500">{copy.hint}</p>
        <div>
          <label
            htmlFor="adjustment-hours"
            className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase"
          >
            {copy.amountLabel}
          </label>
          <input
            id="adjustment-hours"
            type="number"
            min="0.25"
            max="1000"
            step="0.25"
            value={hours}
            onChange={(e) => setHours(e.target.value)}
            placeholder="z. B. 8"
            className="w-full rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-800 focus:border-[#83CD2D] focus:outline-none"
          />
        </div>
        <div>
          <label
            htmlFor="adjustment-date"
            className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase"
          >
            Wirksam am
          </label>
          <ISODatePicker
            id="adjustment-date"
            min={accountStartKey}
            max={maxEffectiveDateKey}
            value={effectiveDate}
            onChange={setEffectiveDate}
            calendarLayout="popover"
            hideClearButton
          />
        </div>
        <NoteField note={note} onChange={setNote} />
      </div>
    </Modal>
  );
}

function ResetModal({
  staffId,
  accountStartKey,
  todayKey,
  balanceMinutes,
  onClose,
  onSaved,
}: {
  readonly staffId: string;
  readonly accountStartKey: string;
  readonly todayKey: string;
  readonly balanceMinutes: number | null;
  readonly onClose: () => void;
  readonly onSaved: () => void | Promise<void>;
}) {
  const lastClosedDateKey = previousDateISO(todayKey);
  const [effectiveDate, setEffectiveDate] = useState(
    defaultResetDateISO(accountStartKey, todayKey),
  );
  const [carryoverHours, setCarryoverHours] = useState("0");
  const [note, setNote] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const toast = useToast();

  const carryover = Number.parseFloat(carryoverHours.replace(",", "."));
  const carryoverValid =
    !Number.isNaN(carryover) && carryover >= 0 && carryover <= 10_000;

  const handleSubmit = async () => {
    if (!carryoverValid) {
      toast.error("Übertrag ungültig.");
      return;
    }
    if (!effectiveDate) {
      toast.error("Datum fehlt.");
      return;
    }
    setSubmitting(true);
    try {
      await staffBalanceAdjustmentService.reset(staffId, {
        effectiveDate,
        carryoverMinutes: Math.round(carryover * 60),
        note: note.trim(),
      });
      toast.success("Stundenkonto zurückgesetzt.");
      await onSaved();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Reset fehlgeschlagen.");
    } finally {
      setSubmitting(false);
    }
  };

  const canSubmit = !submitting && carryoverValid && note.trim() !== "";

  return (
    <Modal
      isOpen
      onClose={() => !submitting && onClose()}
      title="Stundenkonto zurücksetzen"
      footer={
        <AdjustmentModalFooter
          submitting={submitting}
          canSubmit={canSubmit}
          submitLabel="Zurücksetzen"
          onCancel={onClose}
          onSubmit={handleSubmit}
        />
      }
    >
      <div className="space-y-4">
        <p className="text-sm text-gray-500">
          Setzt das Stundenkonto zum gewählten Stichtag auf den angegebenen
          Übertrag zurück (Schuljahresende: 31.07.). Der Server berechnet den
          Kontostand zum Stichtag und bucht die Differenz als Reset.
        </p>
        {balanceMinutes !== null && (
          <div className="space-y-1 rounded-xl bg-gray-50 px-3 py-2 text-sm text-gray-700">
            <p>
              Aktueller Stand:{" "}
              <span className="font-medium tabular-nums">
                {formatSignedDuration(balanceMinutes)}
              </span>
            </p>
            {carryoverValid && (
              <p>
                Gewünschter Übertrag am Stichtag:{" "}
                <span className="font-medium tabular-nums">
                  {formatSignedDuration(Math.round(carryover * 60))}
                </span>
              </p>
            )}
          </div>
        )}
        <div>
          <label
            htmlFor="reset-date"
            className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase"
          >
            Stichtag
          </label>
          <ISODatePicker
            id="reset-date"
            min={accountStartKey}
            max={lastClosedDateKey}
            value={effectiveDate}
            onChange={setEffectiveDate}
            calendarLayout="popover"
            hideClearButton
          />
          <p className="mt-1 text-xs text-gray-400">
            Der Stichtag muss vor dem heutigen Tag liegen.
          </p>
        </div>
        <div>
          <label
            htmlFor="reset-carryover"
            className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase"
          >
            Verbleibender Übertrag (Stunden)
          </label>
          <input
            id="reset-carryover"
            type="number"
            min="0"
            step="0.25"
            value={carryoverHours}
            onChange={(e) => setCarryoverHours(e.target.value)}
            className="w-full rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-800 focus:border-[#83CD2D] focus:outline-none"
          />
        </div>
        <NoteField note={note} onChange={setNote} />
      </div>
    </Modal>
  );
}

// Eröffnungssaldo (#2132): einmalige Übernahme aus dem Altsystem. Gleiche
// Stichtags-Grenzen wie der Reset, aber der Zielwert darf negativ sein.
function OpeningModal({
  staffId,
  accountStartKey,
  todayKey,
  balanceMinutes,
  onClose,
  onSaved,
}: {
  readonly staffId: string;
  readonly accountStartKey: string;
  readonly todayKey: string;
  readonly balanceMinutes: number | null;
  readonly onClose: () => void;
  readonly onSaved: () => void | Promise<void>;
}) {
  const lastClosedDateKey = previousDateISO(todayKey);
  const [effectiveDate, setEffectiveDate] = useState(
    accountStartKey <= lastClosedDateKey ? accountStartKey : lastClosedDateKey,
  );
  const [openingHours, setOpeningHours] = useState("0");
  const [note, setNote] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const toast = useToast();

  const opening = parseDecimalInput(openingHours);
  const openingValid =
    opening !== null && opening >= -10_000 && opening <= 10_000;

  const handleSubmit = async () => {
    if (opening === null || opening < -10_000 || opening > 10_000) {
      toast.error("Eröffnungssaldo ungültig.");
      return;
    }
    if (!effectiveDate) {
      toast.error("Datum fehlt.");
      return;
    }
    setSubmitting(true);
    try {
      await staffBalanceAdjustmentService.createOpening(staffId, {
        effectiveDate,
        balanceMinutes: Math.round(opening * 60),
        note: note.trim(),
      });
      toast.success("Eröffnungssaldo gebucht.");
      await onSaved();
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Buchung fehlgeschlagen.",
      );
    } finally {
      setSubmitting(false);
    }
  };

  const canSubmit = !submitting && openingValid && note.trim() !== "";

  return (
    <Modal
      isOpen
      onClose={() => !submitting && onClose()}
      title="Eröffnungssaldo buchen"
      footer={
        <AdjustmentModalFooter
          submitting={submitting}
          canSubmit={canSubmit}
          submitLabel="Buchen"
          onCancel={onClose}
          onSubmit={handleSubmit}
        />
      }
    >
      <div className="space-y-4">
        <p className="text-sm text-gray-500">
          Der Eröffnungssaldo setzt den Übernahme-Stand aus dem Altsystem. Das
          Stundenkonto steht zum Stichtag danach exakt auf dem eingetragenen
          Wert. Negative Werte sind zulässig, wenn im Altsystem Minusstunden
          bestanden. Pro Person ist genau ein Eröffnungssaldo möglich. Für eine
          Korrektur die bestehende Buchung löschen und neu anlegen.
        </p>
        {balanceMinutes !== null && (
          <div className="space-y-1 rounded-xl bg-gray-50 px-3 py-2 text-sm text-gray-700">
            <p>
              Aktueller Stand:{" "}
              <span className="font-medium tabular-nums">
                {formatSignedDuration(balanceMinutes)}
              </span>
            </p>
            {opening !== null && openingValid && (
              <p>
                Gewünschter Stand am Stichtag:{" "}
                <span className="font-medium tabular-nums">
                  {formatSignedDuration(Math.round(opening * 60))}
                </span>
              </p>
            )}
          </div>
        )}
        <div>
          <label
            htmlFor="opening-date"
            className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase"
          >
            Stichtag
          </label>
          <ISODatePicker
            id="opening-date"
            min={accountStartKey}
            max={lastClosedDateKey}
            value={effectiveDate}
            onChange={setEffectiveDate}
            calendarLayout="popover"
            hideClearButton
          />
          <p className="mt-1 text-xs text-gray-400">
            Der Stichtag muss vor dem heutigen Tag liegen.
          </p>
        </div>
        <div>
          <label
            htmlFor="opening-balance"
            className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase"
          >
            Übernommener Saldo (Stunden)
          </label>
          <input
            id="opening-balance"
            type="text"
            inputMode="decimal"
            value={openingHours}
            onChange={(e) => setOpeningHours(e.target.value)}
            placeholder="z. B. 12,5 oder -3"
            className="w-full rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-800 focus:border-[#83CD2D] focus:outline-none"
          />
        </div>
        <NoteField
          note={note}
          onChange={setNote}
          placeholder="z. B. Übernahme aus Altsystem, Stand 31.07."
        />
      </div>
    </Modal>
  );
}

function AdjustmentModalFooter({
  submitting,
  canSubmit,
  submitLabel,
  onCancel,
  onSubmit,
}: {
  readonly submitting: boolean;
  readonly canSubmit: boolean;
  readonly submitLabel: string;
  readonly onCancel: () => void;
  readonly onSubmit: () => void;
}) {
  return (
    <div className="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={onCancel}
        disabled={submitting}
      >
        Abbrechen
      </Button>
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={onSubmit}
        disabled={!canSubmit}
      >
        {submitting ? "…" : submitLabel}
      </Button>
    </div>
  );
}

function NoteField({
  note,
  onChange,
  placeholder = "z. B. Auszahlung mit Juligehalt",
}: {
  readonly note: string;
  readonly onChange: (value: string) => void;
  /** Beispieltext passend zur Buchungsart; Standard ist die Auszahlung. */
  readonly placeholder?: string;
}) {
  return (
    <div>
      <label
        htmlFor="adjustment-note"
        className="mb-1 block text-xs font-semibold tracking-wider text-gray-500 uppercase"
      >
        Begründung (Pflicht)
      </label>
      <textarea
        id="adjustment-note"
        value={note}
        onChange={(e) => onChange(e.target.value)}
        rows={2}
        placeholder={placeholder}
        className="w-full rounded-xl border border-gray-200 bg-white px-3 py-2 text-sm text-gray-800 focus:border-[#83CD2D] focus:outline-none"
      />
    </div>
  );
}

function previousDateISO(dateKey: string): string {
  const date = parseISODate(dateKey);
  date.setDate(date.getDate() - 1);
  return toISODate(date);
}

// Most recent July 31st in a closed period — the Schuljahresende the reset
// usually anchors on (#1420 5c). Clamped at the account start: a booking
// before the anchor sits outside every carry chain and the server rejects it.
function defaultResetDateISO(
  accountStartKey: string,
  todayKey: string,
): string {
  const lastClosedDateKey = previousDateISO(todayKey);
  const year = Number(lastClosedDateKey.slice(0, 4));
  const thisYearsEnd = `${year}-07-31`;
  const lastSchoolYearEnd =
    thisYearsEnd <= lastClosedDateKey ? thisYearsEnd : `${year - 1}-07-31`;
  if (lastSchoolYearEnd < accountStartKey) {
    return accountStartKey <= lastClosedDateKey
      ? lastClosedDateKey
      : accountStartKey;
  }
  return lastSchoolYearEnd;
}
