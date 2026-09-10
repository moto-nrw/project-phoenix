"use client";

// Die Verwaltungsfläche für die kurzen Stammdaten-Listen einer Schule
// (Kategorien, Planungsspuren, Schichtarten, Abwesenheitsarten, #3114).
//
// Vier Flächen hatten dafür vier eigene Bauten: zwei Slide-over mit
// `view: list | form`, ein Popover mit `select | manage | form` und ein
// Auswahlfeld mit Stiftsymbolen an den Zeilen. Das ist die fünfte Bauart, die
// es laut BAUARTEN-SPEC nicht gibt — und aus einem Formular geöffnet stapelte
// sie zusätzlich Ebenen. Statt vier Seiten mit demselben Umriss zu bauen,
// liegt der Umriss hier: Sammlung links (Bauart 1), Objekt rechts (Bauart 2),
// Anlegen als Kopf-Aktion. Eine Route liefert nur noch ihre Daten und ihre
// Felder.

import { useCallback, useMemo, useState } from "react";
import { ChevronDown, ChevronUp } from "lucide-react";

import { DatabaseCreateAction } from "~/components/database/database-create-action";
import { DatabaseDetailHeader } from "~/components/database/database-detail-header";
import { DatabaseListItem } from "~/components/database/database-list-item";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { DetailPanel } from "~/components/database/detail-panel";
import { EmptyDetailState } from "~/components/database/empty-detail-state";
import {
  GroupedList,
  type GroupDefinition,
} from "~/components/database/grouped-list";
import { MasterDetailLayout } from "~/components/database/master-detail-layout";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DatabaseForm } from "~/components/ui/database/database-form";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type { OverflowMenuEntry } from "~/components/ui/page-header/OverflowMenu";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { Skeleton } from "~/components/ui/skeleton";
import { useToast } from "~/contexts/ToastContext";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { configToFormSection, type SectionConfig } from "~/lib/database/types";
import { MOTO_CONCEPTS, type MotoConceptKey } from "~/lib/moto-concepts";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CatalogPage" });

/** URL-Parameter des ausgewählten Eintrags — wie `?room=` in den Räumen. */
export const CATALOG_SELECTION_PARAM = "eintrag";

export interface CatalogItem {
  readonly id: string;
}

/** Die Zeile, die ein Eintrag in der Sammlung wirft. */
export interface CatalogRow {
  readonly name: string;
  /** Zweite Zeile: was den Eintrag von seinen Nachbarn unterscheidet. */
  readonly subtitle?: string;
  /** Farbpunkt vor dem Namen. Ein archivierter Eintrag zeigt keinen. */
  readonly color?: string | null;
  /** Aus der Auswahl genommen (archiviert bzw. deaktiviert). */
  readonly retired?: boolean;
}

/** Umkehrbarer Rückzug aus der Auswahl: archivieren bzw. deaktivieren. */
export interface CatalogRetire<T> {
  readonly menuLabel: string;
  readonly confirmTitle: string;
  readonly confirmLabel: string;
  readonly describe: (item: T) => React.ReactNode;
  readonly run: (item: T) => Promise<unknown>;
  readonly toast: (item: T) => string;
}

export interface CatalogRestore<T> {
  readonly menuLabel: string;
  readonly run: (item: T) => Promise<unknown>;
  readonly toast: (item: T) => string;
}

/** Endgültiges Löschen. Nur für Kataloge, deren Backend das kennt. */
export interface CatalogRemove<T> {
  readonly menuLabel: string;
  readonly confirmTitle: string;
  readonly describe: (item: T) => React.ReactNode;
  readonly run: (item: T) => Promise<unknown>;
  readonly toast: (item: T) => string;
}

export interface CatalogConfig<T extends CatalogItem> {
  /** Symbol und Farbton der Fläche, aus `MOTO_CONCEPTS`. */
  readonly concept: MotoConceptKey;
  /** Seitentitel, Mehrzahl: „Terminkategorien". */
  readonly title: string;
  /** Einzahl für Knöpfe und Rückfragen: „Terminkategorie". */
  readonly singular: string;
  /** Ein Satz, wofür die Liste da ist (Verständlichkeit-Check). */
  readonly purpose: string;
  readonly searchPlaceholder: string;
  /** Leerzustand der Sammlung, wenn noch nichts angelegt ist. */
  readonly emptyDescription: string;
  /** Statuszeile der Kopfkarte aus den bereits geladenen Einträgen. */
  readonly stats: (items: readonly T[]) => string;
  readonly toRow: (item: T) => CatalogRow;
  /** Felder des Formulars. `null` = Anlegen-Dialog. */
  readonly sections: (item: T | null) => SectionConfig[];
  readonly toFormValues: (item: T) => Record<string, unknown>;
  readonly createDefaults?: Record<string, unknown>;
  readonly create: (values: Record<string, unknown>) => Promise<unknown>;
  readonly update: (
    item: T,
    values: Record<string, unknown>,
  ) => Promise<unknown>;
  readonly retire?: CatalogRetire<T>;
  readonly restore?: CatalogRestore<T>;
  readonly remove?: CatalogRemove<T>;
  /** Reihenfolge der aktiven Einträge, in der übergebenen Folge von ids. */
  readonly reorder?: (orderedIds: string[]) => Promise<unknown>;
  /** Einträge, die nicht bearbeitet werden können (System-Stammdaten). */
  readonly isReadOnly?: (item: T) => boolean;
  /** Ein Satz, warum dieser Eintrag nicht bearbeitet werden kann. */
  readonly readOnlyHint?: string;
}

interface CatalogPageProps<T extends CatalogItem> {
  readonly config: CatalogConfig<T>;
  readonly items: readonly T[] | undefined;
  readonly isLoading: boolean;
  /** Ladefehler der Liste. Ein Fehler ist nie ein Leerzustand. */
  readonly error: string | null;
  /** Neu laden, nachdem geschrieben wurde. */
  readonly onChanged: () => Promise<unknown> | void;
  /** Ohne Recht bleibt die Fläche eine Liste zum Nachlesen. */
  readonly canManage: boolean;
  /** Auswahl aus der Adresse, damit ein Neuladen denselben Eintrag zeigt. */
  readonly selectedId: string | null;
  /** Zusätzliche Kopf-Aktion, z. B. „Beispiele hinzufügen". */
  readonly headAction?: React.ReactNode;
}

function messageOf(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback;
}

export function CatalogPage<T extends CatalogItem>({
  config,
  items,
  isLoading,
  error,
  onChanged,
  canManage,
  selectedId,
  headAction,
}: CatalogPageProps<T>) {
  const toast = useToast();
  const updateUrlParams = useUpdateUrlParams();
  const [searchTerm, setSearchTerm] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [retireTarget, setRetireTarget] = useState<T | null>(null);
  const [removeTarget, setRemoveTarget] = useState<T | null>(null);
  const [busy, setBusy] = useState(false);
  // Zählt jedes gespeicherte Formular hoch, damit `DatabaseForm` seine Felder
  // aus dem frisch geladenen Eintrag neu aufbaut (wie in den Räumen).
  const [formGeneration, setFormGeneration] = useState(0);

  const all = useMemo(() => items ?? [], [items]);

  const select = useCallback(
    (id: string | null) => updateUrlParams({ [CATALOG_SELECTION_PARAM]: id }),
    [updateUrlParams],
  );

  // Gegen die ungefilterte Liste aufgelöst: eine Suche, die die Zeile
  // ausblendet, darf die geöffnete Objektansicht nicht schließen.
  const selected = useMemo(
    () => all.find((item) => item.id === selectedId) ?? null,
    [all, selectedId],
  );

  const visible = useMemo(() => {
    const needle = searchTerm.trim().toLocaleLowerCase("de");
    if (!needle) return all;
    return all.filter((item) =>
      config.toRow(item).name.toLocaleLowerCase("de").includes(needle),
    );
  }, [all, searchTerm, config]);

  const active = useMemo(
    () => visible.filter((item) => !config.toRow(item).retired),
    [visible, config],
  );
  const retired = useMemo(
    () => visible.filter((item) => config.toRow(item).retired),
    [visible, config],
  );

  /** Ein Schreibvorgang mit gemeinsamem Busy-, Fehler- und Nachlade-Vertrag. */
  const runWrite = useCallback(
    async (event: string, fallback: string, write: () => Promise<unknown>) => {
      setBusy(true);
      try {
        await write();
        await onChanged();
        return true;
      } catch (err: unknown) {
        logger.error(event, {
          error: err instanceof Error ? err.message : String(err),
        });
        toast.error(messageOf(err, fallback));
        return false;
      } finally {
        setBusy(false);
      }
    },
    [onChanged, toast],
  );

  const handleCreate = useCallback(
    async (values: Record<string, unknown>) => {
      // Der Fehler gehört ins Formular, nicht in ein Toast (#3113):
      // `DatabaseForm` fängt ihn und zeigt ihn im Alert über den Feldern.
      await config.create(values);
      setCreateOpen(false);
      await onChanged();
      toast.success(`${config.singular} angelegt`);
    },
    [config, onChanged, toast],
  );

  const handleUpdate = useCallback(
    async (values: Record<string, unknown>) => {
      if (!selected) return;
      await config.update(selected, values);
      await onChanged();
      setFormGeneration((generation) => generation + 1);
      toast.success("Änderungen gespeichert");
    },
    [config, onChanged, selected, toast],
  );

  const handleRetire = useCallback(async () => {
    const target = retireTarget;
    const retire = config.retire;
    if (!target || !retire) return;
    const ok = await runWrite(
      "catalog_retire_failed",
      `${config.singular} konnte nicht geändert werden.`,
      () => retire.run(target),
    );
    setRetireTarget(null);
    if (ok) toast.success(retire.toast(target));
  }, [config, retireTarget, runWrite, toast]);

  const handleRestore = useCallback(
    async (item: T) => {
      const restore = config.restore;
      if (!restore) return;
      const ok = await runWrite(
        "catalog_restore_failed",
        `${config.singular} konnte nicht wiederhergestellt werden.`,
        () => restore.run(item),
      );
      if (ok) toast.success(restore.toast(item));
    },
    [config, runWrite, toast],
  );

  const handleRemove = useCallback(async () => {
    const target = removeTarget;
    const remove = config.remove;
    if (!target || !remove) return;
    const ok = await runWrite(
      "catalog_remove_failed",
      `${config.singular} konnte nicht gelöscht werden.`,
      () => remove.run(target),
    );
    setRemoveTarget(null);
    if (ok) {
      select(null);
      toast.success(remove.toast(target));
    }
  }, [config, removeTarget, runWrite, select, toast]);

  const handleMove = useCallback(
    async (index: number, offset: -1 | 1) => {
      const reorder = config.reorder;
      const target = index + offset;
      if (!reorder || target < 0 || target >= active.length) return;
      const ordered = [...active];
      const moved = ordered[index];
      const displaced = ordered[target];
      if (!moved || !displaced) return;
      ordered[index] = displaced;
      ordered[target] = moved;
      await runWrite(
        "catalog_reorder_failed",
        "Die Reihenfolge konnte nicht gespeichert werden.",
        () => reorder(ordered.map((item) => item.id)),
      );
    },
    [active, config, runWrite],
  );

  const renderRow = (item: T, index: number, inActiveGroup: boolean) => {
    const row = config.toRow(item);
    const canMove = inActiveGroup && config.reorder !== undefined && canManage;
    return (
      <div className="flex items-center gap-1 pr-2">
        <div className="min-w-0 flex-1">
          <DatabaseListItem
            title={row.name}
            subtitle={row.subtitle}
            isSelected={selectedId === item.id}
            onSelect={() => select(item.id)}
            trailingAccessory={
              row.color && !row.retired ? (
                <span
                  className="size-4 shrink-0 rounded-full border border-black/10"
                  style={{ backgroundColor: row.color }}
                  aria-hidden="true"
                />
              ) : undefined
            }
          />
        </div>
        {/* Umsortieren ist eine Listenaktion und bleibt sichtbar
            (BAUARTEN-SPEC Bauart 1 Regel 4); alles, was den Eintrag selbst
            betrifft, steht im Kebab der Objektansicht. */}
        {canMove && (
          <>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label={`${row.name} nach oben`}
              disabled={busy || index === 0}
              onClick={() => void handleMove(index, -1)}
            >
              <ChevronUp className="size-4" aria-hidden="true" />
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              aria-label={`${row.name} nach unten`}
              disabled={busy || index === active.length - 1}
              onClick={() => void handleMove(index, 1)}
            >
              <ChevronDown className="size-4" aria-hidden="true" />
            </Button>
          </>
        )}
      </div>
    );
  };

  const entrySuffix = (count: number) => (count === 1 ? "Eintrag" : "Einträge");

  const groups: GroupDefinition<T>[] = [];
  if (active.length > 0 || retired.length === 0) {
    groups.push({
      id: "aktiv",
      title: config.title,
      items: [...active],
      countSuffix: entrySuffix(active.length),
    });
  }
  if (retired.length > 0) {
    groups.push({
      id: "archiviert",
      title: "Nicht mehr in der Auswahl",
      items: [...retired],
      countSuffix: entrySuffix(retired.length),
    });
  }

  const selectedRow = selected ? config.toRow(selected) : null;
  const selectedReadOnly = selected
    ? Boolean(config.isReadOnly?.(selected))
    : false;

  const detailMenu: OverflowMenuEntry[] =
    selected && canManage && !selectedReadOnly
      ? [
          ...(config.restore && selectedRow?.retired
            ? [
                {
                  label: config.restore.menuLabel,
                  disabled: busy,
                  onClick: () => void handleRestore(selected),
                },
              ]
            : []),
          ...(config.retire && !selectedRow?.retired
            ? [
                {
                  label: config.retire.menuLabel,
                  disabled: busy,
                  onClick: () => setRetireTarget(selected),
                },
              ]
            : []),
          ...(config.remove
            ? [
                { kind: "separator" as const },
                {
                  label: config.remove.menuLabel,
                  destructive: true,
                  disabled: busy,
                  onClick: () => setRemoveTarget(selected),
                },
              ]
            : []),
        ]
      : [];

  const detailSections = useMemo(
    () => (selected ? config.sections(selected).map(configToFormSection) : []),
    [config, selected],
  );

  const concept = MOTO_CONCEPTS[config.concept];

  const detail = selected ? (
    <DetailPanel
      header={
        <DatabaseDetailHeader
          icon={
            <MotoDuotoneIcon
              icon={concept.icon}
              tone={concept.tone}
              size={36}
            />
          }
          title={selectedRow?.name ?? config.singular}
          subtitle={selectedRow?.subtitle ?? ""}
          warning={
            selectedReadOnly
              ? (config.readOnlyHint ?? "Dieser Eintrag gehört zum System.")
              : null
          }
          actions={
            detailMenu.length > 0 ? (
              <OverflowMenu
                ariaLabel={`Aktionen für ${selectedRow?.name ?? config.singular}`}
                items={detailMenu}
              />
            ) : null
          }
        />
      }
      tabs={[
        {
          id: "stammdaten",
          label: "Stammdaten",
          content:
            canManage && !selectedReadOnly ? (
              <DatabaseForm
                key={`${selected.id}:${formGeneration}`}
                sections={detailSections}
                initialData={config.toFormValues(selected)}
                onSubmit={handleUpdate}
                onCancel={() =>
                  setFormGeneration((generation) => generation + 1)
                }
                submitLabel="Speichern"
                stickyActions
              />
            ) : (
              <p className="text-sm text-gray-600">
                {selectedReadOnly
                  ? (config.readOnlyHint ??
                    "Dieser Eintrag gehört zum System und lässt sich nicht ändern.")
                  : `Sie können ${config.title} ansehen. Ändern können sie Personen mit dem passenden Recht.`}
              </p>
            ),
        },
      ]}
      activeTab="stammdaten"
      onTabChange={() => undefined}
    />
  ) : (
    <EmptyDetailState
      title={`${config.singular} auswählen`}
      description={`Wählen Sie links einen Eintrag, um ihn zu ändern.`}
    />
  );

  const hasNothing = visible.length === 0 && selected === null;
  const isSearching = searchTerm.trim() !== "";

  return (
    <DatabasePageLayout
      loading={isLoading}
      sessionLoading={false}
      error={error}
      className="flex w-full flex-col"
      empty={
        hasNothing
          ? {
              title: isSearching
                ? "Nichts gefunden"
                : `Noch keine ${config.title}`,
              description: isSearching
                ? "Bitte prüfen Sie die Schreibweise oder löschen Sie die Suche."
                : config.emptyDescription,
              icon: (
                <MotoDuotoneIcon
                  icon={concept.icon}
                  tone={concept.tone}
                  size={48}
                />
              ),
              action:
                isSearching || !canManage ? undefined : (
                  <DatabaseCreateAction
                    label={config.singular}
                    ariaLabel={`${config.singular} anlegen`}
                    onClick={() => setCreateOpen(true)}
                  />
                ),
            }
          : null
      }
      intro={{
        title: config.title,
        description: isLoading ? (
          <Skeleton className="h-4 w-48" />
        ) : (
          config.stats(all)
        ),
        actions: canManage ? (
          <div className="flex items-center gap-2">
            {headAction}
            <DatabaseCreateAction
              label={config.singular}
              ariaLabel={`${config.singular} anlegen`}
              onClick={() => setCreateOpen(true)}
            />
          </div>
        ) : undefined,
      }}
      search={
        <PageHeaderWithSearch
          embedded
          title=""
          badge={{
            icon: (
              <MotoDuotoneIcon
                icon={concept.icon}
                tone={concept.tone}
                size={20}
              />
            ),
            count: visible.length,
            label: config.title,
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: config.searchPlaceholder,
          }}
        />
      }
      overlays={
        <>
          {createOpen && (
            <DatabaseFormModal<Record<string, unknown>>
              isOpen
              onClose={() => setCreateOpen(false)}
              mode="create"
              config={{
                form: {
                  sections: config.sections(null),
                  defaultValues: config.createDefaults,
                },
                labels: {
                  createModalTitle: `${config.singular} anlegen`,
                  editModalTitle: `${config.singular} bearbeiten`,
                },
              }}
              onSubmit={handleCreate}
            />
          )}

          {/* Archivieren ist umkehrbar und damit keine Löschung
              (BAUARTEN-SPEC Bauart 2 Regel 6). */}
          {config.retire && (
            <ConfirmationModal
              isOpen={retireTarget !== null}
              title={config.retire.confirmTitle}
              confirmText={config.retire.confirmLabel}
              cancelText="Abbrechen"
              isConfirmLoading={busy}
              isDismissDisabled={busy}
              onConfirm={() => void handleRetire()}
              onClose={() => setRetireTarget(null)}
            >
              <p className="text-sm text-gray-700">
                {retireTarget ? config.retire.describe(retireTarget) : null}
              </p>
            </ConfirmationModal>
          )}

          {config.remove && (
            <ConfirmDeleteModal
              isOpen={removeTarget !== null}
              title={config.remove.confirmTitle}
              description={
                removeTarget ? config.remove.describe(removeTarget) : ""
              }
              gate={{ mode: "twoStep" }}
              loading={busy}
              error=""
              onConfirm={() => void handleRemove()}
              onClose={() => setRemoveTarget(null)}
            />
          )}
        </>
      }
    >
      <div className="min-h-0 flex-1 pb-4">
        <MasterDetailLayout
          list={
            <GroupedList
              groups={groups}
              keyFor={(item) => item.id}
              renderItem={(item) => {
                const index = active.indexOf(item);
                return renderRow(item, index, index >= 0);
              }}
              emptyState={
                <p className="text-center text-sm text-gray-500">
                  Nichts gefunden.
                </p>
              }
            />
          }
          detail={detail}
          selectedId={selectedId}
          onDeselect={() => select(null)}
          mobileDrawerTitle={selectedRow?.name ?? config.singular}
        />
      </div>
    </DatabasePageLayout>
  );
}
