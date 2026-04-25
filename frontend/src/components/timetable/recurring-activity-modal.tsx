"use client";

/**
 * RecurringActivityModal — form for creating a wiederkehrende Aktivität
 * (Mensa, Lernzeit, AG, externe Förderung) right from the planner.
 *
 * Submits to POST /api/timetable/templates which bundles the timeframe,
 * activities.groups (is_template=true) and activities.schedules into a
 * single transaction and — when materialize_from/to are passed — runs
 * materialization for the visible week so the new instances show up on
 * the grid immediately.
 *
 * Spontaneous one-off rows belong on the device (PyrePortal); see
 * SpontaneousInstanceModal — currently no longer wired into this page.
 */

import { useEffect, useMemo, useState } from "react";

import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Modal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { roomService, type Room } from "~/lib/api";
import type { ActivityCategory } from "~/lib/activity-helpers";
import {
  getActivityColor,
  getGermanWeekdayShort,
} from "~/lib/timetable-helpers";
import { createLogger } from "~/lib/logger";
import { timetableService } from "~/lib/timetable-api";
import type { ActivityType, CreateTemplateResult } from "~/lib/timetable-types";

const logger = createLogger({ component: "RecurringActivityModal" });

interface RecurringActivityModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreated: (result: CreateTemplateResult) => void;
  /** Visible week range — passed to the backend so materialize fills the grid. */
  weekFrom?: string;
  weekTo?: string;
}

interface FormState {
  name: string;
  type: ActivityType;
  weekdays: number[]; // ISO 8601 (Mo=1 … Fr=5)
  startTime: string;
  endTime: string;
  roomId: string;
  categoryId: string;
}

const ALL_WEEKDAYS = [1, 2, 3, 4, 5] as const;

const TYPE_OPTIONS: Array<{
  value: ActivityType;
  label: string;
  hint: string;
}> = [
  { value: "care", label: "Betreuung", hint: "Mensa, Lernzeit, Freispiel" },
  { value: "activity", label: "AG", hint: "Yoga, Bouldern, …" },
  { value: "external", label: "Extern", hint: "DAZ, Musikschule" },
];

function emptyForm(): FormState {
  return {
    name: "",
    type: "care",
    weekdays: [...ALL_WEEKDAYS],
    startTime: "12:00",
    endTime: "13:00",
    roomId: "",
    categoryId: "",
  };
}

// The backend uses ISO 8601 weekdays. We render shortcuts (Mo, Di, …)
// using Date objects pinned to a known Monday so getGermanWeekdayShort()
// returns the right label per ISO weekday number.
function weekdayLabel(iso: number): string {
  const ref = new Date(2024, 0, 1); // 2024-01-01 is a Monday → ISO 1
  ref.setDate(ref.getDate() + (iso - 1));
  return getGermanWeekdayShort(ref);
}

export function RecurringActivityModal({
  isOpen,
  onClose,
  onCreated,
  weekFrom,
  weekTo,
}: RecurringActivityModalProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [form, setForm] = useState<FormState>(emptyForm);
  const [rooms, setRooms] = useState<Room[]>([]);
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [loadingRefs, setLoadingRefs] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [validationError, setValidationError] = useState<string | null>(null);

  useEffect(() => {
    if (!isOpen) return;
    setForm(emptyForm());
    setValidationError(null);
    setLoadingRefs(true);

    void Promise.all([
      roomService.getRooms().catch((err: unknown) => {
        logger.error("rooms_fetch_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        return [] as Room[];
      }),
      fetch("/api/activities/categories", { credentials: "include" })
        .then((r) => r.json() as Promise<{ data?: ActivityCategory[] }>)
        .then((j) => j.data ?? [])
        .catch((err: unknown) => {
          logger.error("categories_fetch_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          return [] as ActivityCategory[];
        }),
    ])
      .then(([roomData, categoryData]) => {
        const sortedRooms = [...roomData].sort((a, b) =>
          a.name.localeCompare(b.name, "de"),
        );
        const sortedCategories = [...categoryData].sort((a, b) =>
          a.name.localeCompare(b.name, "de"),
        );
        setRooms(sortedRooms);
        setCategories(sortedCategories);
        // Pre-select first category so the user can submit without an
        // explicit pick when there's an obvious default.
        setForm((prev) =>
          prev.categoryId
            ? prev
            : { ...prev, categoryId: sortedCategories[0]?.id ?? "" },
        );
      })
      .finally(() => setLoadingRefs(false));
  }, [isOpen]);

  const update = <K extends keyof FormState>(key: K, value: FormState[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }));
    setValidationError(null);
  };

  const toggleWeekday = (iso: number) => {
    setForm((prev) => {
      const has = prev.weekdays.includes(iso);
      const next = has
        ? prev.weekdays.filter((d) => d !== iso)
        : [...prev.weekdays, iso].sort((a, b) => a - b);
      return { ...prev, weekdays: next };
    });
    setValidationError(null);
  };

  const canSubmit = useMemo(() => {
    return (
      form.name.trim().length > 0 &&
      form.weekdays.length > 0 &&
      form.startTime !== "" &&
      form.endTime !== "" &&
      form.roomId !== "" &&
      form.categoryId !== "" &&
      !submitting
    );
  }, [form, submitting]);

  const handleSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!canSubmit) return;

    if (form.endTime <= form.startTime) {
      setValidationError("Endzeit muss nach der Startzeit liegen.");
      return;
    }

    const roomId = Number.parseInt(form.roomId, 10);
    const categoryId = Number.parseInt(form.categoryId, 10);
    if (!Number.isFinite(roomId) || roomId <= 0) {
      setValidationError("Bitte einen Raum auswählen.");
      return;
    }
    if (!Number.isFinite(categoryId) || categoryId <= 0) {
      setValidationError("Bitte eine Kategorie auswählen.");
      return;
    }

    setSubmitting(true);
    try {
      const result = await timetableService.createTemplate({
        name: form.name.trim(),
        type: form.type,
        weekdays: form.weekdays,
        start_time: form.startTime,
        end_time: form.endTime,
        room_id: roomId,
        category_id: categoryId,
        materialize_from: weekFrom,
        materialize_to: weekTo,
      });
      const created = result.instancesCreated ?? 0;
      const dayCount = form.weekdays.length;
      const dayWord = dayCount === 1 ? "Wochentag" : "Wochentage";
      toastSuccess(
        created > 0
          ? `Vorlage angelegt — ${created} Aktivität${created === 1 ? "" : "en"} fuer diese Woche eingeplant`
          : `Vorlage "${form.name.trim()}" angelegt (${dayCount} ${dayWord})`,
      );
      onCreated(result);
      onClose();
    } catch (err) {
      logger.error("template_create_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      const msg =
        err instanceof Error
          ? err.message
          : "Vorlage konnte nicht angelegt werden";
      setValidationError(msg);
      toastError(msg);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Wiederkehrende Aktivität anlegen"
      footer={
        <div className="flex items-center justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={onClose}
            disabled={submitting}
          >
            Abbrechen
          </Button>
          <Button
            type="submit"
            form="recurring-activity-form"
            variant="primary"
            size="sm"
            isLoading={submitting}
            loadingText="Lege an …"
            disabled={!canSubmit}
          >
            Vorlage anlegen
          </Button>
        </div>
      }
    >
      <form
        id="recurring-activity-form"
        onSubmit={(e) => void handleSubmit(e)}
        className="flex flex-col gap-4"
      >
        <Field label="Name der Aktivität" htmlFor="name" required>
          <Input
            id="name"
            value={form.name}
            onChange={(e) => update("name", e.target.value)}
            placeholder="z. B. Mensa, Lernzeit 1a, Yoga AG"
            maxLength={255}
            required
            autoFocus
          />
        </Field>

        <div className="flex flex-col gap-1">
          <span className="text-xs font-semibold text-slate-700">
            Typ <span className="ml-0.5 text-[#FF3130]">*</span>
          </span>
          <div className="grid grid-cols-3 gap-2">
            {TYPE_OPTIONS.map((opt) => {
              const isActive = form.type === opt.value;
              const color = getActivityColor(opt.value);
              return (
                <button
                  key={opt.value}
                  type="button"
                  onClick={() => update("type", opt.value)}
                  className={`flex flex-col items-start gap-0.5 rounded-md border px-3 py-2 text-left transition-colors ${
                    isActive
                      ? "border-2 bg-white"
                      : "border border-slate-200 bg-slate-50 hover:bg-white"
                  }`}
                  style={isActive ? { borderColor: color } : undefined}
                >
                  <span
                    className="text-sm font-semibold"
                    style={{ color: isActive ? color : "#374151" }}
                  >
                    {opt.label}
                  </span>
                  <span className="text-[10px] text-slate-500">{opt.hint}</span>
                </button>
              );
            })}
          </div>
        </div>

        <div className="flex flex-col gap-1">
          <span className="text-xs font-semibold text-slate-700">
            Wochentage <span className="ml-0.5 text-[#FF3130]">*</span>
          </span>
          <div className="flex flex-wrap gap-1.5">
            {ALL_WEEKDAYS.map((iso) => {
              const isActive = form.weekdays.includes(iso);
              return (
                <button
                  key={iso}
                  type="button"
                  onClick={() => toggleWeekday(iso)}
                  className={`min-w-[44px] rounded-md border px-3 py-1.5 text-sm font-semibold transition-colors ${
                    isActive
                      ? "border-[#5080D8] bg-[#5080D8] text-white"
                      : "border-slate-300 bg-white text-slate-600 hover:bg-slate-50"
                  }`}
                  aria-pressed={isActive}
                >
                  {weekdayLabel(iso)}
                </button>
              );
            })}
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Startzeit" htmlFor="start" required>
            <Input
              id="start"
              type="time"
              value={form.startTime}
              onChange={(e) => update("startTime", e.target.value)}
              required
            />
          </Field>
          <Field label="Endzeit" htmlFor="end" required>
            <Input
              id="end"
              type="time"
              value={form.endTime}
              onChange={(e) => update("endTime", e.target.value)}
              required
            />
          </Field>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Raum" htmlFor="room" required>
            <select
              id="room"
              value={form.roomId}
              onChange={(e) => update("roomId", e.target.value)}
              required
              disabled={loadingRefs}
              className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] focus:outline-none disabled:bg-gray-100"
            >
              <option value="">
                {loadingRefs ? "Lade Räume …" : "Raum auswählen …"}
              </option>
              {rooms.map((r) => (
                <option key={r.id} value={r.id}>
                  {r.building ? `${r.building} – ${r.name}` : r.name}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Kategorie" htmlFor="category" required>
            <select
              id="category"
              value={form.categoryId}
              onChange={(e) => update("categoryId", e.target.value)}
              required
              disabled={loadingRefs}
              className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] focus:outline-none disabled:bg-gray-100"
            >
              <option value="">
                {loadingRefs ? "Lade Kategorien …" : "Kategorie wählen …"}
              </option>
              {categories.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.name}
                </option>
              ))}
            </select>
          </Field>
        </div>

        {weekFrom && weekTo && (
          <div className="rounded-md border border-[#BAE6FD] bg-[#F0F9FF] px-3 py-2 text-xs text-[#075985]">
            Die Vorlage wird sofort für die aktuell angezeigte Woche
            materialisiert — die Termine erscheinen direkt im Wochenplan.
          </div>
        )}

        {validationError && (
          <div
            role="alert"
            className="rounded-md border border-[#FECACA] bg-[#FEF2F2] px-3 py-2 text-xs font-semibold text-[#7F1D1D]"
          >
            {validationError}
          </div>
        )}
      </form>
    </Modal>
  );
}

interface FieldProps {
  label: string;
  htmlFor: string;
  required?: boolean;
  children: React.ReactNode;
}

function Field({ label, htmlFor, required = false, children }: FieldProps) {
  return (
    <label htmlFor={htmlFor} className="flex flex-col gap-1">
      <span className="text-xs font-semibold text-slate-700">
        {label}
        {required && <span className="ml-0.5 text-[#FF3130]">*</span>}
      </span>
      {children}
    </label>
  );
}
