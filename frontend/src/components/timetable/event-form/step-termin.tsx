"use client";

import { Repeat } from "lucide-react";

import { CustomSelect } from "~/components/ui/custom-select";
import { ISODatePicker } from "~/components/ui/date-picker";
import { Input } from "~/components/ui/input";
import type { ActivityCategory } from "~/lib/activity-helpers";
import type { PlanningTrack } from "~/lib/planning-track-api";
import { getActivityColor } from "~/lib/timetable-helpers";
import {
  timetableRequiredMark,
  timetableTextAreaClass,
} from "../timetable-style";
import { Field } from "./field";
import { isoWeekday } from "./form-model";
import type { EventFormState } from "./form-model";
import type { RoomOption } from "./use-event-form";
import type { ActivityType, TimetableListKind } from "~/lib/timetable-types";
import { PlanningTrackSelect } from "../planning-track-select";

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
  planningTracks?: PlanningTrack[];
  loadingRefs: boolean;
  expanded: boolean;
  isSeriesFlow: boolean;
  /**
   * True when a stored Regeltermin is edited. The Datum then does not move
   * the series start (#2135) — the hint under the field says so.
   */
  isEditingSeries: boolean;
  /**
   * Present when the edited series has not started yet (#2226): the stored
   * Serienbeginn (`original`, the upper bound) plus the earliest pickable
   * date (`min` — today, clamped to the period start). Renders the
   * Serienbeginn field so the start can be pulled to an earlier date.
   */
  seriesStartEdit?: { original: string; min: string } | null;
  quickPreset: string;
  // Flipped true the moment the user changes Listenart, so an all/following
  // series edit writes the new value instead of echoing the fetched template
  // (#1565 review).
  listKindTouched: React.RefObject<boolean>;
  /** Whether category catalog controls may be shown to this user. */
  canManageCategories: boolean;
  /** Opens the Kategorien-verwalten dialog, either on the list or straight in the create form. */
  onManageCategories: (mode: "list" | "create") => void;
  canManagePlanningTracks?: boolean;
  onPlanningTracksChanged?: (created?: PlanningTrack) => void | Promise<void>;
}

/**
 * Sentinel option value for "+ Neue Kategorie anlegen". Picked so it can never
 * collide with a real category id (those are numeric strings).
 */
export const CREATE_CATEGORY_OPTION = "__create_category__";

const isWeekendDay = (date: Date) => date.getDay() === 0 || date.getDay() === 6;
const EMPTY_PLANNING_TRACKS: PlanningTrack[] = [];
const NOOP_PLANNING_TRACKS_CHANGED = () => undefined;

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
  planningTracks = EMPTY_PLANNING_TRACKS,
  loadingRefs,
  expanded,
  isSeriesFlow,
  isEditingSeries,
  seriesStartEdit = null,
  quickPreset,
  listKindTouched,
  canManageCategories,
  onManageCategories,
  canManagePlanningTracks = false,
  onPlanningTracksChanged = NOOP_PLANNING_TRACKS_CHANGED,
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
            action={
              canManageCategories ? (
                <button
                  type="button"
                  onClick={() => onManageCategories("list")}
                  className="text-xs font-medium text-gray-600 underline underline-offset-2 hover:text-gray-900"
                >
                  Verwalten
                </button>
              ) : undefined
            }
          >
            <CustomSelect
              id="event_category"
              ariaLabel="Kategorie"
              ariaDescribedBy={
                fieldErrors.categoryId ? "event_category_error" : undefined
              }
              value={form.categoryId}
              options={[
                {
                  value: "",
                  label: loadingRefs
                    ? "Lade Kategorien …"
                    : "Kategorie wählen …",
                },
                ...categories.map((category) => ({
                  value: category.id,
                  label: category.name,
                })),
                ...(canManageCategories
                  ? [
                      // Last entry, so the missing category can be created without
                      // leaving the Termin (#2131 — the reported Essenszeiten case).
                      {
                        value: CREATE_CATEGORY_OPTION,
                        label: "+ Neue Kategorie anlegen",
                      },
                    ]
                  : []),
              ]}
              onChange={(next) => {
                if (next === CREATE_CATEGORY_OPTION) {
                  onManageCategories("create");
                  return;
                }
                update("categoryId", next);
              }}
              required
              disabled={loadingRefs}
              invalid={Boolean(fieldErrors.categoryId)}
              placeholder={
                loadingRefs ? "Lade Kategorien …" : "Kategorie wählen …"
              }
            />
          </Field>

          <Field label="Planungsspur" htmlFor="event_planning_track">
            <PlanningTrackSelect
              value={form.planningTrackId}
              tracks={planningTracks}
              onChange={(next) => update("planningTrackId", next)}
              onTracksChanged={onPlanningTracksChanged}
              canManage={canManagePlanningTracks}
              disabled={loadingRefs}
            />
            <p className="mt-1 text-[11px] leading-4 text-gray-500">
              Optional. Farbe und Reihenfolge gelten im Betreuungsplan.
            </p>
          </Field>
        </>
      )}

      {/* Listenart (#1565): visible in every flow that writes an occurrence's
          own list_kind — a one-off create, a standalone-instance edit, and the
          "Nur diesen Termin" scope (all persist form.listKind via instanceBody)
          — plus the expanded series flow (seriesBody). An "Alle/folgende" series
          edit still echoes the template's classification and ignores this field.
          Without this the field only rendered under expanded && isSeriesFlow, so
          spontaneous/one-off slots could not be classified nor an occurrence
          override cleared (#1565 review pass 1 P2). */}
      {(!isSeriesFlow || expanded) && (
        <Field label="Listenart" htmlFor="event_list_kind">
          <CustomSelect
            id="event_list_kind"
            ariaLabel="Listenart"
            value={form.listKind}
            options={[
              { value: "", label: "Keine" },
              ...LIST_KIND_OPTIONS.map((option) => ({
                value: option.value,
                label: option.label,
              })),
            ]}
            onChange={(next) => {
              listKindTouched.current = true;
              update("listKind", next as EventFormState["listKind"]);
            }}
          />
          <p className="mt-1 text-[11px] leading-4 text-gray-500">
            Ordnet den Termin einer druckbaren Tagesliste zu (Planung →
            Tageslisten).
          </p>
        </Field>
      )}

      <Field
        label="Raum"
        htmlFor="event_room"
        required
        error={fieldErrors.roomId}
      >
        <CustomSelect
          id="event_room"
          ariaLabel="Raum"
          ariaDescribedBy={fieldErrors.roomId ? "event_room_error" : undefined}
          value={form.roomId}
          options={[
            {
              value: "",
              label: loadingRefs ? "Lade Räume …" : "Raum auswählen …",
            },
            ...rooms.map((room) => ({
              value: String(room.id),
              label: room.building
                ? `${room.building} - ${room.name}`
                : room.name,
            })),
          ]}
          onChange={(next) => update("roomId", next)}
          disabled={loadingRefs}
          required
          invalid={Boolean(fieldErrors.roomId)}
          placeholder={loadingRefs ? "Lade Räume …" : "Raum auswählen …"}
        />
      </Field>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <Field label="Datum" htmlFor="event_date" required>
          <ISODatePicker
            id="event_date"
            controlSize="md"
            value={form.date}
            error={fieldErrors.date}
            calendarLayout="popover"
            disabledDay={isWeekendDay}
            onChange={(nextDate) => {
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
          />
          {isSeriesFlow && !isEditingSeries && (
            <p className="mt-1 text-[11px] leading-4 text-gray-500">
              Die Serie beginnt an diesem Datum. Frühere Termine werden nicht
              angelegt.
            </p>
          )}
          {isEditingSeries && (
            <p className="mt-1 text-[11px] leading-4 text-gray-500">
              {seriesStartEdit
                ? "Den Serienbeginn ändern Sie unten im Feld „Serienbeginn“."
                : "Der Serienbeginn bleibt unverändert."}
            </p>
          )}
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

      {isEditingSeries && seriesStartEdit && (
        <Field
          label="Serienbeginn"
          htmlFor="event_series_start"
          error={fieldErrors.seriesStartDate}
        >
          <ISODatePicker
            id="event_series_start"
            controlSize="md"
            value={form.seriesStartDate}
            min={seriesStartEdit.min}
            max={seriesStartEdit.original}
            invalid={Boolean(fieldErrors.seriesStartDate)}
            calendarLayout="popover"
            disabledDay={isWeekendDay}
            onChange={(next) => update("seriesStartDate", next)}
          />
          <p className="mt-1 text-[11px] leading-4 text-gray-500">
            Die Serie hat noch nicht begonnen. Sie können den Beginn auf ein
            früheres Datum vorziehen; neue Termine entstehen dann ab diesem
            Datum an den gewählten Wochentagen. Ein späterer Beginn ist nicht
            möglich.
          </p>
        </Field>
      )}

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
            <div className="border-moto-blue/30 bg-moto-blue/10 rounded-lg border p-3">
              <div className="text-moto-blue flex items-center gap-1.5 text-xs font-medium">
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
