"use client";

import { useMemo, useState } from "react";
import { DatabaseDetailHeader } from "~/components/database/database-detail-header";
import { DatabaseListItem } from "~/components/database/database-list-item";
import { DetailDeleteButton } from "~/components/database/detail-delete-button";
import { DetailLoadingSpinner } from "~/components/database/detail-loading-spinner";
import {
  DetailPanel,
  type DetailTab,
} from "~/components/database/detail-panel";
import { EmptyDetailState } from "~/components/database/empty-detail-state";
import { GroupedList } from "~/components/database/grouped-list";
import { MasterDetailLayout } from "~/components/database/master-detail-layout";
import { useGroupedItems } from "~/components/database/use-grouped-items";
import { DatabaseForm } from "~/components/ui/database/database-form";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import type { FormErrorInput } from "~/components/ui/form-error";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import { isSystemActivity, type Activity } from "@/lib/activity-helpers";
import { buildActivityFormSections } from "./activity-form-sections";

interface ActivitiesMasterDetailProps {
  activities: Activity[];
  selectedId: string | null;
  selectedActivity: Activity | null;
  detailLoading: boolean;
  /** Ladefehler des Details (aus `useApiLoadError`); ohne ihn stünde ein
   *  veralteter Stand im Formular. */
  detailError?: FormErrorInput;
  onSelect: (id: string | null) => void;
  onSaveActivity: (data: Partial<Activity>) => Promise<void>;
  onResetForm: () => void;
  onDeleteClick: () => void;
  formResetKey: string;
}

function keyForActivity(activity: Activity): string {
  return activity.id;
}

function buildActivitySubtitle(activity: Activity): string {
  return activity.category_name?.trim() || "Keine Kategorie";
}

export function ActivitiesMasterDetail({
  activities,
  selectedId,
  selectedActivity,
  detailLoading,
  detailError = null,
  onSelect,
  onSaveActivity,
  onResetForm,
  onDeleteClick,
  formResetKey,
}: ActivitiesMasterDetailProps) {
  const groupDefinitions = useGroupedItems(
    activities,
    "none",
    {},
    "Aktivitäten",
  );

  const renderItem = (activity: Activity) => (
    <DatabaseListItem
      title={activity.name}
      subtitle={buildActivitySubtitle(activity)}
      isSelected={selectedId === activity.id}
      onSelect={() => onSelect(activity.id)}
    />
  );

  const listNode = (
    <GroupedList
      groups={groupDefinitions}
      renderItem={renderItem}
      keyFor={keyForActivity}
      emptyState={
        <div className="text-center text-sm text-gray-500">
          Keine Aktivitäten gefunden.
        </div>
      }
    />
  );

  const detailNode = selectedActivity ? (
    <ActivityDetailContent
      activity={selectedActivity}
      loading={detailLoading}
      detailError={detailError}
      onSaveActivity={onSaveActivity}
      onResetForm={onResetForm}
      onDeleteClick={onDeleteClick}
      formResetKey={formResetKey}
    />
  ) : (
    <EmptyDetailState
      title="Keine Aktivität ausgewählt"
      description="Wähle links eine Aktivität, um Stammdaten direkt im Detailbereich zu bearbeiten."
    />
  );

  return (
    <MasterDetailLayout
      list={listNode}
      detail={detailNode}
      selectedId={selectedId}
      onDeselect={() => onSelect(null)}
      unselectedBehavior="expand"
      mobileDrawerTitle={selectedActivity?.name ?? "Aktivität"}
    />
  );
}

interface ActivityDetailContentProps {
  activity: Activity;
  loading: boolean;
  detailError: FormErrorInput;
  onSaveActivity: (data: Partial<Activity>) => Promise<void>;
  onResetForm: () => void;
  onDeleteClick: () => void;
  formResetKey: string;
}

function ActivityDetailContent({
  activity,
  loading,
  detailError,
  onSaveActivity,
  onResetForm,
  onDeleteClick,
  formResetKey,
}: ActivityDetailContentProps) {
  const [activeTab, setActiveTab] = useState<string>("master-data");
  const isSystem = isSystemActivity(activity);

  const headerActions = !isSystem ? (
    <DetailDeleteButton onClick={onDeleteClick} />
  ) : null;

  const tabs: DetailTab[] = [
    {
      id: "master-data",
      label: "Stammdaten",
      content: (
        <ActivityStammdatenTab
          // Eine andere Aktivität beginnt ohne den Fehler der vorigen.
          key={activity.id}
          activity={activity}
          loading={loading}
          detailError={detailError}
          onSaveActivity={onSaveActivity}
          onResetForm={onResetForm}
          formResetKey={formResetKey}
        />
      ),
    },
  ];

  return (
    <DetailPanel
      header={
        <DatabaseDetailHeader
          icon={
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.activities.icon}
              tone={MOTO_CONCEPTS.activities.tone}
              size={36}
            />
          }
          title={activity.name}
          subtitle={buildActivitySubtitle(activity)}
          actions={headerActions}
        />
      }
      tabs={tabs}
      activeTab={activeTab}
      onTabChange={setActiveTab}
    />
  );
}

interface ActivityStammdatenTabProps {
  activity: Activity;
  loading: boolean;
  detailError: FormErrorInput;
  onSaveActivity: (data: Partial<Activity>) => Promise<void>;
  onResetForm: () => void;
  formResetKey: string;
}

function ActivityStammdatenTab({
  activity,
  loading,
  detailError,
  onSaveActivity,
  onResetForm,
  formResetKey,
}: ActivityStammdatenTabProps) {
  const sections = useMemo(
    () => buildActivityFormSections(activity),
    [activity],
  );
  // Speicherfehler im Formular, Feldfehler am Feld (#2517).
  const saveErrors = useApiFormError();

  if (loading) {
    return <DetailLoadingSpinner label="Aktivitätsdaten werden geladen..." />;
  }

  // Ohne frisches Detail kein Formular mit veraltetem Stand.
  if (detailError) {
    return <LoadErrorAlert error={detailError} />;
  }

  return (
    <div className="space-y-6">
      <ActivitySummary activity={activity} />
      <DatabaseForm
        key={formResetKey}
        sections={sections}
        initialData={activity}
        onSubmit={onSaveActivity}
        onCancel={() => {
          saveErrors.clear();
          onResetForm();
        }}
        submitLabel="Speichern"
        stickyActions
        errorPath={saveErrors}
        errorObject="die Aktivität"
      />
    </div>
  );
}

function ActivitySummary({ activity }: { activity: Activity }) {
  const isSystem = isSystemActivity(activity);

  return (
    <section className="moto-content-surface rounded-xl border p-4 shadow-sm">
      <div className="flex flex-wrap items-center gap-2">
        {activity.category_name ? (
          <span className="rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600">
            {activity.category_name}
          </span>
        ) : null}
        <span className="rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600">
          {activity.max_participant == null
            ? "Unbegrenzt"
            : `${activity.max_participant} Plätze`}
        </span>
        {isSystem ? (
          <span className="rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600">
            Systemaktivität
          </span>
        ) : null}
      </div>

      {activity.supervisor_name ? (
        <div className="mt-4">
          <DataGrid>
            <DataField label="Hauptbetreuer" fullWidth>
              <span className="whitespace-pre-wrap">
                {activity.supervisor_name}
              </span>
            </DataField>
          </DataGrid>
        </div>
      ) : null}
    </section>
  );
}
