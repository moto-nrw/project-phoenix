"use client";

import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import {
  SCHULHOF_ROOM_NAME,
  SCHULHOF_TAB_ID,
  roomsOutsideSchulhofStatus,
  supervisionTabLabel,
} from "~/components/active-supervisions/view-model";
import type {
  ActiveSupervisionRoom,
  SchulhofStatusResponse,
  SupervisionSessionInfo,
} from "~/components/active-supervisions/view-model";

interface SupervisionHeaderProps {
  readonly isDesktop: boolean;
  readonly allRooms: ActiveSupervisionRoom[];
  readonly currentRoom: ActiveSupervisionRoom | null;
  readonly isSchulhofTabSelected: boolean;
  readonly schulhofTabEnabled: boolean;
  readonly schulhofTabAvailable: boolean;
  readonly schulhofStatus: SchulhofStatusResponse | null;
  readonly sessionInfoByActiveGroup: ReadonlyMap<
    string,
    SupervisionSessionInfo
  >;
  readonly searchTerm: string;
  readonly onSearchChange: (value: string) => void;
  readonly filterConfigs: FilterConfig[];
  readonly activeFilters: ActiveFilter[];
  readonly onClearAllFilters: () => void;
  readonly onTabChange: (tabId: string) => void;
  // The Schulhof release buttons stay with the page (they carry legacy
  // utility classes tracked in the ui-kit ratchet baseline for page.tsx).
  readonly actionButton: React.ReactNode;
  readonly mobileActionButton: React.ReactNode;
}

/**
 * Page header of the Aktuelle Aufsicht: session title / tab dropdown
 * (mobile), live child count badge, search, filters, and the Schulhof
 * release action.
 */
export function SupervisionHeader({
  isDesktop,
  allRooms,
  currentRoom,
  isSchulhofTabSelected,
  schulhofTabEnabled,
  schulhofTabAvailable,
  schulhofStatus,
  sessionInfoByActiveGroup,
  searchTerm,
  onSearchChange,
  filterConfigs,
  activeFilters,
  onClearAllFilters,
  onTabChange,
  actionButton,
  mobileActionButton,
}: SupervisionHeaderProps) {
  // With the permanent tab enabled, exclude only the active group already
  // represented by schulhofStatus. Other parallel Schulhof sessions stay
  // reachable as normal supervision tabs.
  const roomsOutsideStatus = roomsOutsideSchulhofStatus(allRooms, {
    schulhofTabEnabled,
    statusActiveGroupId: schulhofStatus?.activeGroupId,
  });
  const totalSupervisions =
    roomsOutsideStatus.length + (schulhofTabAvailable ? 1 : 0);

  return (
    <PageHeaderWithSearch
      title={
        // Mobile only: Show title when exactly 1 supervision
        // 1 supervision = title, 2+ supervisions = tabs (dropdown)
        !isDesktop && totalSupervisions === 1
          ? isSchulhofTabSelected
            ? SCHULHOF_ROOM_NAME
            : currentRoom
              ? supervisionTabLabel(
                  currentRoom,
                  sessionInfoByActiveGroup.get(currentRoom.id) ?? null,
                )
              : "Aktuelle Aufsicht"
          : ""
      }
      badge={{
        icon: (
          <svg
            className="h-5 w-5 text-gray-600"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
            />
          </svg>
        ),
        count: isSchulhofTabSelected
          ? (schulhofStatus?.studentCount ?? 0)
          : (currentRoom?.student_count ?? 0),
        label: "Kinder",
      }}
      tabs={
        // Show tabs (dropdown) when 2+ supervisions
        totalSupervisions >= 2 && !isDesktop
          ? {
              items: [
                // Regular supervised sessions, including any parallel
                // Schulhof group not represented by the permanent tab.
                ...roomsOutsideStatus.map((room) => ({
                  id: room.id,
                  label: supervisionTabLabel(
                    room,
                    sessionInfoByActiveGroup.get(room.id) ?? null,
                  ),
                })),
                // Schulhof permanent tab (only with the spontaneous
                // capability, #2161)
                ...(schulhofTabAvailable
                  ? [
                      {
                        id: SCHULHOF_TAB_ID,
                        label: SCHULHOF_ROOM_NAME,
                      },
                    ]
                  : []),
              ],
              activeTab: isSchulhofTabSelected
                ? SCHULHOF_TAB_ID
                : (currentRoom?.id ?? ""),
              onTabChange,
            }
          : undefined
      }
      search={{
        value: searchTerm,
        onChange: onSearchChange,
        placeholder: "Name suchen...",
      }}
      filters={filterConfigs}
      activeFilters={activeFilters}
      onClearAllFilters={onClearAllFilters}
      actionButton={actionButton}
      mobileActionButton={mobileActionButton}
    />
  );
}
