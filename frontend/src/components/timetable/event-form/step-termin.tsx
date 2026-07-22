"use client";

import { Repeat } from "lucide-react";

import { Input } from "~/components/ui/input";
import type { ActivityCategory } from "~/lib/activity-helpers";
import { getActivityColor } from "~/lib/timetable-helpers";
import {
  timetableRequiredMark,
  timetableSelectClass,
  timetableTextAreaClass,
} from "../timetable-style";
import { Field } from "./field";
import { isoWeekday } from "./form-model";
import type { EventFormState } from "./form-model";
import type { RoomOption } from "./use-event-form";
import type { ActivityType, TimetableListKind } from "~/lib/timetable-types";

const FORM_SELECT_CLASS = timetableSelectClass;

const TYPE_OPTIONS: Array<{
  value: ActivityType;
  label: string;
  hint: string;
}> = [
  { value: "care", label: "Betreuung", hint: "Mensa, Lernzeit, Freispiel" },
  { value: "activity", label: "AG", hint: "Yoga, Bouldern, …" },
  { value: "external", label: "Extern", hint: "DAZ, Musikschule" },
];

// Listenart (#1565): optional classification driving the printable
// Tageslisten (Planung → Tageslisten).
const LIST_KIND_OPTIONS: Array<{
  value: TimetableListKind;
  label: string;
}> = [
  { value: "edge_hours", label: "Randstunden" },
  { value: "learning_time", label: "Lernzeit" },
  { value: "activity", label: "AG-Angebote" },
  { value: "mensa", label: "Mensa" },
];

export interface StepTerminProps {
  form: EventFormState;
  update: <K extends keyof EventFormState>(
    key: K,
    value: EventFormState[K],
  ) => void;
  fieldErrors: Record<string, string>;
  rooms: RoomOption[];
  categories: ActivityCategory[];
  loadingRefs: boolean;
  expanded: boolean;
  isSeriesFlow: boolean;
  quickPreset: string;
}

/**
 * Wizard step 1 "Termin": the fields that make an event savable on their own —
 * Titel, Typ/Kategorie, Raum, Datum, Zeiten and the Notizen. Everything here is
 * moved verbatim out of the previous single-page form; the visibility rule
 * `expanded && isSeriesFlow` on Typ/Kategorie is unchanged, only its render
 * location moved.
 */
export function StepTermin({
  form,
  update,
  fieldErrors,
  rooms,
  categories,
  loadingRefs,
  expanded,
  isSeriesFlow,
  quickPreset,
}: Readonly<StepTerminProps>) {
  return (
    <>
      <Field label="Titel" htmlFor="event_title" required>
        <Input
          id="event_title"
          value={form.title}
          onChange={(event) => update("title", event.target.value)}
          placeholder="z. B. Mensa, Lernzeit 1a, Yoga AG"
          maxLength={255}
          controlSize="compact"
          error={fieldErrors.title}
          autoFocus
          required
        />
      </Field>

      {expanded && isSeriesFlow && (
        <>
          <div className="flex flex-col gap-1">
            <span className="text-xs font-semibold text-gray-700">
              Typ <span className={timetableRequiredMark}>*</span>
            </span>
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-3">
              {TYPE_OPTIONS.map((option) => {
                const isActive = form.type === option.value;
                const color = getActivityColor(option.value);
                return (
                  <button
                    key={option.value}
                    type="button"
                    onClick={() => update("type", option.value)}
                    className={`flex flex-col items-start gap-0.5 rounded-lg border border-l-[3px] px-3 py-2 text-left shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                      isActive
                        ? "border-gray-300 bg-white"
                        : "border-gray-200 bg-white hover:bg-gray-50"
                    }`}
                    style={{ borderLeftColor: color }}
                  >
                    <span
                      className="text-sm font-semibold"
                      style={{ color: isActive ? color : "#374151" }}
                    >
                      {option.label}
                    </span>
                    <span className="text-[10px] text-gray-500">
                      {option.hint}
                    </span>
                  </button>
                );
              })}
            </div>
          </div>

          <Field
            label="Kategorie"
            htmlFor="event_category"
            required
            error={fieldErrors.categoryId}
          >
            <select
              id="event_category"
              value={form.categoryId}
              onChange={(event) => update("categoryId", event.target.value)}
              required
              disabled={loadingRefs}
              aria-invalid={fieldErrors.categoryId ? true : undefined}
              aria-describedby={
                fieldErrors.categoryId ? "event_category_error" : undefined
              }
              className={FORM_SELECT_CLASS}
            >
              <option value="">
                {loadingRefs ? "Lade Kategorien …" : "Kategorie wählen …"}
              </option>
              {categories.map((category) => (
                <option key={category.id} value={category.id}>
                  {category.name}
                </option>
              ))}
            </select>
          </Field>

          <Field label="Listenart" htmlFor="event_list_kind">
            <select
              id="event_list_kind"
              value={form.listKind}
              onChange={(event) =>
                update(
                  "listKind",
                  event.target.value as EventFormState["listKind"],
                )
              }
              className={FORM_SELECT_CLASS}
            >
              <option value="">Keine</option>
              {LIST_KIND_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            <p className="mt-1 text-[11px] leading-4 text-gray-500">
              Ordnet den Termin einer druckbaren Tagesliste zu (Planung →
              Tageslisten).
            </p>
          </Field>
        </>
      )}

      <Field
        label="Raum"
        htmlFor="event_room"
        required
        error={fieldErrors.roomId}
      >
        <select
          id="event_room"
          value={form.roomId}
          onChange={(event) => update("roomId", event.target.value)}
          disabled={loadingRefs}
          required
          aria-invalid={fieldErrors.roomId ? true : undefined}
          aria-describedby={fieldErrors.roomId ? "event_room_error" : undefined}
          className={FORM_SELECT_CLASS}
        >
          <option value="">
            {loadingRefs ? "Lade Räume …" : "Raum auswählen …"}
          </option>
          {rooms.map((room) => (
            <option key={room.id} value={room.id}>
              {room.building ? `${room.building} - ${room.name}` : room.name}
            </option>
          ))}
        </select>
      </Field>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <Field label="Datum" htmlFor="event_date" required>
          <Input
            id="event_date"
            type="date"
            value={form.date}
            controlSize="compact"
            error={fieldErrors.date}
            onChange={(event) => {
              const nextDate = event.target.value;
              const nextWeekday = isoWeekday(nextDate);
              update("date", nextDate);
              // One-off events follow the date; the quick preset
              // "Wöchentlich am <Tag>" retargets to the new weekday.
              const followsDate =
                !isSeriesFlow ||
                (!expanded && quickPreset === "woechentlich-am");
              if (followsDate && nextWeekday >= 1 && nextWeekday <= 5) {
                update("weekdays", [nextWeekday]);
              }
            }}
            required
          />
        </Field>
        <Field label="Start" htmlFor="event_start" required>
          <Input
            id="event_start"
            type="time"
            value={form.startTime}
            controlSize="compact"
            error={fieldErrors.startTime}
            onChange={(event) => update("startTime", event.target.value)}
            required
          />
        </Field>
        <Field label="Ende" htmlFor="event_end" required>
          <Input
            id="event_end"
            type="time"
            value={form.endTime}
            controlSize="compact"
            error={fieldErrors.endTime}
            onChange={(event) => update("endTime", event.target.value)}
            required
          />
        </Field>
      </div>

      {isSeriesFlow ? (
        <Field label="Wochennotiz" htmlFor="event_series_notes">
          <textarea
            id="event_series_notes"
            value={form.seriesNotes}
            onChange={(event) => update("seriesNotes", event.target.value)}
            rows={3}
            className={timetableTextAreaClass}
            placeholder="z. B. Raum erst ab 14 Uhr offen"
          />
          <p className="mt-1 text-xs text-gray-500">
            Gilt dauerhaft für die ganze Terminreihe und erscheint an jedem
            Termin. Bleibt bei Re-Plan und Serienänderungen erhalten.
          </p>
        </Field>
      ) : (
        <>
          {form.seriesNotes.trim() !== "" && (
            <div className="rounded-lg border border-[#5080D8]/30 bg-[#5080D8]/10 p-3">
              <div className="flex items-center gap-1.5 text-xs font-medium text-[#5080D8]">
                <Repeat className="h-3.5 w-3.5" aria-hidden="true" />
                Wochennotiz der Terminreihe
              </div>
              <p className="mt-1 text-sm whitespace-pre-wrap text-gray-700">
                {form.seriesNotes}
              </p>
              <p className="mt-1 text-xs text-gray-500">
                Wird über den Regeltermin gepflegt und gilt für alle Termine.
              </p>
            </div>
          )}
          <Field label="Tagesnotiz" htmlFor="event_notes">
            <textarea
              id="event_notes"
              value={form.notes}
              onChange={(event) => update("notes", event.target.value)}
              rows={3}
              className={timetableTextAreaClass}
              placeholder="Nur für diesen einen Termin"
            />
          </Field>
        </>
      )}
    </>
  );
}
