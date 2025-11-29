"use client";

import { Modal } from "~/components/ui/modal";
import { DetailModalActions } from "~/components/ui/detail-modal-actions";
import type { Group } from "@/lib/group-helpers";

interface GroupDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  group: Group | null;
  onEdit: () => void;
  onDelete: () => void;
  loading?: boolean;
}

export function GroupDetailModal({
  isOpen,
  onClose,
  group,
  onEdit,
  onDelete,
  loading = false,
}: GroupDetailModalProps) {
  if (!group) return null;

  const initials = (group.name?.slice(0, 2) ?? "GR").toUpperCase();

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="">
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-[#83CD2D]" />
            <p className="text-gray-600">Daten werden geladen...</p>
          </div>
        </div>
      ) : (
        <div className="space-y-4 md:space-y-6">
          {/* Header */}
          <div className="flex items-center gap-3 border-b border-gray-100 pb-3 md:gap-4 md:pb-4">
            <div className="flex h-14 w-14 items-center justify-center rounded-full bg-gradient-to-br from-[#83CD2D] to-[#70b525] text-xl font-semibold text-white shadow-md md:h-16 md:w-16">
              {initials}
            </div>
            <div className="min-w-0">
              <h2 className="truncate text-lg font-semibold text-gray-900 md:text-xl">
                {group.name}
              </h2>
              <p className="truncate text-sm text-gray-500">
                {group.room_name ?? "Kein Gruppenraum zugewiesen"}
              </p>
            </div>
          </div>

          {/* Details */}
          <div className="space-y-3 md:space-y-4">
            <div className="rounded-xl border border-gray-100 bg-green-50/30 p-3 md:p-4">
              <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
                <svg
                  className="h-3.5 w-3.5 text-green-600 md:h-4 md:w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
                  />
                </svg>
                Gruppendetails
              </h3>
              <dl className="grid grid-cols-1 gap-x-3 gap-y-2 sm:grid-cols-2 md:gap-x-4 md:gap-y-3">
                <div>
                  <dt className="text-xs text-gray-500">Gruppenraum</dt>
                  <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                    {group.room_name ?? "Kein Gruppenraum"}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-gray-500">Anzahl Schüler</dt>
                  <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                    {group.student_count ?? 0}
                  </dd>
                </div>
                <div className="sm:col-span-2">
                  <dt className="text-xs text-gray-500">Gruppenleitung</dt>
                  <dd className="mt-0.5 text-xs break-words whitespace-pre-wrap text-gray-700 md:text-sm">
                    {group.supervisors && group.supervisors.length > 0
                      ? group.supervisors.map((s) => s.name).join(", ")
                      : "Keine Gruppenleitung zugewiesen"}
                  </dd>
                </div>
              </dl>
            </div>
          </div>

          <DetailModalActions
            onEdit={onEdit}
            onDelete={onDelete}
            entityName={group.name}
            entityType="Gruppe"
          />
        </div>
      )}
    </Modal>
  );
}
