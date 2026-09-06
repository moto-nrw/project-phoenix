"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Pencil, Plus, Trash2 } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ColorPickerField } from "~/components/ui/color-picker-field";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { Input } from "~/components/ui/input";
import { MultiCheckboxSelect } from "~/components/ui/multi-checkbox-select";
import {
  SlideOver,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { getCategories } from "~/lib/activity-api";
import type { ActivityCategory } from "~/lib/activity-helpers";
import { getApiErrorMessage } from "~/lib/api-error-message";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import { ShiftTypeApiError, shiftTypeService } from "~/lib/shift-type-api";
import type { ShiftType } from "~/lib/shift-type-helpers";

const logger = createLogger({ component: "ShiftTypeManageModal" });
const DEFAULT_NEW_COLOR = LOCATION_COLORS.GROUP_ROOM;

interface ShiftTypeManageModalProps {
  readonly isOpen: boolean;
  readonly shiftTypes: readonly ShiftType[];
  /** True while shift types are still being fetched (no data yet). */
  readonly isLoading: boolean;
  /** True when the shift-type fetch failed. */
  readonly loadError: boolean;
  readonly onClose: () => void;
  /** Called after any create/update/delete so the caller can refetch. */
  readonly onChanged: () => void;
}

type View = "list" | "form";

// Manage tenant shift types (Schichtarten, #1836): a list with inline
// create/edit form and delete confirmation, opened from the Dienstplan.
export function ShiftTypeManageModal({
  isOpen,
  shiftTypes,
  isLoading,
  loadError,
  onClose,
  onChanged,
}: ShiftTypeManageModalProps) {
  const [view, setView] = useState<View>("list");
  const [editing, setEditing] = useState<ShiftType | null>(null);
  const [name, setName] = useState("");
  const [color, setColor] = useState<string | null>(DEFAULT_NEW_COLOR);
  const [description, setDescription] = useState("");
  const [isActive, setIsActive] = useState(true);
  const [categoryIds, setCategoryIds] = useState<string[]>([]);
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  // Whether the category list has loaded successfully. Until it has, the
  // category_ids payload is omitted so a pending/failed load can never send an
  // empty array that clears existing mappings (#1837 follow-up).
  const [categoriesReady, setCategoriesReady] = useState(false);
  const [busy, setBusy] = useState(false);
  const [seeding, setSeeding] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<ShiftType | null>(null);

  useEffect(() => {
    if (!isOpen) return;
    setView("list");
    setEditing(null);
    setError(null);
    setDeleteTarget(null);
  }, [isOpen]);

  // Monotonic token so only the most recently started category load applies
  // its result. Without it a slow initial load could resolve AFTER a post-save
  // refresh and overwrite fresh mappings with a stale snapshot (#1837 review).
  const loadTokenRef = useRef(0);

  // Load the Timetable-Kategorien so they can be mapped to a shift type. Each
  // category carries its current shift_type_id, which drives the form's
  // preselection (#1837 follow-up). While loading (or after a failure) the
  // list is cleared and categoriesReady is false, so the selector is disabled
  // and category_ids is omitted from saves — never sending stale options.
  const loadCategories = useCallback(async () => {
    const token = loadTokenRef.current + 1;
    loadTokenRef.current = token;
    setCategoriesReady(false);
    setCategories([]);
    try {
      const list = await getCategories();
      if (loadTokenRef.current !== token) return; // superseded by a newer load
      setCategories(list);
      setCategoriesReady(true);
    } catch (err: unknown) {
      if (loadTokenRef.current !== token) return;
      logger.error("shift_type_categories_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setCategories([]);
      setCategoriesReady(false);
    }
  }, []);

  useEffect(() => {
    if (!isOpen) return;
    void loadCategories();
  }, [isOpen, loadCategories]);

  // Preselect the categories currently mapped to the shift type being edited
  // (empty for a new shift type). Re-runs when categories finish loading so a
  // fast "open → edit" cannot miss the preselection.
  useEffect(() => {
    if (view !== "form") return;
    setCategoryIds(
      editing
        ? categories
            .filter((category) => category.shiftTypeId === editing.id)
            .map((category) => category.id)
        : [],
    );
  }, [view, editing, categories]);

  const openCreate = () => {
    setEditing(null);
    setName("");
    setColor(DEFAULT_NEW_COLOR);
    setDescription("");
    setIsActive(true);
    setError(null);
    setView("form");
  };

  const openEdit = (type: ShiftType) => {
    setEditing(type);
    setName(type.name);
    setColor(type.color);
    setDescription(type.description);
    setIsActive(type.isActive);
    setError(null);
    setView("form");
  };

  const describeError = (err: unknown, action: "speichern" | "löschen") => {
    if (err instanceof ShiftTypeApiError) {
      if (err.status === 409) {
        return "Es gibt bereits eine Schichtart mit diesem Namen.";
      }
      if (err.status === 400) {
        return "Ungültige Eingabe. Bitte prüfe Name und Farbe.";
      }
      return err.detail || `Schichtart konnte nicht ${action} werden.`;
    }
    return getApiErrorMessage(
      err,
      action,
      "Schichtart",
      `Schichtart konnte nicht ${action} werden.`,
    );
  };

  const handleSave = async () => {
    const trimmed = name.trim();
    if (trimmed === "") {
      setError("Bitte gib einen Namen ein.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const payload = {
        name: trimmed,
        color: color ?? DEFAULT_NEW_COLOR,
        description: description.trim(),
        isActive,
        // Only manage the mapping when the category list is known. If it is
        // still loading or failed to load, omit category_ids entirely so the
        // backend leaves existing links untouched (never send an empty array
        // that would clear them).
        categoryIds: categoriesReady ? categoryIds : undefined,
      };
      if (editing) {
        await shiftTypeService.updateShiftType(editing.id, payload);
      } else {
        await shiftTypeService.createShiftType(payload);
      }
      onChanged();
      // The save rewrote category.shift_type_id server-side. Refresh the local
      // categories through the same tokened loader so a subsequent edit in the
      // same open modal preselects from fresh mappings. loadCategories clears
      // categoriesReady until the reload succeeds, so a failed refresh leaves
      // the mapping state stale-but-guarded (a later save omits category_ids
      // rather than clearing links that were just written, #1837).
      await loadCategories();
      setView("list");
    } catch (err: unknown) {
      logger.error("shift_type_save_failed", {
        editing_id: editing?.id ?? null,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(describeError(err, "speichern"));
    } finally {
      setBusy(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setBusy(true);
    try {
      await shiftTypeService.deleteShiftType(deleteTarget.id);
      setDeleteTarget(null);
      onChanged();
    } catch (err: unknown) {
      logger.error("shift_type_delete_failed", {
        shift_type_id: deleteTarget.id,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(describeError(err, "löschen"));
      setDeleteTarget(null);
    } finally {
      setBusy(false);
    }
  };

  const handleSeedDefaults = async () => {
    setSeeding(true);
    setError(null);
    try {
      await shiftTypeService.createDefaults();
      onChanged();
    } catch (err: unknown) {
      logger.error("shift_type_seed_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(describeError(err, "speichern"));
    } finally {
      setSeeding(false);
    }
  };

  const listFooter = (
    <div className="flex w-full justify-end">
      <Button type="button" variant="outline" size="md" onClick={onClose}>
        Schließen
      </Button>
    </div>
  );

  const formFooter = (
    <div className="flex w-full flex-col-reverse gap-2 sm:flex-row sm:justify-end">
      <Button
        type="button"
        variant="outline"
        size="md"
        disabled={busy}
        onClick={() => setView("list")}
      >
        Abbrechen
      </Button>
      <Button
        type="button"
        variant="primary"
        size="md"
        isLoading={busy}
        loadingText="Speichern…"
        onClick={handleSave}
      >
        {editing ? "Änderungen speichern" : "Schichtart anlegen"}
      </Button>
    </div>
  );

  return (
    <>
      <SlideOver
        open={isOpen && !deleteTarget}
        onOpenChange={(next) => {
          if (!next) onClose();
        }}
      >
        <SlideOverContent widthClass="sm:w-[560px]">
          <SlideOverHeader className="flex-row items-start justify-between gap-3">
            <div className="min-w-0">
              <SlideOverTitle>
                {view === "form"
                  ? editing
                    ? "Schichtart bearbeiten"
                    : "Neue Schichtart"
                  : "Schichtarten verwalten"}
              </SlideOverTitle>
            </div>
            <SlideOverCloseButton />
          </SlideOverHeader>
          <div className="flex-1 overflow-y-auto px-5 py-4">
            {view === "list" ? (
              <div className="space-y-4 text-sm">
                <p className="text-gray-600">
                  Schichtarten kennzeichnen die Aufgabe einer geplanten Schicht
                  (z. B. Betreuung, Vorbereitung, Vertretung, Pause). Die Farbe
                  macht sie im Wochenplan sofort unterscheidbar.
                </p>

                {loadError ? (
                  <Alert
                    type="error"
                    message="Die Schichtarten konnten nicht geladen werden. Bitte schließe den Dialog und lade die Seite neu."
                  />
                ) : isLoading ? (
                  <div className="rounded-2xl border border-dashed border-gray-200 bg-gray-50 px-4 py-8 text-center text-sm text-gray-500">
                    Schichtarten werden geladen…
                  </div>
                ) : shiftTypes.length === 0 ? (
                  <div className="rounded-2xl border border-dashed border-gray-200 bg-gray-50 px-4 py-8 text-center">
                    <p className="text-sm text-gray-600">
                      Noch keine Schichtarten angelegt.
                    </p>
                    <div className="mt-4 flex flex-col justify-center gap-2 sm:flex-row">
                      <Button
                        type="button"
                        variant="primary"
                        size="md"
                        isLoading={seeding}
                        loadingText="Wird angelegt…"
                        onClick={handleSeedDefaults}
                      >
                        Beispiele hinzufügen
                      </Button>
                      <Button
                        type="button"
                        variant="outline"
                        size="md"
                        onClick={openCreate}
                      >
                        Eigene Schichtart
                      </Button>
                    </div>
                  </div>
                ) : (
                  <>
                    <ul className="divide-y divide-gray-100 rounded-2xl border border-gray-200">
                      {shiftTypes.map((type) => (
                        <li
                          key={type.id}
                          className="flex items-center gap-3 px-3 py-2.5"
                        >
                          <span
                            className="h-4 w-4 shrink-0 rounded-full border border-black/10"
                            style={{ backgroundColor: type.color }}
                            aria-hidden="true"
                          />
                          <div className="min-w-0 flex-1">
                            <div className="flex items-center gap-2">
                              <span className="truncate font-medium text-gray-900">
                                {type.name}
                              </span>
                              {!type.isActive && (
                                <span className="shrink-0 rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-500">
                                  Inaktiv
                                </span>
                              )}
                            </div>
                            {type.description && (
                              <p className="truncate text-xs text-gray-500">
                                {type.description}
                              </p>
                            )}
                          </div>
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            aria-label={`${type.name} bearbeiten`}
                            onClick={() => openEdit(type)}
                          >
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            aria-label={`${type.name} löschen`}
                            onClick={() => setDeleteTarget(type)}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </li>
                      ))}
                    </ul>
                    <div className="flex flex-col gap-2 sm:flex-row sm:justify-between">
                      <Button
                        type="button"
                        variant="outline"
                        size="md"
                        isLoading={seeding}
                        loadingText="Wird angelegt…"
                        onClick={handleSeedDefaults}
                      >
                        Beispiele hinzufügen
                      </Button>
                      <Button
                        type="button"
                        variant="outline"
                        size="md"
                        onClick={openCreate}
                      >
                        <Plus className="mr-1.5 h-4 w-4" />
                        Neue Schichtart
                      </Button>
                    </div>
                  </>
                )}

                {error && <Alert type="error" message={error} />}
              </div>
            ) : (
              <div className="space-y-4 text-sm">
                <Input
                  name="shift-type-name"
                  label="Name*"
                  value={name}
                  maxLength={100}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="z. B. Betreuung / Zeit am Kind"
                />

                <ColorPickerField
                  value={color}
                  onChange={setColor}
                  label="Farbe"
                  fallbackColor={DEFAULT_NEW_COLOR}
                  helpText="Die Farbe kennzeichnet diese Schichtart im Wochenplan."
                />

                <label htmlFor="shift-type-description" className="block">
                  <span className="mb-2 block text-sm font-medium text-gray-700">
                    Beschreibung (optional)
                  </span>
                  <textarea
                    id="shift-type-description"
                    value={description}
                    maxLength={500}
                    rows={2}
                    onChange={(e) => setDescription(e.target.value)}
                    className="block w-full resize-none rounded-lg border-0 bg-white px-4 py-3 text-base text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-400"
                  />
                </label>

                <div>
                  <span
                    id="shift-type-categories-label"
                    className="mb-2 block text-sm font-medium text-gray-700"
                  >
                    Timetable-Kategorien (optional)
                  </span>
                  <MultiCheckboxSelect
                    ariaLabel="Timetable-Kategorien"
                    value={categoryIds}
                    onChange={setCategoryIds}
                    disabled={!categoriesReady}
                    options={categories.map((category) => ({
                      value: category.id,
                      label: category.name,
                    }))}
                    emptyLabel="Keine Kategorie zugeordnet"
                    unavailableLabel="Kategorien werden geladen…"
                    multipleLabel={(count) => `${count} Kategorien`}
                    searchable
                    searchPlaceholder="Kategorie suchen…"
                  />
                  {!categoriesReady && (
                    <p className="mt-1 text-xs text-gray-500">
                      Kategorien werden geladen … Namen, Farbe und Status lassen
                      sich bereits speichern; die Kategorie-Zuordnung bleibt
                      dabei unverändert.
                    </p>
                  )}
                  <p className="mt-1 text-xs text-gray-500">
                    Ordnet Betreuungsblöcke dieser Kategorien im Stundenplan
                    dieser Schichtart zu. Eine Kategorie kann nur einer
                    Schichtart zugeordnet sein.
                  </p>
                </div>

                <label
                  htmlFor="shift-type-active"
                  className="flex cursor-pointer items-center gap-2.5"
                >
                  <Checkbox
                    id="shift-type-active"
                    checked={isActive}
                    onChange={(e) => setIsActive(e.target.checked)}
                  />
                  <span className="text-sm text-gray-700">
                    Aktiv (zur Auswahl in neuen Schichten)
                  </span>
                </label>

                {error && <Alert type="error" message={error} />}
              </div>
            )}
          </div>
          <SlideOverFooter className="flex-row justify-end gap-2">
            {view === "form" ? formFooter : listFooter}
          </SlideOverFooter>
        </SlideOverContent>
      </SlideOver>

      <ConfirmDeleteModal
        isOpen={deleteTarget !== null}
        title="Schichtart löschen"
        description={
          <>
            Die Schichtart <strong>{deleteTarget?.name}</strong> wird gelöscht.
            Bereits geplante Schichten bleiben erhalten, verlieren aber diese
            Kennzeichnung.
          </>
        }
        gate={{ mode: "twoStep" }}
        loading={busy}
        error=""
        onConfirm={handleDelete}
        onClose={() => setDeleteTarget(null)}
      />
    </>
  );
}
