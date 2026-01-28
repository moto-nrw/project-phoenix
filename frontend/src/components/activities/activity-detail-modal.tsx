"use client";

import { Modal } from "~/components/ui/modal";
import { DetailModalActions } from "~/components/ui/detail-modal-actions";
import type { Activity } from "@/lib/activity-helpers";

interface ActivityDetailModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly activity: Activity | null;
  readonly onEdit: () => void;
  readonly onDelete: () => void;
  readonly loading?: boolean;
  /**
   * Custom click handler for delete button.
   * When provided, bypasses internal confirmation modal.
   * Use this to handle confirmation at the page level.
   */
  readonly onDeleteClick?: () => void;
}

export function ActivityDetailModal({
  isOpen,
  onClose,
  activity,
  onEdit,
  onDelete,
  loading = false,
  onDeleteClick,
}: ActivityDetailModalProps) {
  if (!activity) return null;

  const initials = (activity.name?.slice(0, 2) ?? "AG").toUpperCase();

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="">
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-[#FF3130]" />
            <p className="text-gray-600">Daten werden geladen...</p>
          </div>
        </div>
      ) : (
        <div className="space-y-4 md:space-y-6">
          {/* Header */}
          <div className="flex items-center gap-3 border-b border-gray-100 pb-3 md:gap-4 md:pb-4">
            <div className="flex h-14 w-14 items-center justify-center rounded-full bg-gradient-to-br from-[#FF3130] to-[#e02020] text-xl font-semibold text-white shadow-md md:h-16 md:w-16">
              {initials}
            </div>
            <div className="min-w-0">
              <h2 className="truncate text-lg font-semibold text-gray-900 md:text-xl">
                {activity.name}
              </h2>
              <p className="truncate text-sm text-gray-500">
                {activity.category_name ?? "Keine Kategorie"}
              </p>
            </div>
          </div>

          {/* Details */}
          <div className="space-y-3 md:space-y-4">
            <div className="rounded-xl border border-gray-100 bg-red-50/30 p-3 md:p-4">
              <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
                <svg
                  className="h-3.5 w-3.5 text-red-600 md:h-4 md:w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
                  />
                </svg>
                Aktivitätsdetails
              </h3>
              <dl className="grid grid-cols-1 gap-x-3 gap-y-2 sm:grid-cols-2 md:gap-x-4 md:gap-y-3">
                <div>
                  <dt className="text-xs text-gray-500">Kategorie</dt>
                  <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                    {activity.category_name ?? "Keine Kategorie"}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-gray-500">Maximale Teilnehmer</dt>
                  <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                    {activity.max_participant}
                  </dd>
                </div>
                {activity.supervisor_name && (
                  <div className="sm:col-span-2">
                    <dt className="text-xs text-gray-500">Hauptbetreuer</dt>
                    <dd className="mt-0.5 text-xs break-words whitespace-pre-wrap text-gray-700 md:text-sm">
                      {activity.supervisor_name}
                    </dd>
                  </div>
                )}
              </dl>
            </div>
          </div>

          <DetailModalActions
            onEdit={onEdit}
            onDelete={onDelete}
            entityName={activity.name}
            entityType="Aktivität"
            onDeleteClick={onDeleteClick}
          />
        </div>
      )}
    </Modal>
  );
}
