"use client";

import { useState, useEffect } from "react";
import { Modal } from "~/components/ui/modal";
import type { Teacher } from "@/lib/teacher-api";

interface TeacherDetailModalProps {
    isOpen: boolean;
    onClose: () => void;
    teacher: Teacher | null;
    onEdit: () => void;
    onDelete: () => void;
    loading?: boolean;
}

export function TeacherDetailModal({
    isOpen,
    onClose,
    teacher,
    onEdit,
    onDelete,
    loading = false
}: TeacherDetailModalProps) {
    const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

    // Reset confirmation state when modal closes
    useEffect(() => {
        if (!isOpen) {
            setShowDeleteConfirm(false);
        }
    }, [isOpen]);

    if (!teacher) return null;

    const handleDeleteClick = () => {
        setShowDeleteConfirm(true);
    };

    const handleCancelDelete = () => {
        setShowDeleteConfirm(false);
    };

    const handleConfirmDelete = () => {
        setShowDeleteConfirm(false);
        onDelete();
    };

    return (
        <Modal
            isOpen={isOpen}
            onClose={onClose}
            title="" // No title, we'll use custom header
        >
            {loading ? (
                <div className="flex items-center justify-center py-12">
                    <div className="flex flex-col items-center gap-4">
                        <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-[#F78C10]"></div>
                        <p className="text-gray-600">Daten werden geladen...</p>
                    </div>
                </div>
            ) : showDeleteConfirm ? (
                /* Delete Confirmation View */
                <div className="space-y-6">
                    {/* Warning Icon */}
                    <div className="flex justify-center">
                        <div className="w-16 h-16 rounded-full bg-red-100 flex items-center justify-center">
                            <svg className="w-8 h-8 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                            </svg>
                        </div>
                    </div>

                    {/* Confirmation Message */}
                    <div className="text-center space-y-3">
                        <h3 className="text-xl font-bold text-gray-900">Betreuer löschen?</h3>
                        <p className="text-sm text-gray-700">
                            Möchten Sie den Betreuer <strong>{teacher.first_name} {teacher.last_name}</strong> wirklich löschen?
                        </p>
                        <p className="text-sm text-red-600 font-medium">
                            Diese Aktion kann nicht rückgängig gemacht werden.
                        </p>
                    </div>

                    {/* Action Buttons */}
                    <div className="flex gap-3 pt-4 border-t border-gray-100">
                        <button
                            type="button"
                            onClick={handleCancelDelete}
                            className="flex-1 px-4 py-2 rounded-lg border border-gray-300 text-sm font-medium text-gray-700 hover:bg-gray-50 hover:border-gray-400 hover:shadow-md hover:scale-105 active:scale-100 transition-all duration-200"
                        >
                            Abbrechen
                        </button>
                        <button
                            type="button"
                            onClick={handleConfirmDelete}
                            className="flex-1 px-4 py-2 rounded-lg bg-red-600 text-sm font-medium text-white hover:bg-red-700 hover:shadow-lg hover:scale-105 active:scale-100 transition-all duration-200"
                        >
                            <span className="flex items-center justify-center gap-2">
                                <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                                </svg>
                                Löschen
                            </span>
                        </button>
                    </div>
                </div>
            ) : (
                /* Detail View */
                <div className="space-y-4 md:space-y-6">
                    {/* Header with Avatar */}
                    <div className="flex items-center gap-3 md:gap-4 pb-3 md:pb-4 border-b border-gray-100">
                        <div className="h-14 w-14 md:h-16 md:w-16 rounded-full bg-gradient-to-br from-[#F78C10] to-[#e57a00] flex items-center justify-center text-white text-lg md:text-xl font-bold shadow-lg flex-shrink-0">
                            {teacher.first_name?.[0]}{teacher.last_name?.[0]}
                        </div>
                        <div className="flex-1 min-w-0">
                            <h2 className="text-lg md:text-xl font-bold text-gray-900 truncate">
                                {teacher.first_name} {teacher.last_name}
                            </h2>
                        </div>
                    </div>

                    {/* Teacher Information Sections */}
                    <div className="space-y-4">
                        {/* Personal Information */}
                        <div className="rounded-xl border border-gray-100 bg-orange-50/30 p-3 md:p-4">
                            <h3 className="text-xs md:text-sm font-semibold text-gray-900 mb-2 md:mb-3 flex items-center gap-2">
                                <svg className="h-3.5 w-3.5 md:h-4 md:w-4 text-orange-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                                </svg>
                                Persönliche Daten
                            </h3>
                            <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-3 md:gap-x-4 gap-y-2 md:gap-y-3">
                                <div>
                                    <dt className="text-xs text-gray-500">Vorname</dt>
                                    <dd className="text-sm font-medium text-gray-900 mt-0.5 break-words">{teacher.first_name}</dd>
                                </div>
                                <div>
                                    <dt className="text-xs text-gray-500">Nachname</dt>
                                    <dd className="text-sm font-medium text-gray-900 mt-0.5 break-words">{teacher.last_name}</dd>
                                </div>
                                {teacher.email && (
                                    <div className="col-span-1 sm:col-span-2">
                                        <dt className="text-xs text-gray-500">E-Mail</dt>
                                        <dd className="text-xs md:text-sm font-medium text-gray-900 mt-0.5 break-all">{teacher.email}</dd>
                                    </div>
                                )}
                                {teacher.tag_id && (
                                    <div className="col-span-1 sm:col-span-2">
                                        <dt className="text-xs text-gray-500">RFID-Karte</dt>
                                        <dd className="text-xs md:text-sm font-mono text-gray-600 mt-0.5 break-all">{teacher.tag_id}</dd>
                                    </div>
                                )}
                                {teacher.id && (
                                    <div className="col-span-1 sm:col-span-2">
                                        <dt className="text-xs text-gray-500">Betreuer-ID</dt>
                                        <dd className="text-xs md:text-sm font-mono text-gray-600 mt-0.5 break-all">{teacher.id}</dd>
                                    </div>
                                )}
                            </dl>
                        </div>

                        {/* Professional Information */}
                        {(() => {
                            const trimmedRole = teacher.role?.trim() ?? "";
                            const trimmedQualifications = teacher.qualifications?.trim() ?? "";
                            const hasProfessionalInfo = [trimmedRole, trimmedQualifications].some((value) => value.length > 0);

                            if (!hasProfessionalInfo) {
                                return null;
                            }

                            return (
                                <div className="rounded-xl border border-gray-100 bg-orange-50/30 p-3 md:p-4">
                                    <h3 className="text-xs md:text-sm font-semibold text-gray-900 mb-2 md:mb-3 flex items-center gap-2">
                                        <svg className="h-3.5 w-3.5 md:h-4 md:w-4 text-orange-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 13.255A23.931 23.931 0 0112 15c-3.183 0-6.22-.62-9-1.745M16 6V4a2 2 0 00-2-2h-4a2 2 0 00-2 2v2m4 6h.01M5 20h14a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                                        </svg>
                                        Berufliche Informationen
                                    </h3>
                                    <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-3 md:gap-x-4 gap-y-2 md:gap-y-3">
                                        {trimmedRole && (
                                            <div>
                                                <dt className="text-xs text-gray-500">Rolle</dt>
                                                <dd className="text-sm font-medium text-gray-900 mt-0.5 break-words">{trimmedRole}</dd>
                                            </div>
                                        )}
                                        {trimmedQualifications && (
                                            <div className="col-span-1 sm:col-span-2">
                                                <dt className="text-xs text-gray-500">Qualifikationen</dt>
                                                <dd className="text-xs md:text-sm text-gray-700 mt-0.5 whitespace-pre-wrap break-words">{trimmedQualifications}</dd>
                                            </div>
                                        )}
                                    </dl>
                                </div>
                            );
                        })()}

                        {/* Staff Notes */}
                        {(() => {
                            const trimmedNotes = teacher.staff_notes?.trim() ?? "";
                            if (trimmedNotes.length === 0) {
                                return null;
                            }
                            return (
                            <div className="rounded-xl border border-gray-100 bg-orange-50/30 p-3 md:p-4">
                                <h3 className="text-xs md:text-sm font-semibold text-gray-900 mb-2 md:mb-3 flex items-center gap-2">
                                    <svg className="h-3.5 w-3.5 md:h-4 md:w-4 text-orange-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                                    </svg>
                                    Notizen
                                </h3>
                                <p className="text-xs md:text-sm text-gray-700 whitespace-pre-wrap break-words">{trimmedNotes}</p>
                            </div>
                            );
                        })()}

                        {/* Timestamps */}
                        {(teacher.created_at ?? teacher.updated_at) && (
                            <div className="rounded-xl border border-gray-100 bg-gray-50 p-3 md:p-4">
                                <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-3 md:gap-x-4 gap-y-2 md:gap-y-3">
                                    {teacher.created_at && (
                                        <div>
                                            <dt className="text-xs text-gray-500">Erstellt am</dt>
                                            <dd className="text-xs md:text-sm font-medium text-gray-900 mt-0.5">
                                                {new Date(teacher.created_at).toLocaleDateString("de-DE", {
                                                    day: "2-digit",
                                                    month: "2-digit",
                                                    year: "numeric",
                                                    hour: "2-digit",
                                                    minute: "2-digit",
                                                })}
                                            </dd>
                                        </div>
                                    )}
                                    {teacher.updated_at && (
                                        <div>
                                            <dt className="text-xs text-gray-500">Aktualisiert am</dt>
                                            <dd className="text-xs md:text-sm font-medium text-gray-900 mt-0.5">
                                                {new Date(teacher.updated_at).toLocaleDateString("de-DE", {
                                                    day: "2-digit",
                                                    month: "2-digit",
                                                    year: "numeric",
                                                    hour: "2-digit",
                                                    minute: "2-digit",
                                                })}
                                            </dd>
                                        </div>
                                    )}
                                </dl>
                            </div>
                        )}
                    </div>

                    {/* Action Buttons */}
                    <div className="sticky bottom-0 bg-white/95 backdrop-blur-sm flex gap-2 md:gap-3 py-3 md:py-4 border-t border-gray-100 -mx-4 md:-mx-6 -mb-4 md:-mb-6 px-4 md:px-6 mt-4 md:mt-6">
                        <button
                            type="button"
                            onClick={handleDeleteClick}
                            className="px-3 md:px-4 py-2 rounded-lg border border-red-300 text-xs md:text-sm font-medium text-red-700 hover:bg-red-50 hover:border-red-400 hover:shadow-md md:hover:scale-105 active:scale-100 transition-all duration-200"
                        >
                            <span className="flex items-center gap-2">
                                <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                                </svg>
                                Löschen
                            </span>
                        </button>

                        <button
                            type="button"
                            onClick={onEdit}
                            className="flex-1 px-3 md:px-4 py-2 rounded-lg bg-gray-900 text-xs md:text-sm font-medium text-white hover:bg-gray-700 hover:shadow-lg md:hover:scale-105 active:scale-100 transition-all duration-200"
                        >
                            <span className="flex items-center justify-center gap-2">
                                <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                                </svg>
                                Bearbeiten
                            </span>
                        </button>
                    </div>
                </div>
            )}
        </Modal>
    );
}
