"use client";

import { useState, useEffect, useCallback } from "react";
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

interface StudentEditModalProps {
    isOpen: boolean;
    onClose: () => void;
    student: Student | null;
    onSave: (data: Partial<Student>) => Promise<void>;
    loading?: boolean;
    groups?: Array<{ value: string; label: string }>;
}

export function StudentEditModal({
    isOpen,
    onClose,
    student,
    onSave,
    loading = false,
    groups = []
}: StudentEditModalProps) {
    const [formData, setFormData] = useState<Partial<Student>>({});
    const [errors, setErrors] = useState<Record<string, string>>({});
    const [saveLoading, setSaveLoading] = useState(false);
    const [guardians, setGuardians] = useState<Guardian[]>([]);
    const [additionalInfo, setAdditionalInfo] = useState<string>("");

    // Parse guardians and additional info from student data
    const isGuardian = useCallback((g: unknown): g is Guardian => {
        return typeof g === 'object' && g !== null &&
            typeof (g as { id?: unknown }).id === 'string' &&
            typeof (g as { name?: unknown }).name === 'string';
    }, []);
    const isPayload = useCallback((x: unknown): x is { guardians: Guardian[]; additionalInfo?: string } => {
        if (typeof x !== 'object' || x === null) return false;
        const arr = (x as { guardians?: unknown }).guardians;
        return Array.isArray(arr) && arr.every(isGuardian);
    }, [isGuardian]);
    const parseGuardiansAndInfo = useCallback((student: Student): { guardians: Guardian[], additionalInfo: string } => {
        try {
            // Try to parse from extra_info if it contains guardian data
            if (student.extra_info) {
                const parsed: unknown = JSON.parse(student.extra_info);
                if (isPayload(parsed)) {
                    return {
                        guardians: parsed.guardians,
                        additionalInfo: parsed.additionalInfo ?? ""
                    };
                }
            }
        } catch {
            // If parsing fails, fall through to legacy format
        }

        // Legacy format: single guardian from existing fields
        const legacyGuardian: Guardian = {
            id: '1',
            name: student.name_lg ?? "",
            contact: student.contact_lg ?? "",
            email: student.guardian_email ?? "",
            phone: student.guardian_phone ?? "",
            relationship: "Erziehungsberechtigter"
        };

        // Check if we have guardian data or just extra_info text
        const hasGuardianData = [legacyGuardian.name, legacyGuardian.contact, legacyGuardian.email, legacyGuardian.phone]
            .some(v => typeof v === 'string' && v.length > 0);

        return {
            guardians: hasGuardianData ? [legacyGuardian] : [],
            additionalInfo: !hasGuardianData && student.extra_info ? student.extra_info : ""
        };
    }, [isPayload]);

    // Initialize form data when student changes
    useEffect(() => {
        if (student) {
            const { guardians: parsedGuardians, additionalInfo: parsedInfo } = parseGuardiansAndInfo(student);

            setFormData({
                first_name: student.first_name ?? "",
                second_name: student.second_name ?? "",
                school_class: student.school_class ?? "",
                group_id: student.group_id ?? "",
                birthday: student.birthday ?? "",
                health_info: student.health_info ?? "",
                supervisor_notes: student.supervisor_notes ?? "",
                extra_info: student.extra_info ?? "",
                privacy_consent_accepted: student.privacy_consent_accepted ?? false,
                data_retention_days: student.data_retention_days ?? 30,
                bus: student.bus ?? false,
            });
            setGuardians(parsedGuardians);
            setAdditionalInfo(parsedInfo);
            setErrors({});
        }
    }, [student, parseGuardiansAndInfo]);

    const validateForm = (): boolean => {
        const newErrors: Record<string, string> = {};

        if (!formData.first_name?.trim()) {
            newErrors.first_name = "Vorname ist erforderlich";
        }
        if (!formData.second_name?.trim()) {
            newErrors.second_name = "Nachname ist erforderlich";
        }
        if (formData.data_retention_days && (formData.data_retention_days < 1 || formData.data_retention_days > 31)) {
            newErrors.data_retention_days = "Aufbewahrungsdauer muss zwischen 1 und 31 Tagen liegen";
        }

        setErrors(newErrors);
        return Object.keys(newErrors).length === 0;
    };

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();

        if (!validateForm()) {
            return;
        }

        try {
            setSaveLoading(true);

            // Prepare guardian data for backend
            const primaryGuardian = guardians[0];
            const updatedFormData = {
                ...formData,
                // Map first guardian to legacy fields for backward compatibility
                name_lg: primaryGuardian?.name ?? "",
                contact_lg: primaryGuardian?.contact ?? "",
                guardian_email: primaryGuardian?.email ?? "",
                guardian_phone: primaryGuardian?.phone ?? "",
                // Store all guardians AND additional info in extra_info
                extra_info: JSON.stringify({
                    guardians: guardians,
                    additionalInfo: additionalInfo
                })
            };

            await onSave(updatedFormData);
        } catch (error) {
            console.error("Error saving student:", error);
            setErrors({ submit: "Fehler beim Speichern. Bitte versuchen Sie es erneut." });
        } finally {
            setSaveLoading(false);
        }
    };

    const addGuardian = () => {
        const newGuardian: Guardian = {
            id: Date.now().toString(),
            name: "",
            contact: "",
            email: "",
            phone: "",
            relationship: "Erziehungsberechtigter"
        };
        setGuardians([...guardians, newGuardian]);
    };

    const removeGuardian = (id: string) => {
        setGuardians(guardians.filter(g => g.id !== id));
    };

    const updateGuardian = (id: string, field: keyof Guardian, value: string) => {
        setGuardians(guardians.map(g =>
            g.id === id ? { ...g, [field]: value } : g
        ));
    };

    const handleChange = (field: keyof Student, value: string | boolean | number | null) => {
        setFormData(prev => ({ ...prev, [field]: value }));
        // Clear error for this field
        if (errors[field]) {
            setErrors(prev => {
                const newErrors = { ...prev };
                delete newErrors[field];
                return newErrors;
            });
        }
    };

    if (!student) return null;

    return (
        <Modal
            isOpen={isOpen}
            onClose={onClose}
            title="Schüler bearbeiten"
        >
            {loading ? (
                <div className="flex items-center justify-center py-12">
                    <div className="flex flex-col items-center gap-4">
                        <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-[#5080D8]"></div>
                        <p className="text-gray-600">Daten werden geladen...</p>
                    </div>
                </div>
            ) : (
                <form onSubmit={handleSubmit} className="space-y-4 md:space-y-6">
                    {/* Submit Error */}
                    {errors.submit && (
                        <div className="rounded-lg border border-red-200 bg-red-50 p-2 md:p-3">
                            <p className="text-xs md:text-sm text-red-800">{errors.submit}</p>
                        </div>
                    )}

                    {/* Personal Information */}
                    <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
                        <h3 className="text-xs md:text-sm font-semibold text-gray-900 mb-3 md:mb-4 flex items-center gap-2">
                            <svg className="h-3.5 w-3.5 md:h-4 md:w-4 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                            </svg>
                            Persönliche Daten
                        </h3>
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-3 md:gap-4">
                            <div>
                                <label className="block text-xs font-medium text-gray-700 mb-1">
                                    Vorname <span className="text-red-500">*</span>
                                </label>
                                <input
                                    type="text"
                                    value={formData.first_name ?? ""}
                                    onChange={(e) => handleChange("first_name", e.target.value)}
                                    className={`block w-full rounded-lg border px-3 py-2 text-sm transition-colors ${
                                        errors.first_name
                                            ? "border-red-300 bg-red-50"
                                            : "border-gray-200 bg-white focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                                    }`}
                                    placeholder="Max"
                                />
                                {errors.first_name && (
                                    <p className="mt-1 text-xs text-red-600">{errors.first_name}</p>
                                )}
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-gray-700 mb-1">
                                    Nachname <span className="text-red-500">*</span>
                                </label>
                                <input
                                    type="text"
                                    value={formData.second_name ?? ""}
                                    onChange={(e) => handleChange("second_name", e.target.value)}
                                    className={`block w-full rounded-lg border px-3 py-2 text-sm transition-colors ${
                                        errors.second_name
                                            ? "border-red-300 bg-red-50"
                                            : "border-gray-200 bg-white focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                                    }`}
                                    placeholder="Mustermann"
                                />
                                {errors.second_name && (
                                    <p className="mt-1 text-xs text-red-600">{errors.second_name}</p>
                                )}
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-gray-700 mb-1">Klasse</label>
                                <input
                                    type="text"
                                    value={formData.school_class ?? ""}
                                    onChange={(e) => handleChange("school_class", e.target.value)}
                                    className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] transition-colors"
                                    placeholder="5A"
                                />
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-gray-700 mb-1">Gruppe</label>
                                <div className="relative">
                                    <select
                                        value={formData.group_id ?? ""}
                                        onChange={(e) => {
                                            const v = e.target.value;
                                            handleChange("group_id", v === "" ? null : v);
                                        }}
                                        className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 pr-10 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] transition-colors appearance-none"
                                    >
                                        <option value="">Keine Gruppe</option>
                                        {groups.map(group => (
                                            <option key={group.value} value={group.value}>
                                                {group.label}
                                            </option>
                                        ))}
                                    </select>
                                    <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center px-3 text-gray-500">
                                        <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                                        </svg>
                                    </div>
                                </div>
                            </div>
                            <div>
                                <label className="block text-xs font-medium text-gray-700 mb-1">Geburtstag</label>
                                <input
                                    type="date"
                                    value={formData.birthday ?? ""}
                                    onChange={(e) => handleChange("birthday", e.target.value)}
                                    className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] transition-colors"
                                />
                            </div>
                        </div>
                    </div>

                    {/* Guardian Information */}
                    <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
                        <div className="flex items-center justify-between mb-3 md:mb-4">
                            <h3 className="text-xs md:text-sm font-semibold text-gray-900 flex items-center gap-2">
                                <svg className="h-3.5 w-3.5 md:h-4 md:w-4 text-purple-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                                </svg>
                                Erziehungsberechtigte
                            </h3>
                            <button
                                type="button"
                                onClick={addGuardian}
                                className="flex items-center gap-1 px-3 py-1.5 text-xs font-medium text-purple-700 bg-purple-100 hover:bg-purple-200 rounded-lg transition-colors"
                            >
                                <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                                </svg>
                                Hinzufügen
                            </button>
                        </div>

                        {guardians.length === 0 ? (
                            <div className="text-center py-6 text-gray-500 text-sm">
                                <svg className="h-12 w-12 mx-auto mb-2 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                                </svg>
                                Noch keine Erziehungsberechtigten hinzugefügt
                            </div>
                        ) : (
                            <div className="space-y-4">
                                {guardians.map((guardian, index) => (
                                    <div key={guardian.id} className="p-4 bg-white rounded-lg border border-purple-100">
                                        <div className="flex items-center justify-between mb-3">
                                            <span className="text-xs font-semibold text-purple-700">
                                                Erziehungsberechtigter {index + 1}
                                            </span>
                                            {guardians.length > 1 && (
                                                <button
                                                    type="button"
                                                    onClick={() => removeGuardian(guardian.id)}
                                                    className="text-red-600 hover:text-red-700 hover:bg-red-50 p-1 rounded transition-colors"
                                                    aria-label="Entfernen"
                                                >
                                                    <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                                                    </svg>
                                                </button>
                                            )}
                                        </div>
                                        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                                            <div>
                                                <label className="block text-xs font-medium text-gray-700 mb-1">Name</label>
                                                <input
                                                    type="text"
                                                    value={guardian.name}
                                                    onChange={(e) => updateGuardian(guardian.id, "name", e.target.value)}
                                                    className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] transition-colors"
                                                    placeholder="Maria Mustermann"
                                                />
                                            </div>
                                            <div>
                                                <label className="block text-xs font-medium text-gray-700 mb-1">Verhältnis</label>
                                                <div className="relative">
                                                    <select
                                                        value={guardian.relationship}
                                                        onChange={(e) => updateGuardian(guardian.id, "relationship", e.target.value)}
                                                        className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 pr-10 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] transition-colors appearance-none"
                                                    >
                                                        <option value="Erziehungsberechtigter">Erziehungsberechtigter</option>
                                                        <option value="Mutter">Mutter</option>
                                                        <option value="Vater">Vater</option>
                                                        <option value="Großmutter">Großmutter</option>
                                                        <option value="Großvater">Großvater</option>
                                                        <option value="Onkel">Onkel</option>
                                                        <option value="Tante">Tante</option>
                                                        <option value="Vormund">Vormund</option>
                                                        <option value="Sonstiges">Sonstiges</option>
                                                    </select>
                                                    <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center px-3 text-gray-500">
                                                        <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                                                        </svg>
                                                    </div>
                                                </div>
                                            </div>
                                            <div>
                                                <label className="block text-xs font-medium text-gray-700 mb-1">E-Mail</label>
                                                <input
                                                    type="email"
                                                    value={guardian.email}
                                                    onChange={(e) => updateGuardian(guardian.id, "email", e.target.value)}
                                                    className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] transition-colors"
                                                    placeholder="maria@example.com"
                                                />
                                            </div>
                                            <div>
                                                <label className="block text-xs font-medium text-gray-700 mb-1">Telefon</label>
                                                <input
                                                    type="tel"
                                                    value={guardian.phone}
                                                    onChange={(e) => updateGuardian(guardian.id, "phone", e.target.value)}
                                                    className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] transition-colors"
                                                    placeholder="+49 123 456789"
                                                />
                                            </div>
                                            <div className="md:col-span-2">
                                                <label className="block text-xs font-medium text-gray-700 mb-1">Zusätzliche Kontaktinfo</label>
                                                <input
                                                    type="text"
                                                    value={guardian.contact}
                                                    onChange={(e) => updateGuardian(guardian.id, "contact", e.target.value)}
                                                    className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] transition-colors"
                                                    placeholder="Weitere Kontaktinformationen..."
                                                />
                                            </div>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        )}
                    </div>

                    {/* Health Information */}
                    <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
                        <h3 className="text-xs md:text-sm font-semibold text-gray-900 mb-3 md:mb-4 flex items-center gap-2">
                            <svg className="h-3.5 w-3.5 md:h-4 md:w-4 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z" />
                            </svg>
                            Gesundheitsinformationen
                        </h3>
                        <textarea
                            value={formData.health_info ?? ""}
                            onChange={(e) => handleChange("health_info", e.target.value)}
                            rows={3}
                            className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs md:text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] transition-colors resize-none"
                            placeholder="Allergien, Medikamente, gesundheitliche Einschränkungen..."
                        />
                    </div>

                    {/* Supervisor Notes */}
                    <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
                        <h3 className="text-xs md:text-sm font-semibold text-gray-900 mb-3 md:mb-4 flex items-center gap-2">
                            <svg className="h-3.5 w-3.5 md:h-4 md:w-4 text-amber-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                            </svg>
                            Betreuernotizen
                        </h3>
                        <textarea
                            value={formData.supervisor_notes ?? ""}
                            onChange={(e) => handleChange("supervisor_notes", e.target.value)}
                            rows={3}
                            className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs md:text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] transition-colors resize-none"
                            placeholder="Interne Notizen für Betreuer..."
                        />
                    </div>

                    {/* Additional Information */}
                    <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
                        <h3 className="text-xs md:text-sm font-semibold text-gray-900 mb-3 md:mb-4 flex items-center gap-2">
                            <svg className="h-3.5 w-3.5 md:h-4 md:w-4 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                            </svg>
                            Elternnotizen
                        </h3>
                        <textarea
                            value={additionalInfo}
                            onChange={(e) => setAdditionalInfo(e.target.value)}
                            rows={3}
                            className="block w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs md:text-sm focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8] transition-colors resize-none"
                            placeholder="Weitere Informationen über den Schüler..."
                        />
                    </div>

                    {/* Privacy & Data Retention */}
                    <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
                        <h3 className="text-xs md:text-sm font-semibold text-gray-900 mb-3 md:mb-4 flex items-center gap-2">
                            <svg className="h-3.5 w-3.5 md:h-4 md:w-4 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                            </svg>
                            Datenschutz
                        </h3>
                        <div className="space-y-4">
                            <label className="flex items-start gap-3 cursor-pointer group">
                                <input
                                    type="checkbox"
                                    checked={formData.privacy_consent_accepted ?? false}
                                    onChange={(e) => handleChange("privacy_consent_accepted", e.target.checked)}
                                    className="mt-0.5 h-4 w-4 rounded border-gray-300 text-[#5080D8] focus:ring-[#5080D8]"
                                />
                                <span className="text-sm text-gray-700 group-hover:text-gray-900 transition-colors">
                                    Einwilligung zur Datenverarbeitung erteilt
                                </span>
                            </label>
                            <div>
                                <label className="block text-xs font-medium text-gray-700 mb-1">
                                    Aufbewahrungsdauer (Tage)
                                </label>
                                <input
                                    type="number"
                                    min="1"
                                    max="31"
                                    value={formData.data_retention_days ?? 30}
                                    onChange={(e) => {
                                        const v = parseInt(e.target.value, 10);
                                        handleChange("data_retention_days", Number.isNaN(v) ? 30 : v);
                                    }}
                                    className={`block w-full rounded-lg border px-3 py-2 text-sm transition-colors ${
                                        errors.data_retention_days
                                            ? "border-red-300 bg-red-50"
                                            : "border-gray-200 bg-white focus:border-[#5080D8] focus:ring-1 focus:ring-[#5080D8]"
                                    }`}
                                />
                                {errors.data_retention_days && (
                                    <p className="mt-1 text-xs text-red-600">{errors.data_retention_days}</p>
                                )}
                                <p className="mt-1 text-xs text-gray-500">
                                    Daten werden nach dieser Zeit automatisch gelöscht (1-31 Tage)
                                </p>
                            </div>
                        </div>
                    </div>

                    {/* Bus Status */}
                    <div className="rounded-xl border border-orange-200 bg-orange-50 p-4">
                        <label className="flex items-center gap-3 cursor-pointer group">
                            <input
                                type="checkbox"
                                checked={formData.bus ?? false}
                                onChange={(e) => handleChange("bus", e.target.checked)}
                                className="h-4 w-4 rounded border-gray-300 text-orange-600 focus:ring-orange-600"
                            />
                            <div className="flex items-center gap-2">
                                <svg className="h-5 w-5 text-orange-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
                                </svg>
                                <span className="text-sm font-medium text-orange-900">Fährt mit dem Bus</span>
                            </div>
                        </label>
                    </div>

                    {/* Action Buttons */}
                    <div className="sticky bottom-0 bg-white/95 backdrop-blur-sm flex gap-2 md:gap-3 py-3 md:py-4 border-t border-gray-100 -mx-4 md:-mx-6 -mb-4 md:-mb-6 px-4 md:px-6 mt-4 md:mt-6">
                        <button
                            type="button"
                            onClick={onClose}
                            disabled={saveLoading}
                            className="flex-1 px-3 md:px-4 py-2 rounded-lg border border-gray-300 text-xs md:text-sm font-medium text-gray-700 hover:bg-gray-50 hover:border-gray-400 hover:shadow-md md:hover:scale-105 active:scale-100 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200"
                        >
                            Abbrechen
                        </button>
                        <button
                            type="submit"
                            disabled={saveLoading}
                            className="flex-1 px-3 md:px-4 py-2 rounded-lg bg-gray-900 text-xs md:text-sm font-medium text-white hover:bg-gray-700 hover:shadow-lg md:hover:scale-105 active:scale-100 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200"
                        >
                            {saveLoading ? (
                                <span className="flex items-center justify-center gap-2">
                                    <svg className="animate-spin h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
                                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                                    </svg>
                                    Wird gespeichert...
                                </span>
                            ) : (
                                "Speichern"
                            )}
                        </button>
                    </div>
                </form>
            )}
        </Modal>
    );
}
