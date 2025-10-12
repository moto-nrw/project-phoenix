"use client";

import { useState, useEffect } from "react";
import { Modal } from "~/components/ui/modal";
import type { Student } from "@/lib/api";

interface Guardian {
    id: string;
    name: string;
    contact: string;
    email: string;
    phone: string;
    relationship: string;
}

interface StudentDetailModalProps {
    isOpen: boolean;
    onClose: () => void;
    student: Student | null;
    onEdit: () => void;
    onDelete: () => void;
    loading?: boolean;
}

export function StudentDetailModal({
    isOpen,
    onClose,
    student,
    onEdit,
    onDelete,
    loading = false
}: StudentDetailModalProps) {
    const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

    // Parse guardians from student extra_info
    const parseGuardians = (student: Student): Guardian[] | null => {
        try {
            if (student.extra_info) {
                const parsed = JSON.parse(student.extra_info);
                if (parsed.guardians && Array.isArray(parsed.guardians)) {
                    return parsed.guardians;
                }
            }
        } catch {
            // If parsing fails, return null
        }
        return null;
    };

    const guardians = student ? parseGuardians(student) : null;

    // Reset confirmation state when modal closes
    useEffect(() => {
        if (!isOpen) {
            setShowDeleteConfirm(false);
        }
    }, [isOpen]);

    if (!student) return null;

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
                        <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-[#5080D8]"></div>
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
                        <h3 className="text-xl font-bold text-gray-900">Schüler löschen?</h3>
                        <p className="text-sm text-gray-700">
                            Möchten Sie den Schüler <strong>{student.first_name} {student.second_name}</strong> wirklich löschen?
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
                <div className="space-y-6">
                    {/* Header with Avatar */}
                    <div className="flex items-center gap-4 pb-4 border-b border-gray-100">
                            <div className="h-16 w-16 rounded-full bg-gradient-to-br from-[#5080D8] to-[#4070c8] flex items-center justify-center text-white text-xl font-bold shadow-lg">
                                {student.first_name?.[0]}{student.second_name?.[0]}
                            </div>
                            <div className="flex-1">
                                <h2 className="text-xl font-bold text-gray-900">
                                    {student.first_name} {student.second_name}
                                </h2>
                                {student.school_class && (
                                    <p className="text-sm text-gray-500 mt-0.5">
                                        Klasse {student.school_class}
                                    </p>
                                )}
                            </div>
                        </div>

                        {/* Student Information Sections */}
                        <div className="space-y-4">
                            {/* Personal Information */}
                            <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-4">
                                <h3 className="text-sm font-semibold text-gray-900 mb-3 flex items-center gap-2">
                                    <svg className="h-4 w-4 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                                    </svg>
                                    Persönliche Daten
                                </h3>
                                <dl className="grid grid-cols-2 gap-x-4 gap-y-3">
                                    <div>
                                        <dt className="text-xs text-gray-500">Vorname</dt>
                                        <dd className="text-sm font-medium text-gray-900 mt-0.5">{student.first_name}</dd>
                                    </div>
                                    <div>
                                        <dt className="text-xs text-gray-500">Nachname</dt>
                                        <dd className="text-sm font-medium text-gray-900 mt-0.5">{student.second_name}</dd>
                                    </div>
                                    <div>
                                        <dt className="text-xs text-gray-500">Klasse</dt>
                                        <dd className="text-sm font-medium text-gray-900 mt-0.5">{student.school_class ?? "Nicht angegeben"}</dd>
                                    </div>
                                    <div>
                                        <dt className="text-xs text-gray-500">Gruppe</dt>
                                        <dd className="text-sm font-medium text-gray-900 mt-0.5">{student.group_name ?? "Keine Gruppe"}</dd>
                                    </div>
                                    {student.id && (
                                        <div className="col-span-2">
                                            <dt className="text-xs text-gray-500">Schüler-ID</dt>
                                            <dd className="text-sm font-mono text-gray-600 mt-0.5">{student.id}</dd>
                                        </div>
                                    )}
                                </dl>
                            </div>

                            {/* Guardian Information */}
                            {(guardians && guardians.length > 0) ? (
                                <div className="rounded-xl border border-gray-100 bg-purple-50/30 p-4">
                                    <h3 className="text-sm font-semibold text-gray-900 mb-3 flex items-center gap-2">
                                        <svg className="h-4 w-4 text-purple-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                                        </svg>
                                        Erziehungsberechtigte
                                    </h3>
                                    <div className="space-y-4">
                                        {guardians.map((guardian, index) => (
                                            <div key={guardian.id} className={`${index > 0 ? 'pt-4 border-t border-purple-100' : ''}`}>
                                                <div className="text-xs font-semibold text-purple-700 mb-2">
                                                    {guardian.relationship} {guardians.length > 1 ? `${index + 1}` : ''}
                                                </div>
                                                <dl className="space-y-2">
                                                    {guardian.name && (
                                                        <div>
                                                            <dt className="text-xs text-gray-500">Name</dt>
                                                            <dd className="text-sm font-medium text-gray-900 mt-0.5">{guardian.name}</dd>
                                                        </div>
                                                    )}
                                                    {guardian.email && (
                                                        <div>
                                                            <dt className="text-xs text-gray-500">E-Mail</dt>
                                                            <dd className="text-sm font-medium text-gray-900 mt-0.5">{guardian.email}</dd>
                                                        </div>
                                                    )}
                                                    {guardian.phone && (
                                                        <div>
                                                            <dt className="text-xs text-gray-500">Telefon</dt>
                                                            <dd className="text-sm font-medium text-gray-900 mt-0.5">{guardian.phone}</dd>
                                                        </div>
                                                    )}
                                                    {guardian.contact && (
                                                        <div>
                                                            <dt className="text-xs text-gray-500">Zusätzliche Kontaktinfo</dt>
                                                            <dd className="text-sm font-medium text-gray-900 mt-0.5">{guardian.contact}</dd>
                                                        </div>
                                                    )}
                                                </dl>
                                            </div>
                                        ))}
                                    </div>
                                </div>
                            ) : (student.name_lg || student.contact_lg || student.guardian_email || student.guardian_phone) && (
                                /* Legacy guardian display for backwards compatibility */
                                <div className="rounded-xl border border-gray-100 bg-purple-50/30 p-4">
                                    <h3 className="text-sm font-semibold text-gray-900 mb-3 flex items-center gap-2">
                                        <svg className="h-4 w-4 text-purple-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                                        </svg>
                                        Erziehungsberechtigter
                                    </h3>
                                    <dl className="space-y-3">
                                        {student.name_lg && (
                                            <div>
                                                <dt className="text-xs text-gray-500">Name</dt>
                                                <dd className="text-sm font-medium text-gray-900 mt-0.5">{student.name_lg}</dd>
                                            </div>
                                        )}
                                        {student.contact_lg && (
                                            <div>
                                                <dt className="text-xs text-gray-500">Kontakt</dt>
                                                <dd className="text-sm font-medium text-gray-900 mt-0.5">{student.contact_lg}</dd>
                                            </div>
                                        )}
                                        {student.guardian_email && (
                                            <div>
                                                <dt className="text-xs text-gray-500">E-Mail</dt>
                                                <dd className="text-sm font-medium text-gray-900 mt-0.5">{student.guardian_email}</dd>
                                            </div>
                                        )}
                                        {student.guardian_phone && (
                                            <div>
                                                <dt className="text-xs text-gray-500">Telefon</dt>
                                                <dd className="text-sm font-medium text-gray-900 mt-0.5">{student.guardian_phone}</dd>
                                            </div>
                                        )}
                                    </dl>
                                </div>
                            )}

                            {/* Health Information */}
                            {student.health_info && (
                                <div className="rounded-xl border border-gray-100 bg-red-50/30 p-4">
                                    <h3 className="text-sm font-semibold text-gray-900 mb-3 flex items-center gap-2">
                                        <svg className="h-4 w-4 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z" />
                                        </svg>
                                        Gesundheitsinformationen
                                    </h3>
                                    <p className="text-sm text-gray-700 whitespace-pre-wrap">{student.health_info}</p>
                                </div>
                            )}

                            {/* Supervisor Notes */}
                            {student.supervisor_notes && (
                                <div className="rounded-xl border border-gray-100 bg-amber-50/30 p-4">
                                    <h3 className="text-sm font-semibold text-gray-900 mb-3 flex items-center gap-2">
                                        <svg className="h-4 w-4 text-amber-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                                        </svg>
                                        Betreuernotizen
                                    </h3>
                                    <p className="text-sm text-gray-700 whitespace-pre-wrap">{student.supervisor_notes}</p>
                                </div>
                            )}

                            {/* Additional Information - Only show if extra_info doesn't contain guardian JSON */}
                            {student.extra_info && !guardians && (
                                <div className="rounded-xl border border-gray-100 bg-gray-50/30 p-4">
                                    <h3 className="text-sm font-semibold text-gray-900 mb-3 flex items-center gap-2">
                                        <svg className="h-4 w-4 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                                        </svg>
                                        Zusätzliche Informationen
                                    </h3>
                                    <p className="text-sm text-gray-700 whitespace-pre-wrap">{student.extra_info}</p>
                                </div>
                            )}

                            {/* Privacy & Data Retention */}
                            <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-4">
                                <h3 className="text-sm font-semibold text-gray-900 mb-3 flex items-center gap-2">
                                    <svg className="h-4 w-4 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                                    </svg>
                                    Datenschutz
                                </h3>
                                <dl className="grid grid-cols-2 gap-x-4 gap-y-3">
                                    <div>
                                        <dt className="text-xs text-gray-500">Einwilligung erteilt</dt>
                                        <dd className="text-sm font-medium mt-0.5">
                                            {student.privacy_consent_accepted ? (
                                                <span className="inline-flex items-center gap-1 text-green-700">
                                                    <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                                                    </svg>
                                                    Ja
                                                </span>
                                            ) : (
                                                <span className="inline-flex items-center gap-1 text-gray-900">
                                                    <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                                    </svg>
                                                    Nein
                                                </span>
                                            )}
                                        </dd>
                                    </div>
                                    {student.data_retention_days && (
                                        <div>
                                            <dt className="text-xs text-gray-500">Aufbewahrungsdauer</dt>
                                            <dd className="text-sm font-medium text-gray-900 mt-0.5">{student.data_retention_days} Tage</dd>
                                        </div>
                                    )}
                                </dl>
                            </div>

                            {/* Bus Status */}
                            {student.bus && (
                                <div className="rounded-xl border border-orange-200 bg-orange-50 p-4">
                                    <div className="flex items-center gap-2">
                                        <svg className="h-5 w-5 text-orange-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
                                        </svg>
                                        <span className="text-sm font-medium text-orange-900">Fährt mit dem Bus</span>
                                    </div>
                                </div>
                            )}
                        </div>

                        {/* Action Buttons */}
                        <div className="flex gap-3 pt-4 border-t border-gray-100">
                            <button
                                type="button"
                                onClick={handleDeleteClick}
                                className="px-4 py-2 rounded-lg border border-red-300 text-sm font-medium text-red-700 hover:bg-red-50 hover:border-red-400 hover:shadow-md hover:scale-105 active:scale-100 transition-all duration-200"
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
                                className="flex-1 px-4 py-2 rounded-lg bg-gray-900 text-sm font-medium text-white hover:bg-gray-700 hover:shadow-lg hover:scale-105 active:scale-100 transition-all duration-200"
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
                )
            }
        </Modal>
    );
}
