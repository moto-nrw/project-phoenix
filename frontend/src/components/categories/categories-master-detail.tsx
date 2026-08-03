"use client";

import { useMemo, useState } from "react";
import { DatabaseDetailHeader } from "~/components/database/database-detail-header";
import { DatabaseListItem } from "~/components/database/database-list-item";
import { DetailDeleteButton } from "~/components/database/detail-delete-button";
import {
  DetailPanel,
  type DetailTab,
} from "~/components/database/detail-panel";
import { EmptyDetailState } from "~/components/database/empty-detail-state";
import { GroupedList } from "~/components/database/grouped-list";
import { MasterDetailLayout } from "~/components/database/master-detail-layout";
import { useGroupedItems } from "~/components/database/use-grouped-items";
import { DatabaseForm } from "~/components/ui/database/database-form";
import { Button } from "~/components/ui/button";
import { StatusBadge } from "~/components/ui/status-badge";
import { categoriesConfig } from "@/components/database/configs/categories.config";
import { configToFormSection } from "@/lib/database/types";
import type { ActivityCategory } from "@/lib/activity-helpers";

interface CategoriesMasterDetailProps {
  categories: ActivityCategory[];
  selectedId: string | null;
  /**
   * Resolved by the page against the unfiltered list so the detail panel
   * survives a search narrowing the visible rows.
   */
  selectedCategory: ActivityCategory | null;
  onSelect: (id: string | null) => void;
  onSaveCategory: (data: Partial<ActivityCategory>) => Promise<void>;
  onArchiveClick: () => void;
  onRestore: () => Promise<void>;
}

function keyForCategory(category: ActivityCategory): string {
  return category.id;
}

function usageLabel(category: ActivityCategory): string {
  const count = category.usageCount ?? 0;
  if (count === 0) return "Noch nicht verwendet";
  if (count === 1) return "In 1 Aktivität/Termin verwendet";
  return `In ${count} Aktivitäten/Terminen verwendet`;
}

export function CategoriesMasterDetail({
  categories,
  selectedId,
  selectedCategory,
  onSelect,
  onSaveCategory,
  onArchiveClick,
  onRestore,
}: CategoriesMasterDetailProps) {
  // Active first, archived collected at the bottom — the archived block is the
  // only place a school can get a category back, so it must stay visible.
  const groupDefinitions = useGroupedItems(
    categories,
    "status",
    {
      status: (category: ActivityCategory) =>
        category.archivedAt
          ? { id: "archived", title: "Archiviert", sortKey: "2" }
          : { id: "active", title: "Aktiv", sortKey: "1" },
    },
    "Kategorien",
  );

  const renderItem = (category: ActivityCategory) => (
    <DatabaseListItem
      title={category.name}
      subtitle={usageLabel(category)}
      isSelected={selectedId === category.id}
      onSelect={() => onSelect(category.id)}
    />
  );

  const listNode = (
    <GroupedList
      groups={groupDefinitions}
      renderItem={renderItem}
      keyFor={keyForCategory}
      emptyState={
        <div className="text-center text-sm text-gray-500">
          Keine Kategorien gefunden.
        </div>
      }
    />
  );

  const detailNode = selectedCategory ? (
    <CategoryDetailContent
      category={selectedCategory}
      onSaveCategory={onSaveCategory}
      onArchiveClick={onArchiveClick}
      onRestore={onRestore}
    />
  ) : (
    <EmptyDetailState
      title="Keine Kategorie ausgewählt"
      description="Wähle links eine Kategorie, um sie umzubenennen oder zu archivieren."
    />
  );

  return (
    <MasterDetailLayout
      list={listNode}
      detail={detailNode}
      selectedId={selectedId}
      onDeselect={() => onSelect(null)}
      unselectedBehavior="expand"
      mobileDrawerTitle={selectedCategory?.name ?? "Kategorie"}
    />
  );
}

interface CategoryDetailContentProps {
  category: ActivityCategory;
  onSaveCategory: (data: Partial<ActivityCategory>) => Promise<void>;
  onArchiveClick: () => void;
  onRestore: () => Promise<void>;
}

function CategoryDetailContent({
  category,
  onSaveCategory,
  onArchiveClick,
  onRestore,
}: CategoryDetailContentProps) {
  const [activeTab, setActiveTab] = useState<string>("master-data");
  const [formResetCounter, setFormResetCounter] = useState(0);
  const [restoring, setRestoring] = useState(false);

  const isArchived = Boolean(category.archivedAt);

  const handleSaveCategory = async (data: Partial<ActivityCategory>) => {
    await onSaveCategory(data);
    setFormResetCounter((n) => n + 1);
  };

  const handleRestore = async () => {
    setRestoring(true);
    try {
      await onRestore();
    } finally {
      setRestoring(false);
    }
  };

  const headerActions = isArchived ? (
    <Button
      type="button"
      variant="secondary"
      size="md"
      onClick={() => void handleRestore()}
      disabled={restoring}
    >
      {restoring ? "Wird wiederhergestellt…" : "Wiederherstellen"}
    </Button>
  ) : (
    <DetailDeleteButton onClick={onArchiveClick} label="Archivieren" />
  );

  const tabs: DetailTab[] = [
    {
      id: "master-data",
      label: "Stammdaten",
      content: (
        <CategoryStammdatenTab
          category={category}
          onSaveCategory={handleSaveCategory}
          onResetForm={() => setFormResetCounter((n) => n + 1)}
          formResetKey={`${category.id}:${formResetCounter}`}
        />
      ),
    },
  ];

  return (
    <DetailPanel
      header={
        <DatabaseDetailHeader
          avatar={category.name?.charAt(0)?.toUpperCase() || "K"}
          title={category.name}
          subtitle={usageLabel(category)}
          actions={headerActions}
        />
      }
      tabs={tabs}
      activeTab={activeTab}
      onTabChange={setActiveTab}
    />
  );
}

interface CategoryStammdatenTabProps {
  category: ActivityCategory;
  onSaveCategory: (data: Partial<ActivityCategory>) => Promise<void>;
  onResetForm: () => void;
  formResetKey: string;
}

function CategoryStammdatenTab({
  category,
  onSaveCategory,
  onResetForm,
  formResetKey,
}: CategoryStammdatenTabProps) {
  const sections = useMemo(
    () => categoriesConfig.form.sections.map(configToFormSection),
    [],
  );

  const isArchived = Boolean(category.archivedAt);

  return (
    <div className="space-y-6">
      <CategorySummary category={category} />
      {isArchived ? (
        <p className="rounded-md border border-gray-200 bg-gray-50 p-4 text-sm text-gray-600">
          Diese Kategorie ist archiviert. Bestehende Termine und Aktivitäten
          behalten sie, für neue wird sie nicht mehr angeboten. Zum Bearbeiten
          zuerst wiederherstellen.
        </p>
      ) : (
        <DatabaseForm
          key={formResetKey}
          theme={categoriesConfig.theme}
          sections={sections}
          initialData={category}
          onSubmit={onSaveCategory}
          onCancel={onResetForm}
          submitLabel="Speichern"
          stickyActions
        />
      )}
    </div>
  );
}

function CategorySummary({ category }: { category: ActivityCategory }) {
  return (
    <section className="rounded-lg border border-gray-200 bg-white p-4">
      <div className="flex flex-wrap items-center gap-2">
        <StatusBadge
          tone={category.archivedAt ? "gray" : "green"}
          label={category.archivedAt ? "Archiviert" : "Aktiv"}
        />
        <span className="rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600">
          {usageLabel(category)}
        </span>
        {category.color ? (
          <span className="inline-flex items-center gap-1.5 rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600">
            <span
              aria-hidden="true"
              className="inline-block h-2.5 w-2.5 rounded-full"
              style={{ backgroundColor: category.color }}
            />
            Farbe
          </span>
        ) : null}
      </div>

      {category.description ? (
        <dl className="mt-4">
          <div>
            <dt className="text-xs font-medium text-gray-500">Beschreibung</dt>
            <dd className="mt-0.5 text-sm whitespace-pre-wrap text-gray-900">
              {category.description}
            </dd>
          </div>
        </dl>
      ) : null}
    </section>
  );
}
