"use client";

import { WarningCircleIcon } from "@phosphor-icons/react";
import {
  SkeletonRegion,
  PageHeaderSkeleton,
  CardGridSkeleton,
} from "~/components/ui/page-skeletons";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { ConfirmationModal } from "~/components/ui/modal";
import { UnclaimedRooms } from "~/components/active/unclaimed-rooms";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { useNFCEnabled } from "~/lib/tenant-context";

interface MinimalActiveGroup {
  id: string;
  room?: { name?: string };
}

interface EmptyRoomsViewProps {
  readonly onClaimed: () => void;
  readonly cachedActiveGroups: MinimalActiveGroup[];
  readonly currentStaffId: string | undefined;
  readonly searchTerm: string;
  readonly setSearchTerm: (term: string) => void;
  readonly setGroupFilter: (filter: string) => void;
  readonly setSelectedYear: (year: string) => void;
  readonly filterConfigs: FilterConfig[];
  readonly activeFilters: ActiveFilter[];
}

interface SchulhofNotSupervisingViewProps {
  readonly supervisorCount: number;
  readonly supervisorNames: string[];
  readonly isToggling: boolean;
  readonly onToggle: () => void;
}

interface ReleaseSupervisionModalProps {
  readonly isOpen: boolean;
  readonly isConfirmLoading: boolean;
  readonly onClose: () => void;
  readonly onConfirm: () => void;
}

// withHeader is false when the real PageHeaderWithSearch chrome is already
// on screen (e.g. re-loading only the student grid) — showing a second
// header skeleton underneath it would duplicate the page's own chrome.
export function ActiveSupervisionLoadingView({
  withHeader = true,
}: Readonly<{ withHeader?: boolean }> = {}) {
  return (
    <SkeletonRegion label="Aktuelle Aufsicht wird geladen…">
      {withHeader && <PageHeaderSkeleton actions={1} />}
      <CardGridSkeleton
        cards={6}
        rowsPerCard={2}
        className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3"
      />
    </SkeletonRegion>
  );
}

export function NoActiveSupervisionAccessView() {
  const nfcEnabled = useNFCEnabled();

  useSetBreadcrumb({
    pageTitle: "Aktuelle Aufsicht",
  });

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Aktuelle Aufsicht" />

      <EmptyState
        icon={<MotoConceptIcon concept="rooms" size={48} />}
        title="Keine aktive Raum-Aufsicht"
        description={`Sie sind aktuell in keinem Raum als Live-Aktivität registriert. Starten Sie eine Aktivität ${
          nfcEnabled ? "an einem Terminal" : "in der Web-App"
        }, um Live-Raumdaten einzusehen.`}
      />
    </div>
  );
}

export function EmptyRoomsView({
  onClaimed,
  cachedActiveGroups,
  currentStaffId,
  searchTerm,
  setSearchTerm,
  setGroupFilter,
  setSelectedYear,
  filterConfigs,
  activeFilters,
}: EmptyRoomsViewProps) {
  return (
    <div className="w-full">
      <UnclaimedRooms
        onClaimed={onClaimed}
        activeGroups={
          cachedActiveGroups.length > 0 ? cachedActiveGroups : undefined
        }
        currentStaffId={currentStaffId}
      />

      <PageHeaderWithSearch
        title="Aktuelle Aufsicht"
        search={{
          value: searchTerm,
          onChange: setSearchTerm,
          placeholder: "Name suchen…",
        }}
        filters={filterConfigs}
        activeFilters={activeFilters}
        onClearAllFilters={() => {
          setSearchTerm("");
          setGroupFilter("all");
          setSelectedYear("all");
        }}
      />

      <EmptyState
        icon={<MotoConceptIcon concept="rooms" size={48} />}
        title="Keine aktive Raum-Aufsicht"
        description="Sie beaufsichtigen aktuell keinen Raum."
      />
    </div>
  );
}

export function ReleaseSupervisionModal({
  isOpen,
  isConfirmLoading,
  onClose,
  onConfirm,
}: ReleaseSupervisionModalProps) {
  return (
    <ConfirmationModal
      isOpen={isOpen}
      onClose={onClose}
      onConfirm={onConfirm}
      title="Aufsicht abgeben"
      confirmText="Abgeben"
      confirmButtonClass="bg-moto-red hover:bg-moto-red-hover"
      isConfirmLoading={isConfirmLoading}
    >
      <div className="space-y-4">
        <div className="border-moto-red/20 bg-moto-red-soft rounded-lg border p-3">
          <div className="flex items-start gap-3">
            <MotoDuotoneIcon
              icon={WarningCircleIcon}
              tone="red"
              size={20}
              className="mt-0.5 flex-shrink-0"
            />
            <div className="flex-1">
              <p className="text-sm text-gray-600">
                Sie werden nicht mehr als Aufsicht angezeigt. Der Schulhof wird
                dann als &bdquo;ohne Aufsicht&ldquo; angezeigt, bis eine andere
                Lehrkraft die Aufsicht übernimmt.
              </p>
            </div>
          </div>
        </div>
      </div>
    </ConfirmationModal>
  );
}

export function SchulhofNotSupervisingView({
  supervisorCount,
  supervisorNames,
  isToggling,
  onToggle,
}: SchulhofNotSupervisingViewProps) {
  return (
    <EmptyState
      icon={<MotoConceptIcon concept="schoolyard" size={48} />}
      title="Schulhof ohne Aufsicht"
      description={
        supervisorCount > 0
          ? `Aktuelle Aufsicht: ${supervisorNames.join(", ")}`
          : "Übernehmen Sie die Aufsicht, um Kinder zu sehen."
      }
      action={
        <Button
          type="button"
          variant="primary"
          size="md"
          onClick={onToggle}
          disabled={isToggling}
        >
          {isToggling ? "Wird übernommen…" : "Beaufsichtigen"}
        </Button>
      }
    />
  );
}
