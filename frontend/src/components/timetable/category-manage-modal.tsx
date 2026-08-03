"use client";

import { useEffect, useState } from "react";
import { ArchiveRestore, Pencil, Plus } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ColorPickerField } from "~/components/ui/color-picker-field";
import { Input } from "~/components/ui/input";
import { ConfirmationModal, Modal } from "~/components/ui/modal";
import type { ActivityCategory } from "~/lib/activity-helpers";
import { categoryService, CategoryApiError } from "~/lib/category-api";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CategoryManageModal" });
const DEFAULT_NEW_COLOR = LOCATION_COLORS.GROUP_ROOM;

/** Mirrors maxCategoryNameLength / maxCategoryDescriptionLength in Go. */
const NAME_MAX_LENGTH = 60;
const DESCRIPTION_MAX_LENGTH = 255;

interface CategoryManageModalProps {
  readonly isOpen: boolean;
  /**
   * Where the dialog opens. "create" jumps straight into the empty form —
   * that is the "+ Neue Kategorie anlegen" entry in the Kategorie dropdown,
   * where the user already knows what is missing.
   */
  readonly initialView?: "list" | "create";
  readonly onClose: () => void;
  /**
   * Called after every write so the caller can refetch its category list.
   * `created` is set only when a new category was just added, so the Termin
   * form can select it right away.
   */
  readonly onChanged: (created?: ActivityCategory) => void;
}

type View = "list" | "form";

function describeError(err: unknown, fallback: string): string {
  if (err instanceof CategoryApiError && err.detail) return err.detail;
  if (err instanceof Error && err.message) return err.message;
  return fallback;
}

/**
 * Manage the school's Timetable-Kategorien (#2131) without leaving the Termin
 * form: list, create, rename, archive, restore. Same shape as
 * ShiftTypeManageModal in the Dienstplan — a small tenant taxonomy is managed
 * where it is used, not in a separate Datenverwaltung screen.
 *
 * System categories (Schulhof, WC) never appear here: the backend refuses
 * every write on them, so listing them would only offer dead ends.
 */
export function CategoryManageModal({
  isOpen,
  initialView = "list",
  onClose,
  onChanged,
}: CategoryManageModalProps) {
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState(false);

  const [view, setView] = useState<View>("list");
  const [editing, setEditing] = useState<ActivityCategory | null>(null);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [color, setColor] = useState<string | null>(DEFAULT_NEW_COLOR);

  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [archiveTarget, setArchiveTarget] = useState<ActivityCategory | null>(
    null,
  );

  const reload = async () => {
    setLoading(true);
    setLoadError(false);
    try {
      setCategories(await categoryService.getCategories(true));
    } catch (err: unknown) {
      logger.error("categories_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setLoadError(true);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!isOpen) return;
    setError(null);
    setEditing(null);
    setName("");
    setDescription("");
    setColor(DEFAULT_NEW_COLOR);
    setView(initialView === "create" ? "form" : "list");
    void reload();
    // reload is stable enough for this dialog: it closes and remounts its
    // state on every open, which is exactly when we want a fresh fetch.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, initialView]);

  const active = categories.filter((category) => !category.archivedAt);
  const archived = categories.filter((category) => category.archivedAt);

  const openCreate = () => {
    setEditing(null);
    setName("");
    setDescription("");
    setColor(DEFAULT_NEW_COLOR);
    setError(null);
    setView("form");
  };

  const openEdit = (category: ActivityCategory) => {
    setEditing(category);
    setName(category.name);
    setDescription(category.description ?? "");
    setColor(category.color ?? null);
    setError(null);
    setView("form");
  };

  const handleSave = async () => {
    const trimmed = name.trim();
    if (!trimmed) {
      setError("Name ist erforderlich");
      return;
    }

    setBusy(true);
    setError(null);
    try {
      const payload = {
        name: trimmed,
        description: description.trim(),
        color: color ?? "",
      };
      if (editing) {
        await categoryService.updateCategory(editing.id, payload);
        await reload();
        onChanged();
      } else {
        const created = await categoryService.createCategory(payload);
        await reload();
        onChanged(created);
      }
      setView("list");
    } catch (err: unknown) {
      logger.error("category_save_failed", {
        category_id: editing?.id,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(describeError(err, "Kategorie konnte nicht gespeichert werden"));
    } finally {
      setBusy(false);
    }
  };

  const handleArchive = async () => {
    if (!archiveTarget) return;
    setBusy(true);
    try {
      await categoryService.archiveCategory(archiveTarget.id);
      setArchiveTarget(null);
      await reload();
      onChanged();
    } catch (err: unknown) {
      logger.error("category_archive_failed", {
        category_id: archiveTarget.id,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(describeError(err, "Kategorie konnte nicht archiviert werden"));
      setArchiveTarget(null);
    } finally {
      setBusy(false);
    }
  };

  const handleRestore = async (category: ActivityCategory) => {
    setBusy(true);
    setError(null);
    try {
      await categoryService.restoreCategory(category.id);
      await reload();
      onChanged();
    } catch (err: unknown) {
      logger.error("category_restore_failed", {
        category_id: category.id,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        describeError(err, "Kategorie konnte nicht wiederhergestellt werden"),
      );
    } finally {
      setBusy(false);
    }
  };

  const usageLabel = (category: ActivityCategory): string => {
    const count = category.usageCount ?? 0;
    if (count === 0) return "Noch nicht verwendet";
    if (count === 1) return "In 1 Termin/Aktivität verwendet";
    return `In ${count} Terminen/Aktivitäten verwendet`;
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
        onClick={() => (initialView === "create" ? onClose() : setView("list"))}
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
        {editing ? "Änderungen speichern" : "Anlegen und auswählen"}
      </Button>
    </div>
  );

  return (
    <>
      <Modal
        isOpen={isOpen && !archiveTarget}
        onClose={onClose}
        title={
          view === "form"
            ? editing
              ? "Kategorie bearbeiten"
              : "Neue Kategorie"
            : "Kategorien verwalten"
        }
        footer={view === "form" ? formFooter : listFooter}
      >
        {view === "list" ? (
          <div className="space-y-4 text-sm">
            <p className="text-gray-600">
              Kategorien ordnen Termine und Aktivitäten ein, zum Beispiel
              Essenszeiten, Lernzeit oder Freispiel. Sie gelten nur für diese
              OGS.
            </p>

            {loadError ? (
              <Alert
                type="error"
                message="Die Kategorien konnten nicht geladen werden. Bitte schließe den Dialog und lade die Seite neu."
              />
            ) : loading ? (
              <div className="rounded-2xl border border-dashed border-gray-200 bg-gray-50 px-4 py-8 text-center text-sm text-gray-500">
                Kategorien werden geladen…
              </div>
            ) : (
              <>
                {active.length === 0 ? (
                  <div className="rounded-2xl border border-dashed border-gray-200 bg-gray-50 px-4 py-8 text-center text-sm text-gray-600">
                    Noch keine Kategorien angelegt.
                  </div>
                ) : (
                  <ul className="divide-y divide-gray-100 rounded-2xl border border-gray-200">
                    {active.map((category) => (
                      <li
                        key={category.id}
                        className="flex items-center gap-3 px-3 py-2.5"
                      >
                        <span
                          className="h-4 w-4 shrink-0 rounded-full border border-black/10"
                          style={{
                            backgroundColor:
                              category.color ?? LOCATION_COLORS.UNKNOWN,
                          }}
                          aria-hidden="true"
                        />
                        <div className="min-w-0 flex-1">
                          <span className="block truncate font-medium text-gray-900">
                            {category.name}
                          </span>
                          <p className="truncate text-xs text-gray-500">
                            {usageLabel(category)}
                          </p>
                        </div>
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon"
                          aria-label={`${category.name} bearbeiten`}
                          onClick={() => openEdit(category)}
                        >
                          <Pencil className="h-4 w-4" />
                        </Button>
                        <Button
                          type="button"
                          variant="ghost"
                          size="compact"
                          onClick={() => setArchiveTarget(category)}
                        >
                          Archivieren
                        </Button>
                      </li>
                    ))}
                  </ul>
                )}

                {archived.length > 0 && (
                  <div>
                    <p className="mb-2 text-xs font-semibold text-gray-500 uppercase">
                      Archiviert
                    </p>
                    <ul className="divide-y divide-gray-100 rounded-2xl border border-gray-200">
                      {archived.map((category) => (
                        <li
                          key={category.id}
                          className="flex items-center gap-3 px-3 py-2.5"
                        >
                          <div className="min-w-0 flex-1">
                            <span className="block truncate font-medium text-gray-500">
                              {category.name}
                            </span>
                            <p className="truncate text-xs text-gray-500">
                              {usageLabel(category)}
                            </p>
                          </div>
                          <Button
                            type="button"
                            variant="ghost"
                            size="compact"
                            disabled={busy}
                            onClick={() => void handleRestore(category)}
                          >
                            <ArchiveRestore className="mr-1.5 h-4 w-4" />
                            Wiederherstellen
                          </Button>
                        </li>
                      ))}
                    </ul>
                  </div>
                )}

                <div className="flex justify-end">
                  <Button
                    type="button"
                    variant="outline"
                    size="md"
                    onClick={openCreate}
                  >
                    <Plus className="mr-1.5 h-4 w-4" />
                    Neue Kategorie
                  </Button>
                </div>
              </>
            )}

            {error && <Alert type="error" message={error} />}
          </div>
        ) : (
          <div className="space-y-4 text-sm">
            <Input
              name="category-name"
              label="Name*"
              value={name}
              maxLength={NAME_MAX_LENGTH}
              onChange={(e) => setName(e.target.value)}
              placeholder="z. B. Essen"
              autoFocus
            />

            <ColorPickerField
              value={color}
              onChange={setColor}
              label="Farbe"
              fallbackColor={DEFAULT_NEW_COLOR}
              helpText="Die Farbe kennzeichnet die Kategorie im Betreuungsplan."
            />

            <label htmlFor="category-description" className="block">
              <span className="mb-2 block text-sm font-medium text-gray-700">
                Beschreibung (optional)
              </span>
              <textarea
                id="category-description"
                value={description}
                maxLength={DESCRIPTION_MAX_LENGTH}
                rows={2}
                onChange={(e) => setDescription(e.target.value)}
                className="block w-full resize-none rounded-lg border-0 bg-white px-4 py-3 text-base text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-400"
              />
            </label>

            {error && <Alert type="error" message={error} />}
          </div>
        )}
      </Modal>

      {/* Archiving is reversible, so this is a plain confirmation rather than
          the gated ConfirmDeleteModal used for real deletions. */}
      <ConfirmationModal
        isOpen={archiveTarget !== null}
        title="Kategorie archivieren?"
        confirmText="Archivieren"
        cancelText="Abbrechen"
        isConfirmLoading={busy}
        onConfirm={() => void handleArchive()}
        onClose={() => setArchiveTarget(null)}
      >
        <p className="text-sm text-gray-700">
          Die Kategorie <strong>{archiveTarget?.name}</strong> wird für neue
          Termine und Aktivitäten nicht mehr angeboten. Bestehende Einträge
          behalten sie und bleiben gültig.
        </p>
      </ConfirmationModal>
    </>
  );
}
