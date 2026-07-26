"use client";

import { Loading } from "~/components/ui/loading";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { ConfirmationModal } from "~/components/ui/modal";
import { UnclaimedRooms } from "~/components/active/unclaimed-rooms";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";

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

export function ActiveSupervisionLoadingView() {
  return <Loading fullPage={false} />;
}

export function NoActiveSupervisionAccessView() {
  useSetBreadcrumb({
    pageTitle: "Aktuelle Aufsicht",
  });

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Aktuelle Aufsicht" />

      <div className="flex min-h-[60vh] items-center justify-center px-4">
        <div className="flex max-w-md flex-col items-center gap-6 text-center">
          <svg
            className="h-12 w-12 text-gray-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
            />
          </svg>
          <div className="space-y-2">
            <h3 className="text-lg font-medium text-gray-900">
              Keine aktive Raum-Aufsicht
            </h3>
            <p className="text-gray-600">
              Du bist aktuell in keinem Raum als Live-Aktivität registriert.
            </p>
            <p className="mt-4 text-sm text-gray-500">
              Starte eine Aktivität an einem Terminal, um Live-Raumdaten
              einzusehen.
            </p>
          </div>
        </div>
      </div>
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
        title=""
        search={{
          value: searchTerm,
          onChange: setSearchTerm,
          placeholder: "Name suchen...",
        }}
        filters={filterConfigs}
        activeFilters={activeFilters}
        onClearAllFilters={() => {
          setSearchTerm("");
          setGroupFilter("all");
          setSelectedYear("all");
        }}
      />

      <div className="mt-8 flex min-h-[30vh] items-center justify-center">
        <div className="flex max-w-md flex-col items-center gap-4 text-center">
          <svg
            className="h-12 w-12 text-gray-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
            />
          </svg>
          <div className="space-y-1">
            <h3 className="text-lg font-medium text-gray-900">
              Keine aktive Raum-Aufsicht
            </h3>
            <p className="text-sm text-gray-500">
              Du beaufsichtigst aktuell keinen Raum.
            </p>
          </div>
        </div>
      </div>
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
      confirmButtonClass="bg-red-600 hover:bg-red-700"
      isConfirmLoading={isConfirmLoading}
    >
      <div className="space-y-4">
        <div className="rounded-lg border border-red-100 bg-red-50/50 p-3">
          <div className="flex items-start gap-3">
            <svg
              className="mt-0.5 size-5 flex-shrink-0 text-red-500"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
              />
            </svg>
            <div className="flex-1">
              <p className="text-sm text-gray-600">
                Du wirst nicht mehr als Aufsicht angezeigt. Der Schulhof wird
                dann als &quot;ohne Aufsicht&quot; angezeigt, bis eine andere
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
    <div className="flex flex-col items-center gap-4 py-12 text-center">
      <svg
        className="h-12 w-12 text-gray-400"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        strokeWidth={1.5}
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          d="M12 3v2.25m6.364.386l-1.591 1.591M21 12h-2.25m-.386 6.364l-1.591-1.591M12 18.75V21m-4.773-4.227l-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0z"
        />
      </svg>
      <p className="text-lg font-medium text-gray-900">
        Schulhof ohne Aufsicht
      </p>
      <p className="text-sm text-gray-500">
        {supervisorCount > 0
          ? `Aktuelle Aufsicht: ${supervisorNames.join(", ")}`
          : "Übernimm die Aufsicht, um Kinder zu sehen."}
      </p>
      <button
        type="button"
        onClick={onToggle}
        disabled={isToggling}
        className="mt-2 rounded-full bg-gray-900 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700 disabled:opacity-50"
      >
        {isToggling ? "Wird übernommen..." : "Beaufsichtigen"}
      </button>
    </div>
  );
}
