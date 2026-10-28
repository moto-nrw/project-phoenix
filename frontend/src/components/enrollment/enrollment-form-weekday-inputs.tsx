// The per-weekday fields of the OGS-Anmeldung (enrollment-form.tsx): pickup
// times (`weekday_schedule`), day toggles such as Buskind or Abholregelung
// (`weekday_boolean`), one way home per day (`weekday_mode`) and the allowed
// ways home per day with the coupled "mit wem" note (`weekday_multi_mode`).

import { useEffect, useId, useState } from "react";

import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { CustomSelect } from "~/components/ui/custom-select";
import { RequiredMark } from "~/components/ui/field-label";
import { Input } from "~/components/ui/input";
import { ConfirmationModal } from "~/components/ui/modal";
import { Radio } from "~/components/ui/radio";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { ToggleChip } from "~/components/ui/toggle-chip";

import {
  asScheduleObject,
  asStringMap,
  asWeekdayBooleanObject,
  asWeekdayModeObject,
  asWeekdayMultiModeObject,
  departureMultiModeOptionsForField,
  WEEKDAYS,
  type CustomFieldInputProps,
  type DepartureModeValue,
} from "./enrollment-form-shared";

// Single-mode legacy departure field (target student.departure). The
// accompanied mode is intentionally NOT offered here: it requires the coupled
// "mit wem" companion note, which is only captured + required-validated on the
// multi-mode student.allowed_departure_modes field. Offering it here would let
// a parent pick an accompanied departure with no companion note, defeating the
// data requirement of #1694.
const DEPARTURE_MODE_OPTIONS: ReadonlyArray<DepartureModeValue> = [
  "alone",
  "bus",
  "pickup",
];

export function WeekdayScheduleInput({
  field,
  inputName,
  value,
  onChange,
  error,
  tr,
  relevantDays,
  careConstrained,
}: CustomFieldInputProps) {
  const sched = asScheduleObject(value);
  const weekdayLabels = asStringMap(tr.raw("weekdays"));
  // Only offer the weekdays the child is actually in care on (derived from the
  // selected care offerings). When no offerings constrain the form, relevantDays
  // is undefined and all weekdays are shown (historical behaviour).
  const daysToRender = relevantDays ?? WEEKDAYS;
  // When the field configures fixed pickup times, parents pick from a
  // dropdown limited to those values per weekday instead of a free time
  // input. Empty list = free entry (the historical behaviour).
  const allowedTimes = field.allowed_times ?? [];
  return (
    <fieldset
      className={`rounded-lg border p-3 ${error ? "border-moto-red" : "border-gray-200"}`}
      aria-invalid={error ? "true" : undefined}
    >
      <legend className="px-1 text-xs font-medium text-gray-700">
        {field.label}
        {field.required && <RequiredMark />}
      </legend>
      {field.help_text && (
        <p className="mt-1 text-xs leading-5 text-gray-500">
          {field.help_text}
        </p>
      )}
      {daysToRender.length === 0 ? (
        <p className="mt-1 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-xs text-gray-600">
          {tr("structured.weekdayMultiModeNoDays")}
        </p>
      ) : (
        <>
          <p className="text-xs text-gray-500">
            {tr(
              !field.required
                ? // Optional: blanks are accepted (validation skips the field).
                  "structured.emptySchedule"
                : careConstrained
                  ? // Required + care-constrained: every shown care day needs a time.
                    "structured.everyDayTime"
                  : // Required + all weekdays shown: at least one time is enough
                    // (matches the errors.schedule gate, not a per-day demand).
                    "structured.atLeastOneTime",
            )}
          </p>
          <div className="mt-2 grid gap-2 sm:grid-cols-5">
            {daysToRender.map((weekday) => {
              const current = sched[weekday] ?? "";
              const dayLabel = weekdayLabels[weekday] ?? weekday;
              if (allowedTimes.length === 0) {
                return (
                  <Input
                    key={weekday}
                    id={`${inputName}-${weekday}`}
                    label={dayLabel}
                    type="time"
                    controlSize="compact"
                    value={current}
                    onChange={(e) =>
                      onChange({ ...sched, [weekday]: e.target.value })
                    }
                  />
                );
              }
              // A previously saved value that is no longer in the configured
              // list (admin changed the times after the draft was saved) must
              // still be shown, otherwise the dropdown silently blanks it and
              // the parent never sees that their time was dropped. It is
              // flagged "nicht mehr verfügbar" so the parent notices it has to
              // be re-picked before submit — the backend rejects it anyway.
              const isStale =
                Boolean(current) && !allowedTimes.includes(current);
              return (
                <label key={weekday} className="block text-xs">
                  <span className="block text-gray-600">{dayLabel}</span>
                  <CustomSelect
                    value={current}
                    onChange={(selected) =>
                      onChange({ ...sched, [weekday]: selected })
                    }
                    ariaLabel={`${dayLabel} – ${field.label}`}
                    placeholder={tr("structured.selectTime")}
                    invalid={Boolean(error) || isStale}
                    className="mt-1 border-gray-200 bg-white"
                    options={[
                      { value: "", label: tr("structured.selectTime") },
                      ...allowedTimes.map((time) => ({
                        value: time,
                        label: time,
                      })),
                      ...(isStale
                        ? [
                            {
                              value: current,
                              label: `${current} (${tr("structured.timeUnavailable")})`,
                            },
                          ]
                        : []),
                    ]}
                  />
                </label>
              );
            })}
          </div>
        </>
      )}
      {error && <p className="text-moto-red mt-2 text-xs">{error}</p>}
    </fieldset>
  );
}

export function WeekdayBooleanInput({
  field,
  value,
  onChange,
  error,
  tr,
}: CustomFieldInputProps) {
  const days = asWeekdayBooleanObject(value);
  const weekdayLabels = asStringMap(tr.raw("weekdaysShort"));
  return (
    <fieldset
      className={`rounded-lg border p-3 ${error ? "border-moto-red" : "border-gray-200"}`}
      aria-invalid={error ? "true" : undefined}
    >
      <legend className="px-1 text-xs font-medium text-gray-700">
        {field.label}
        {field.required && <RequiredMark />}
      </legend>
      {field.help_text && (
        <p className="mt-1 text-xs leading-5 text-gray-500">
          {field.help_text}
        </p>
      )}
      <p className="text-xs text-gray-500">
        {field.target === "student.pickup_status"
          ? tr("structured.pickupDaysHelp")
          : tr("structured.busDaysHelp")}
      </p>
      <div className="mt-2 grid gap-2 sm:grid-cols-5">
        {WEEKDAYS.map((w) => {
          const checked = Boolean(days[w]);
          return (
            <ChoiceTile
              key={w}
              selected={checked}
              className="h-10 gap-2 px-2 py-0 text-xs"
            >
              <Checkbox
                checked={checked}
                onChange={(e) => onChange({ ...days, [w]: e.target.checked })}
              />
              {weekdayLabels[w] ?? w}
            </ChoiceTile>
          );
        })}
      </div>
      {error && <p className="text-moto-red mt-2 text-xs">{error}</p>}
    </fieldset>
  );
}

export function WeekdayModeInput({
  field,
  value,
  onChange,
  error,
  tr,
}: CustomFieldInputProps) {
  const modes = asWeekdayModeObject(value);
  const weekdayLabels = asStringMap(tr.raw("weekdaysShort"));
  const departureModeLabels = asStringMap(tr.raw("structured.departureModes"));
  const modeFor = (key: string): DepartureModeValue => {
    const m = modes[key];
    return m === "bus" || m === "pickup" || m === "accompanied" ? m : "alone";
  };
  return (
    <fieldset
      className={`rounded-lg border p-3 ${error ? "border-moto-red" : "border-gray-200"}`}
      aria-invalid={error ? "true" : undefined}
    >
      <legend className="px-1 text-xs font-medium text-gray-700">
        {field.label}
        {field.required && <RequiredMark />}
      </legend>
      {field.help_text && (
        <p className="mt-1 text-xs leading-5 text-gray-500">
          {field.help_text}
        </p>
      )}
      <p className="text-xs text-gray-500">
        {tr("structured.departureModeHelp")}
      </p>
      <div className="mt-2 space-y-2">
        {WEEKDAYS.map((w) => {
          const current = modeFor(w);
          return (
            <div key={w} className="flex items-center gap-2">
              <span className="w-20 shrink-0 text-xs font-medium text-gray-600">
                {weekdayLabels[w] ?? w}
              </span>
              <SegmentedControl
                className="flex-1"
                fullWidth
                ariaLabel={`Heimweg ${weekdayLabels[w] ?? w}`}
                value={current}
                onChange={(mode) => {
                  const next = { ...modes };
                  if (mode === "alone") delete next[w];
                  else next[w] = mode;
                  onChange(next);
                }}
                items={DEPARTURE_MODE_OPTIONS.map((mode) => ({
                  value: mode,
                  label: departureModeLabels[mode] ?? mode,
                }))}
              />
            </div>
          );
        })}
      </div>
      {error && <p className="text-moto-red mt-2 text-xs">{error}</p>}
    </fieldset>
  );
}

export function WeekdayMultiModeInput({
  field,
  value,
  onChange,
  error,
  tr,
  relevantDays,
  companionNote,
  onCompanionNoteChange,
  companionNoteError,
  companionNoteName,
  singleMode,
}: CustomFieldInputProps) {
  const daysToRender = relevantDays ?? WEEKDAYS;
  const modeOptions = departureMultiModeOptionsForField(field);
  const modes = asWeekdayMultiModeObject(value, daysToRender, modeOptions);
  // Instance-unique radio-group prefix for single mode: field.key repeats
  // across children, so naming groups by key alone would couple the
  // siblings' radios in the same form.
  const radioGroupId = useId();
  const [expandedDays, setExpandedDays] = useState<Set<string>>(
    () => new Set(daysToRender.filter((day) => (modes[day]?.length ?? 0) > 0)),
  );
  // Opt-in convenience: pick the allowed ways home once and apply them to
  // every care day. The stored value stays per-day (each day gets the same
  // array), so backend, validation, and pruning are untouched. Always starts
  // in the per-day view; the parent flips it on explicitly via the toggle.
  const [uniform, setUniform] = useState(false);
  // The shared selection while uniform is on. This is the source of truth for
  // the uniform block (rather than re-deriving it from the per-day payload),
  // so the choice survives even when the entire care-day set is swapped out
  // from under it -- a payload-derived value would read empty once its source
  // days disappear and silently reset to nothing.
  const [uniformSelection, setUniformSelection] = useState<
    DepartureModeValue[]
  >([]);
  // Open only when enabling uniform mode would discard divergent per-day
  // choices (see toggleUniform).
  const [confirmUniformOpen, setConfirmUniformOpen] = useState(false);
  const weekdayLabels = asStringMap(tr.raw("weekdaysShort"));
  const departureModeLabels = asStringMap(tr.raw("structured.departureModes"));
  const activeDays = daysToRender.filter(
    (day) => expandedDays.has(day) || (modes[day]?.length ?? 0) > 0,
  );
  const toggleDay = (day: string) => {
    const isActive = activeDays.includes(day);
    const nextExpanded = new Set(expandedDays);
    if (isActive) {
      nextExpanded.delete(day);
      const nextRaw = { ...modes };
      delete nextRaw[day];
      onChange(asWeekdayMultiModeObject(nextRaw, daysToRender, modeOptions));
    } else {
      nextExpanded.add(day);
    }
    setExpandedDays(nextExpanded);
  };
  const updateDay = (day: string, mode: DepartureModeValue) => {
    const current = new Set(modes[day] ?? []);
    if (singleMode) {
      // Heimweg-Beschränkung (#2381): a radio group — picking a mode
      // replaces the previous one, so a day never carries more than one,
      // including stale multi-selections prefilled from before the rule.
      // Clearing a day happens by collapsing it (toggleDay).
      current.clear();
      current.add(mode);
    } else if (current.has(mode)) current.delete(mode);
    else current.add(mode);
    const nextRaw = {
      ...modes,
      [day]: modeOptions.filter((m) => current.has(m)),
    };
    onChange(asWeekdayMultiModeObject(nextRaw, daysToRender, modeOptions));
  };
  // The payload-derived shared selection: the first *populated* day's modes.
  // Used only to decide whether enabling uniform is lossless and, if so, to
  // seed the uniform state from the existing per-day data. The live uniform
  // block reads `uniformSelection` (state), not this.
  const payloadModes =
    daysToRender.map((day) => modes[day]).find((m) => m && m.length > 0) ?? [];
  // Switching into uniform mode is lossless only when every care day already
  // holds the identical selection (including all-empty); the shared block then
  // simply adopts that selection. join keys compare the pruned arrays.
  const allDaysIdentical = daysToRender.every(
    (day) => (modes[day] ?? []).join(",") === payloadModes.join(","),
  );
  // Fan the shared selection across the current care days whenever the payload
  // falls out of sync with it -- e.g. a care day is added/removed elsewhere on
  // the form after uniform was enabled. A newly added day starts empty, so
  // without this the uniform block would claim coverage the payload doesn't
  // have. Driven by string-signature deps (not the fresh-every-render `modes`
  // /`daysToRender`/`onChange` references), so it fires only on a real change
  // -- the selection, the day set, or the payload -- never on every render.
  // The needsSync guard keeps it idempotent and self-terminating.
  const uniformKey = uniformSelection.join(",");
  const daysKey = daysToRender.join(",");
  const modesKey = daysToRender
    .map((day) => `${day}:${(modes[day] ?? []).join("|")}`)
    .join(";");
  useEffect(() => {
    if (!uniform) return;
    const needsSync = daysToRender.some(
      (day) => (modes[day] ?? []).join(",") !== uniformKey,
    );
    if (!needsSync) return;
    const nextRaw: Record<string, DepartureModeValue[]> = {};
    for (const day of daysToRender) nextRaw[day] = uniformSelection;
    onChange(asWeekdayMultiModeObject(nextRaw, daysToRender, modeOptions));
    // String-signature deps keep the effect from re-running every render while
    // still reacting to every meaningful change. `modes`/`daysToRender`/
    // `onChange`/`uniformSelection` are read from the current render's closure;
    // they change identity every render, so depending on them directly would
    // defeat the point of the signatures.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [uniform, uniformKey, daysKey, modesKey]);
  const enableUniform = () => {
    setExpandedDays(new Set());
    setUniformSelection([]);
    setUniform(true);
    // Clear divergent per-day data so the parent makes one fresh selection;
    // the shared block then fans it out to every day via updateUniform.
    onChange({});
  };
  const toggleUniform = (next: boolean) => {
    if (!next) {
      setUniform(false);
      return;
    }
    // Enabling over already-uniform data is lossless: adopt the existing
    // selection as the shared state and flip the view.
    if (allDaysIdentical) {
      setUniformSelection(payloadModes);
      setUniform(true);
      return;
    }
    // Enabling over divergent per-day choices would discard them silently;
    // confirm first, then wipe in enableUniform.
    setConfirmUniformOpen(true);
  };
  const updateUniform = (mode: DepartureModeValue) => {
    const current = new Set(uniformSelection);
    if (singleMode) {
      current.clear();
      current.add(mode);
    } else if (current.has(mode)) current.delete(mode);
    else current.add(mode);
    const arr = modeOptions.filter((m) => current.has(m));
    setUniformSelection(arr);
    const nextRaw: Record<string, DepartureModeValue[]> = {};
    for (const day of daysToRender) nextRaw[day] = arr;
    onChange(asWeekdayMultiModeObject(nextRaw, daysToRender, modeOptions));
  };
  // The coupled "mit wem" note (#1694) is shown the moment any care day allows
  // the accompanied mode, so parents can always say which child it is.
  const accompaniedSelected = uniform
    ? uniformSelection.includes("accompanied")
    : daysToRender.some((day) => (modes[day] ?? []).includes("accompanied"));
  return (
    <fieldset
      className={`rounded-lg border p-3 ${error ? "border-moto-red" : "border-gray-200"}`}
      aria-invalid={error ? "true" : undefined}
    >
      <legend className="px-1 text-xs font-medium text-gray-700">
        {field.label}
        {field.required && <RequiredMark />}
      </legend>
      {field.help_text && (
        <p className="mt-1 text-xs leading-5 text-gray-500">
          {field.help_text}
        </p>
      )}
      <p className="text-xs text-gray-500">
        {tr(
          singleMode
            ? "structured.weekdayMultiModeSingleHelp"
            : "structured.weekdayMultiModeHelp",
        )}
      </p>
      {singleMode &&
      daysToRender.some((day) => (modes[day]?.length ?? 0) > 1) ? (
        // Stale multi-selection carried over from before the rule (e.g. a
        // change request on an older submission): the backend rejects it,
        // so warn up front instead of failing on submit.
        <p className="text-moto-red mt-1 text-xs">
          {tr("structured.weekdayMultiModeSingleLimit")}
        </p>
      ) : null}
      <div className="mt-3">
        {daysToRender.length > 0 ? (
          <>
            <label
              className={`flex cursor-pointer items-center gap-2 rounded-lg border bg-white px-3 py-2 text-xs font-medium transition-colors ${
                uniform
                  ? "border-gray-900 text-gray-900"
                  : "border-gray-200 text-gray-700 hover:border-gray-300"
              }`}
            >
              <Checkbox
                checked={uniform}
                onChange={(e) => toggleUniform(e.target.checked)}
              />
              <span>{tr("structured.weekdayMultiModeUniform")}</span>
            </label>
            {uniform ? (
              <div className="mt-3 rounded-lg border border-gray-200 bg-gray-50/70 p-3">
                <p className="mb-2 text-xs text-gray-500">
                  {tr("structured.weekdayMultiModeUniformHint")}
                </p>
                <div className="grid gap-2 sm:grid-cols-2">
                  {modeOptions.map((mode) => {
                    const checked = uniformSelection.includes(mode);
                    return (
                      <label
                        key={mode}
                        className={`flex min-h-10 cursor-pointer items-center gap-2 rounded-lg border bg-white px-3 py-2 text-xs font-medium transition-colors ${
                          checked
                            ? "border-gray-900 text-gray-900"
                            : "border-gray-200 text-gray-700 hover:border-gray-300"
                        }`}
                      >
                        {singleMode ? (
                          // Heimweg-Beschränkung (#2381): a real radio group,
                          // not a checkbox that merely behaves like one.
                          <Radio
                            name={`${radioGroupId}-uniform`}
                            checked={checked}
                            onChange={() => updateUniform(mode)}
                          />
                        ) : (
                          <Checkbox
                            checked={checked}
                            onChange={() => updateUniform(mode)}
                          />
                        )}
                        <span>{departureModeLabels[mode] ?? mode}</span>
                      </label>
                    );
                  })}
                </div>
              </div>
            ) : (
              <>
                <div className="mt-3 grid grid-cols-5 gap-2">
                  {daysToRender.map((day) => (
                    <ToggleChip
                      key={day}
                      pressed={activeDays.includes(day)}
                      onPressedChange={() => toggleDay(day)}
                      className="h-9 px-2 text-xs font-semibold"
                    >
                      {weekdayLabels[day] ?? day}
                    </ToggleChip>
                  ))}
                </div>
                <div className="mt-3 space-y-2">
                  {activeDays.map((day) => (
                    <div
                      key={day}
                      className="rounded-lg border border-gray-200 bg-gray-50/70 p-3"
                    >
                      <div className="mb-2 text-xs font-semibold text-gray-700">
                        {weekdayLabels[day] ?? day}
                      </div>
                      <div className="grid gap-2 sm:grid-cols-2">
                        {modeOptions.map((mode) => {
                          const checked = modes[day]?.includes(mode) ?? false;
                          return (
                            <label
                              key={mode}
                              className={`flex min-h-10 cursor-pointer items-center gap-2 rounded-lg border bg-white px-3 py-2 text-xs font-medium transition-colors ${
                                checked
                                  ? "border-gray-900 text-gray-900"
                                  : "border-gray-200 text-gray-700 hover:border-gray-300"
                              }`}
                            >
                              {singleMode ? (
                                <Radio
                                  name={`${radioGroupId}-${day}`}
                                  checked={checked}
                                  onChange={() => updateDay(day, mode)}
                                />
                              ) : (
                                <Checkbox
                                  checked={checked}
                                  onChange={() => updateDay(day, mode)}
                                />
                              )}
                              <span>{departureModeLabels[mode] ?? mode}</span>
                            </label>
                          );
                        })}
                      </div>
                    </div>
                  ))}
                </div>
              </>
            )}
          </>
        ) : (
          <p className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-xs text-gray-600">
            {tr("structured.weekdayMultiModeNoDays")}
          </p>
        )}
        {onCompanionNoteChange && accompaniedSelected && (
          <div className="mt-3">
            <Input
              label={tr("structured.departureCompanionLabel")}
              name={companionNoteName}
              controlSize="compact"
              required
              value={companionNote ?? ""}
              onChange={(e) => onCompanionNoteChange(e.target.value)}
              placeholder={tr("structured.departureCompanionPlaceholder")}
              maxLength={255}
              error={companionNoteError}
            />
          </div>
        )}
      </div>
      {error && <p className="text-moto-red mt-2 text-xs">{error}</p>}
      <ConfirmationModal
        isOpen={confirmUniformOpen}
        onClose={() => setConfirmUniformOpen(false)}
        onConfirm={() => {
          setConfirmUniformOpen(false);
          enableUniform();
        }}
        title={tr("structured.weekdayMultiModeUniformConfirmTitle")}
        confirmText={tr("structured.weekdayMultiModeUniformConfirmCta")}
        cancelText={tr("structured.weekdayMultiModeUniformConfirmCancel")}
        confirmVariant="danger"
      >
        {tr("structured.weekdayMultiModeUniformConfirmBody")}
      </ConfirmationModal>
    </fieldset>
  );
}
