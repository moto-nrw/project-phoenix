"use client";

import { useEffect, useState } from "react";
import { ArchiveRestore, Pencil, Plus } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ColorPickerField } from "~/components/ui/color-picker-field";
import { Input } from "~/components/ui/input";
import { ConfirmationModal, Modal } from "~/components/ui/modal";
import { Textarea } from "~/components/ui/textarea";
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
  readonly onChanged: (created?: ActivityCategory) => void | Promise<void>;
}

type View = "list" | "form";

function describeError(err: unknown, fallback: string): string {
  if (err instanceof CategoryApiError && err.detail) return err.detail;
  if (err instanceof Error && err.message) return err.message;
  return fallback;
}

function usageLabel(category: ActivityCategory): string {
  const count = category.usageCount ?? 0;
  if (count === 0) return "Noch nicht verwendet";
  if (count === 1) return "In 1 Termin/Aktivität verwendet";
  return `In ${count} Terminen/Aktivitäten verwendet`;
}

const categoryListClass =
  "divide-y divide-gray-100 rounded-2xl border border-gray-200";

/**
 * One row of the category list: colour dot, name, usage hint, and whatever
 * actions the section offers. Archived rows drop the dot and mute the name.
 */
function CategoryRow({
  category,
  archived = false,
  children,
}: {
  readonly category: ActivityCategory;
  readonly archived?: boolean;
  readonly children: React.ReactNode;
}) {
  return (
    <li className="flex items-center gap-3 px-3 py-2.5">
      {!archived && (
        <span
          className="h-4 w-4 shrink-0 rounded-full border border-black/10"
          style={{
            backgroundColor: category.color ?? LOCATION_COLORS.UNKNOWN,
          }}
          aria-hidden="true"
        />
      )}
      <div className="min-w-0 flex-1">
        <span
          className={`block truncate font-medium ${archived ? "text-gray-500" : "text-gray-900"}`}
        >
          {category.name}
        </span>
        <p className="truncate text-xs text-gray-500">{usageLabel(category)}</p>
      </div>
      {children}
    </li>
  );
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
      setCategories(await categoryService.getManagedCategories());
    } catch (err: unknown) {
      logger.error("categories_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setLoadError(true);
    } finally {
      setLoading(false);
    }
  };

  /** Clears the form back to "new category" defaults. */
  const resetForm = () => {
    setEditing(null);
    setName("");
    setDescription("");
    setColor(DEFAULT_NEW_COLOR);
    setError(null);
  };

  useEffect(() => {
    if (!isOpen) return;
    resetForm();
    setView(initialView === "create" ? "form" : "list");
    void reload();
    // reload/resetForm only touch setters and are stable enough for this
    // dialog: it resets its state on every open, which is exactly when we
    // want a fresh fetch.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, initialView]);

  const active = categories.filter((category) => !category.archivedAt);
  const archived = categories.filter((category) => category.archivedAt);

  const openCreate = () => {
    resetForm();
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

  /**
   * Runs one write against the category API with the shared busy/error
   * contract: block the buttons, clear the previous error, log and surface a
   * German message on failure.
   */
  const runWrite = async (
    event: string,
    fallback: string,
    categoryId: string | undefined,
    write: () => Promise<void>,
  ) => {
    setBusy(true);
    setError(null);
    try {
      await write();
    } catch (err: unknown) {
      logger.error(event, {
        category_id: categoryId,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(describeError(err, fallback));
    } finally {
      setBusy(false);
    }
  };

  const handleSave = async () => {
    const trimmed = name.trim();
    if (!trimmed) {
      setError("Name ist erforderlich");
      return;
    }
    const payload = {
      name: trimmed,
      description: description.trim(),
      color: color ?? "",
    };

    await runWrite(
      "category_save_failed",
      "Kategorie konnte nicht gespeichert werden",
      editing?.id,
      async () => {
        if (editing) {
          await categoryService.updateCategory(editing.id, payload);
          await reload();
          await onChanged();
          setView("list");
          return;
        }
        // No reload and no view switch: the caller closes the dialog as soon
        // as a category was created (it gets selected in the Termin), so both
        // would be thrown away along with the fetch they cost.
        await onChanged(await categoryService.createCategory(payload));
      },
    );
  };

  const handleArchive = async (category: ActivityCategory) => {
    await runWrite(
      "category_archive_failed",
      "Kategorie konnte nicht archiviert werden",
      category.id,
      async () => {
        await categoryService.archiveCategory(category.id);
        await reload();
        onChanged();
      },
    );
    // After, not before: the confirmation keeps showing its spinner
    // (isConfirmLoading={busy}) while the archive is in flight.
    setArchiveTarget(null);
  };

  const handleRestore = async (category: ActivityCategory) => {
    await runWrite(
      "category_restore_failed",
      "Kategorie konnte nicht wiederhergestellt werden",
      category.id,
      async () => {
        await categoryService.restoreCategory(category.id);
        await reload();
        onChanged();
      },
    );
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
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <p className="max-w-lg text-gray-600">
                Kategorien ordnen Termine und Aktivitäten ein, zum Beispiel
                Essenszeiten, Lernzeit oder Freispiel. Sie gelten nur für diese
                OGS.
              </p>
              <Button
                type="button"
                variant="outline"
                size="md"
                className="shrink-0"
                onClick={openCreate}
              >
                <Plus className="mr-1.5 h-4 w-4" />
                Neue Kategorie
              </Button>
            </div>

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
                  <ul className={categoryListClass}>
                    {active.map((category) => (
                      <CategoryRow key={category.id} category={category}>
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
                      </CategoryRow>
                    ))}
                  </ul>
                )}

                {archived.length > 0 && (
                  <div>
                    <p className="mb-2 text-xs font-semibold text-gray-500 uppercase">
                      Archiviert
                    </p>
                    <ul className={categoryListClass}>
                      {archived.map((category) => (
                        <CategoryRow
                          key={category.id}
                          category={category}
                          archived
                        >
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
                        </CategoryRow>
                      ))}
                    </ul>
                  </div>
                )}
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

            <Textarea
              name="category-description"
              label="Beschreibung (optional)"
              value={description}
              maxLength={DESCRIPTION_MAX_LENGTH}
              rows={2}
              onChange={(e) => setDescription(e.target.value)}
            />

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
        onConfirm={() => archiveTarget && void handleArchive(archiveTarget)}
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
