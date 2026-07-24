import { Search } from "lucide-react";
import { useMemo, useState } from "react";

import { Checkbox } from "~/components/ui/checkbox";
import { CustomSelect } from "~/components/ui/custom-select";
import { getSchoolYear } from "~/lib/student-helpers";

import {
  timetableMutedSurface,
  timetableNestedSurface,
  timetableSearchClass,
} from "../timetable-style";
import type { PersonOption } from "./form-model";

const FORM_SEARCH_CLASS = timetableSearchClass;

export function MultiSelectField({
  label,
  options,
  value,
  onChange,
  metadata,
  bulkOptions,
}: {
  label: string;
  options: PersonOption[];
  value: string[];
  onChange: (value: string[]) => void;
  metadata: "student" | "staff";
  /**
   * Whole-cohort entries (e.g. "Klasse 1a") rendered as a select in the
   * action row; choosing one unions its memberIds into the selection.
   */
  bulkOptions?: Array<{ key: string; label: string; memberIds: string[] }>;
}) {
  const [query, setQuery] = useState("");
  const [gradeFilter, setGradeFilter] = useState("all");
  const [classFilter, setClassFilter] = useState("all");
  const [groupFilter, setGroupFilter] = useState("all");
  const selected = useMemo(() => new Set(value), [value]);
  const normalizedQuery = query.trim().toLocaleLowerCase("de");

  const classOptions = useMemo(
    () =>
      Array.from(
        new Set(
          options
            .map((option) => option.schoolClass?.trim())
            .filter((item): item is string => Boolean(item)),
        ),
      ).sort((a, b) => a.localeCompare(b, "de")),
    [options],
  );

  const gradeOptions = useMemo(
    () =>
      Array.from(
        new Set(
          options
            .map((option) => getSchoolYear(option.schoolClass?.trim() ?? ""))
            .filter((item): item is string => Boolean(item)),
        ),
      ).sort((a, b) => a.localeCompare(b, "de", { numeric: true })),
    [options],
  );

  const groupOptions = useMemo(
    () =>
      Array.from(
        new Set(
          options
            .map((option) => option.groupName?.trim())
            .filter((item): item is string => Boolean(item)),
        ),
      ).sort((a, b) => a.localeCompare(b, "de")),
    [options],
  );

  const filteredOptions = useMemo(
    () =>
      options.filter((option) => {
        const matchesQuery =
          normalizedQuery === "" ||
          [option.name, option.schoolClass, option.groupName]
            .filter(Boolean)
            .join(" ")
            .toLocaleLowerCase("de")
            .includes(normalizedQuery);
        const matchesClass =
          classFilter === "all" || option.schoolClass?.trim() === classFilter;
        const matchesGrade =
          gradeFilter === "all" ||
          getSchoolYear(option.schoolClass?.trim() ?? "") === gradeFilter;
        const matchesGroup =
          groupFilter === "all" || option.groupName?.trim() === groupFilter;
        return matchesQuery && matchesGrade && matchesClass && matchesGroup;
      }),
    [classFilter, gradeFilter, groupFilter, normalizedQuery, options],
  );

  const toggle = (id: string) => {
    const next = selected.has(id)
      ? value.filter((item) => item !== id)
      : [...value, id];
    onChange(next);
  };
  const visibleIds = filteredOptions.map((option) => option.id);
  const selectedVisibleCount = visibleIds.filter((id) =>
    selected.has(id),
  ).length;
  const allVisibleSelected =
    visibleIds.length > 0 && selectedVisibleCount === visibleIds.length;
  const hasFilters =
    query.trim() !== "" ||
    gradeFilter !== "all" ||
    classFilter !== "all" ||
    groupFilter !== "all";

  const selectVisible = () => {
    const next = Array.from(new Set([...value, ...visibleIds]));
    onChange(next);
  };

  const clearVisible = () => {
    const visibleSet = new Set(visibleIds);
    onChange(value.filter((id) => !visibleSet.has(id)));
  };

  return (
    <div className={`${timetableMutedSurface} flex flex-col gap-2 p-3`}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-xs font-semibold text-gray-700">{label}</span>
        <span className="text-[11px] text-gray-500">
          {value.length} ausgewählt
        </span>
      </div>

      <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-[1fr_auto_auto_auto]">
        <label className="relative">
          <span className="sr-only">{label} suchen</span>
          <Search
            className="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-gray-400"
            aria-hidden
          />
          <input
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={`${label} suchen …`}
            className={FORM_SEARCH_CLASS}
          />
        </label>
        {metadata === "student" && gradeOptions.length > 0 && (
          <CustomSelect
            value={gradeFilter}
            options={[
              { value: "all", label: "Alle Jahrgänge" },
              ...gradeOptions.map((gradeLevel) => ({
                value: gradeLevel,
                label: `Jahrgang ${gradeLevel}`,
              })),
            ]}
            onChange={setGradeFilter}
            ariaLabel="Nach Jahrgang filtern"
          />
        )}
        {metadata === "student" && classOptions.length > 0 && (
          <CustomSelect
            value={classFilter}
            options={[
              { value: "all", label: "Alle Klassen" },
              ...classOptions.map((schoolClass) => ({
                value: schoolClass,
                label: schoolClass,
              })),
            ]}
            onChange={setClassFilter}
            ariaLabel="Nach Klasse filtern"
          />
        )}
        {metadata === "student" && groupOptions.length > 0 && (
          <CustomSelect
            value={groupFilter}
            options={[
              { value: "all", label: "Alle Gruppen" },
              ...groupOptions.map((groupName) => ({
                value: groupName,
                label: groupName,
              })),
            ]}
            onChange={setGroupFilter}
            ariaLabel="Nach Gruppe filtern"
          />
        )}
      </div>

      <div className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={allVisibleSelected ? clearVisible : selectVisible}
          disabled={visibleIds.length === 0}
          className="rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-xs font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-100 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
        >
          {allVisibleSelected ? "Sichtbare abwählen" : "Sichtbare auswählen"}
        </button>
        {bulkOptions && bulkOptions.length > 0 && (
          // Pinned to "" so picking an entry adds its members and the
          // select snaps back to the placeholder.
          <CustomSelect
            value=""
            options={bulkOptions.map((option) => ({
              value: option.key,
              label: option.label,
            }))}
            onChange={(next) => {
              const entry = bulkOptions.find((option) => option.key === next);
              if (!entry) return;
              onChange(Array.from(new Set([...value, ...entry.memberIds])));
            }}
            ariaLabel="Jahrgang, Klasse oder Gruppe komplett hinzufügen"
            placeholder="Jahrgang/Klasse/Gruppe komplett hinzufügen …"
            triggerClassName="border-gray-200 bg-white font-medium text-gray-700 hover:border-gray-300 hover:bg-gray-100"
          />
        )}
        {value.length > 0 && (
          <button
            type="button"
            onClick={() => onChange([])}
            className="rounded-lg px-2.5 py-1.5 text-xs font-medium text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            Auswahl leeren
          </button>
        )}
        {hasFilters && (
          <button
            type="button"
            onClick={() => {
              setQuery("");
              setGradeFilter("all");
              setClassFilter("all");
              setGroupFilter("all");
            }}
            className="rounded-lg px-2.5 py-1.5 text-xs font-medium text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            Filter zurücksetzen
          </button>
        )}
      </div>

      <div className={`${timetableNestedSurface} max-h-72 overflow-y-auto p-2`}>
        {options.length === 0 ? (
          <div className="px-2 py-3 text-xs text-gray-500">
            Keine Einträge gefunden
          </div>
        ) : filteredOptions.length === 0 ? (
          <div className="px-2 py-3 text-xs text-gray-500">
            Keine passenden Einträge gefunden
          </div>
        ) : (
          <div className="grid gap-1 sm:grid-cols-2 lg:grid-cols-3">
            {filteredOptions.map((option) => (
              <label
                key={option.id}
                className="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-sm text-gray-700 [contain-intrinsic-size:auto_36px] [content-visibility:auto] hover:bg-gray-50"
              >
                <Checkbox
                  checked={selected.has(option.id)}
                  onChange={() => toggle(option.id)}
                />
                <span className="min-w-0 flex-1">
                  <span className="block truncate">{option.name}</span>
                  {(option.schoolClass || option.groupName) && (
                    <span className="block truncate text-[11px] text-gray-400">
                      {[option.schoolClass, option.groupName]
                        .filter(Boolean)
                        .join(" · ")}
                    </span>
                  )}
                </span>
              </label>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
