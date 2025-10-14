"use client";

import { Modal } from "~/components/ui/modal";
import type { Activity } from "@/lib/activity-helpers";

interface ActivityDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  activity: Activity | null;
  onEdit: () => void;
  onDelete: () => void;
  onManageStudents: () => void;
  onManageTimes: () => void;
  loading?: boolean;
}

export function ActivityDetailModal({
  isOpen,
  onClose,
  activity,
  onEdit,
  onDelete,
  onManageStudents,
  onManageTimes,
  loading = false
}: ActivityDetailModalProps) {
  if (!activity) return null;

  const initials = (activity.name?.slice(0,2) ?? 'AG').toUpperCase();

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
          <div className="flex items-center gap-3 md:gap-4 pb-3 md:pb-4 border-b border-gray-100">
            <div className="h-14 w-14 md:h-16 md:w-16 rounded-full bg-gradient-to-br from-[#FF3130] to-[#e02020] flex items-center justify-center text-white text-xl font-semibold shadow-md">
              {initials}
            </div>
            <div className="min-w-0">
              <h2 className="text-lg md:text-xl font-semibold text-gray-900 truncate">{activity.name}</h2>
              <p className="text-sm text-gray-500 truncate">{activity.category_name ?? 'Keine Kategorie'}</p>
            </div>
          </div>

          {/* Details */}
          <div className="space-y-3 md:space-y-4">
            <div className="rounded-xl border border-gray-100 bg-red-50/30 p-3 md:p-4">
              <h3 className="text-xs md:text-sm font-semibold text-gray-900 mb-2 md:mb-3 flex items-center gap-2">
                <svg className="h-3.5 w-3.5 md:h-4 md:w-4 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" /></svg>
                Aktivitätsdetails
              </h3>
              <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-3 md:gap-x-4 gap-y-2 md:gap-y-3">
                <div>
                  <dt className="text-xs text-gray-500">Kategorie</dt>
                  <dd className="text-sm font-medium text-gray-900 mt-0.5 break-words">{activity.category_name ?? 'Keine Kategorie'}</dd>
                </div>
                <div>
                  <dt className="text-xs text-gray-500">Teilnehmer</dt>
                  <dd className="text-sm font-medium text-gray-900 mt-0.5 break-words">{(activity.participant_count ?? 0)}/{activity.max_participant}</dd>
                </div>
                {activity.supervisor_name && (
                  <div className="sm:col-span-2">
                    <dt className="text-xs text-gray-500">Hauptbetreuer</dt>
                    <dd className="text-xs md:text-sm text-gray-700 mt-0.5 whitespace-pre-wrap break-words">{activity.supervisor_name}</dd>
                  </div>
                )}
              </dl>
            </div>
          </div>

          {/* Actions */}
          <div className="sticky bottom-0 bg-white/95 backdrop-blur-sm flex flex-wrap gap-2 md:gap-3 py-3 md:py-4 border-t border-gray-100 -mx-4 md:-mx-6 -mb-4 md:-mb-6 px-4 md:px-6 mt-4 md:mt-6">
            <button type="button" onClick={onDelete} className="px-3 md:px-4 py-2 rounded-lg border border-red-300 text-xs md:text-sm font-medium text-red-700 hover:bg-red-50 hover:border-red-400 hover:shadow-md md:hover:scale-105 active:scale-100 transition-all duration-200">Löschen</button>
            <button type="button" onClick={onManageStudents} className="px-3 md:px-4 py-2 rounded-lg bg-blue-600 text-xs md:text-sm font-medium text-white hover:bg-blue-700 hover:shadow-lg md:hover:scale-105 active:scale-100 transition-all duration-200">Schüler verwalten</button>
            <button type="button" onClick={onManageTimes} className="px-3 md:px-4 py-2 rounded-lg bg-green-600 text-xs md:text-sm font-medium text-white hover:bg-green-700 hover:shadow-lg md:hover:scale-105 active:scale-100 transition-all duration-200">Zeiten verwalten</button>
            <button type="button" onClick={onEdit} className="flex-1 px-3 md:px-4 py-2 rounded-lg bg-gray-900 text-xs md:text-sm font-medium text-white hover:bg-gray-700 hover:shadow-lg md:hover:scale-105 active:scale-100 transition-all duration-200">Bearbeiten</button>
          </div>
        </div>
      )}
    </Modal>
  );
}

