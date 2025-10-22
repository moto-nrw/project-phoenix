"use client";

import { useState, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import Link from "next/link";
import { StudentEditModal } from "~/components/students/student-edit-modal";
import type { Student } from "@/lib/api";
import { Loading } from "~/components/ui/loading";

// Status types for CSV rows
type RowStatus = "new" | "existing" | "error" | "updated";

interface CSVStudent {
    row: number;
    status: RowStatus;
    errors: string[];
    first_name: string;
    second_name: string;
    school_class: string;
    group_name: string;
    birthday: string;
    guardian_name: string;
    guardian_relationship: string;
    guardian_email: string;
    guardian_phone: string;
    guardian_contact: string;
    health_info: string;
    supervisor_notes: string;
    additional_info: string;
    privacy_consent: boolean;
    data_retention_days: number;
    bus: boolean;
}

export default function StudentCSVImportPage() {
    const [uploadedFile, setUploadedFile] = useState<File | null>(null);
    const [csvData, setCsvData] = useState<CSVStudent[]>([]);
    const [isDragging, setIsDragging] = useState(false);
    const [editingRowIndex, setEditingRowIndex] = useState<number | null>(null);
    const [showEditModal, setShowEditModal] = useState(false);
    const [editingStudent, setEditingStudent] = useState<Student | null>(null);

    const { status } = useSession({
        required: true,
        onUnauthenticated() {
            redirect("/");
        },
    });

    // Handle file download
    const handleDownloadTemplate = () => {
        const csvContent = [
            "Vorname,Nachname,Klasse,Gruppe,Geburtstag,Erz.Name,Erz.Verhältnis,Erz.Email,Erz.Telefon,Erz.Kontakt,Gesundheitsinfo,Betreuernotizen,Zusatzinfo,Datenschutz,Aufbewahrung(Tage),Bus",
            "Max,Mustermann,1A,Gruppe A,2015-08-15,Maria Mustermann,Mutter,maria@example.com,+49123456789,Auch per WhatsApp erreichbar,Keine Allergien,Sehr ruhiges Kind,Vegetarisch,Ja,30,Ja",
            "Anna,Schmidt,1B,Gruppe B,2016-03-22,Peter Schmidt,Vater,peter@example.com,+49987654321,Nur Notfälle,Laktoseintoleranz,Braucht mehr Aufmerksamkeit,,Ja,30,Nein"
        ].join('\n');

        const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
        const link = document.createElement('a');
        const url = URL.createObjectURL(blob);
        link.setAttribute('href', url);
        link.setAttribute('download', 'schueler_vorlage.csv');
        link.style.visibility = 'hidden';
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
    };

    // Handle file upload and parsing
    const handleFileUpload = useCallback((file: File) => {
        setUploadedFile(file);

        const reader = new FileReader();
        reader.onload = (e) => {
            const text = e.target?.result as string;
            const lines = text.split('\n').filter(line => line.trim());

            // Skip header row
            const dataRows = lines.slice(1);

            // Parse CSV and create student objects
            const parsedData: CSVStudent[] = dataRows.map((line, index) => {
                const values = line.split(',').map(v => v.trim());
                const errors: string[] = [];

                // Validate required fields
                if (!values[0]) errors.push("Vorname fehlt");
                if (!values[1]) errors.push("Nachname fehlt");

                // Validate guardian (at least name or contact required)
                if (!values[5] && !values[7] && !values[8] && !values[9]) {
                    errors.push("Mindestens ein Erziehungsberechtigter mit Kontaktinformationen erforderlich");
                }

                // Validate data retention days (must be 1-31)
                const retentionDays = parseInt(values[14] ?? "30", 10);
                if (retentionDays < 1 || retentionDays > 31) {
                    errors.push("Aufbewahrungsdauer muss zwischen 1 und 31 Tagen liegen");
                }

                // Simulate checking if student exists (in real implementation, check against API)
                const isExisting = Math.random() > 0.7; // 30% chance of existing
                const hasError = errors.length > 0;

                return {
                    row: index + 2, // +2 because of header and 0-index
                    status: hasError ? "error" : (isExisting ? "existing" : "new"),
                    errors,
                    first_name: values[0] ?? "",
                    second_name: values[1] ?? "",
                    school_class: values[2] ?? "",
                    group_name: values[3] ?? "",
                    birthday: values[4] ?? "",
                    guardian_name: values[5] ?? "",
                    guardian_relationship: values[6] ?? "Erziehungsberechtigter",
                    guardian_email: values[7] ?? "",
                    guardian_phone: values[8] ?? "",
                    guardian_contact: values[9] ?? "",
                    health_info: values[10] ?? "",
                    supervisor_notes: values[11] ?? "",
                    additional_info: values[12] ?? "",
                    privacy_consent: values[13]?.toLowerCase() === "ja",
                    data_retention_days: retentionDays,
                    bus: values[15]?.toLowerCase() === "ja",
                };
            });

            setCsvData(parsedData);
        };

        reader.readAsText(file);
    }, []);

    // Drag and drop handlers
    const handleDragEnter = (e: React.DragEvent) => {
        e.preventDefault();
        e.stopPropagation();
        setIsDragging(true);
    };

    const handleDragLeave = (e: React.DragEvent) => {
        e.preventDefault();
        e.stopPropagation();
        setIsDragging(false);
    };

    const handleDragOver = (e: React.DragEvent) => {
        e.preventDefault();
        e.stopPropagation();
    };

    const handleDrop = (e: React.DragEvent) => {
        e.preventDefault();
        e.stopPropagation();
        setIsDragging(false);

        const files = e.dataTransfer.files;
        if (files.length > 0) {
            const file = files[0];
            if (file && (file.type === 'text/csv' || file.name.endsWith('.csv'))) {
                handleFileUpload(file);
            }
        }
    };

    // Convert CSVStudent to Student format for modal
    const convertCSVStudentToStudent = (csvStudent: CSVStudent, index: number): Student => {
        return {
            id: `csv-temp-${index}`, // Temporary ID for CSV rows
            name: `${csvStudent.first_name} ${csvStudent.second_name}`.trim(),
            first_name: csvStudent.first_name,
            second_name: csvStudent.second_name,
            school_class: csvStudent.school_class,
            group_name: csvStudent.group_name,
            group_id: undefined,
            current_location: "Zuhause",
            in_house: false,
            bus: csvStudent.bus,
            name_lg: csvStudent.guardian_name,
            contact_lg: csvStudent.guardian_contact,
            guardian_email: csvStudent.guardian_email,
            guardian_phone: csvStudent.guardian_phone,
            birthday: csvStudent.birthday,
            health_info: csvStudent.health_info,
            supervisor_notes: csvStudent.supervisor_notes,
            extra_info: csvStudent.additional_info,
            privacy_consent_accepted: csvStudent.privacy_consent,
            data_retention_days: csvStudent.data_retention_days,
        };
    };

    // Handle opening edit modal
    const handleEditRow = (index: number) => {
        const csvStudent = csvData[index];
        if (!csvStudent) return;

        const student = convertCSVStudentToStudent(csvStudent, index);
        setEditingStudent(student);
        setEditingRowIndex(index);
        setShowEditModal(true);
    };

    // Handle saving from modal
    const handleSaveFromModal = async (updatedStudent: Partial<Student>) => {
        if (editingRowIndex === null) return;

        // Convert Student back to CSVStudent format
        const updatedCSVStudent: CSVStudent = {
            ...csvData[editingRowIndex]!,
            first_name: updatedStudent.first_name ?? "",
            second_name: updatedStudent.second_name ?? "",
            school_class: updatedStudent.school_class ?? "",
            group_name: updatedStudent.group_name ?? "",
            birthday: updatedStudent.birthday ?? "",
            guardian_name: updatedStudent.name_lg ?? "",
            guardian_relationship: "Erziehungsberechtigter",
            guardian_email: updatedStudent.guardian_email ?? "",
            guardian_phone: updatedStudent.guardian_phone ?? "",
            guardian_contact: updatedStudent.contact_lg ?? "",
            health_info: updatedStudent.health_info ?? "",
            supervisor_notes: updatedStudent.supervisor_notes ?? "",
            additional_info: updatedStudent.extra_info ?? "",
            privacy_consent: updatedStudent.privacy_consent_accepted ?? false,
            data_retention_days: updatedStudent.data_retention_days ?? 30,
            bus: updatedStudent.bus ?? false,
        };

        // Re-validate
        const errors: string[] = [];
        if (!updatedCSVStudent.first_name) errors.push("Vorname fehlt");
        if (!updatedCSVStudent.second_name) errors.push("Nachname fehlt");

        // Validate guardian
        if (!updatedCSVStudent.guardian_name && !updatedCSVStudent.guardian_email &&
            !updatedCSVStudent.guardian_phone && !updatedCSVStudent.guardian_contact) {
            errors.push("Mindestens ein Erziehungsberechtigter mit Kontaktinformationen erforderlich");
        }

        // Validate data retention days
        if (updatedCSVStudent.data_retention_days < 1 || updatedCSVStudent.data_retention_days > 31) {
            errors.push("Aufbewahrungsdauer muss zwischen 1 und 31 Tagen liegen");
        }

        updatedCSVStudent.errors = errors;
        updatedCSVStudent.status = errors.length > 0 ? "error" : csvData[editingRowIndex]!.status;

        // Update csvData
        setCsvData(prev => prev.map((row, idx) => idx === editingRowIndex ? updatedCSVStudent : row));

        // Close modal
        setShowEditModal(false);
        setEditingStudent(null);
        setEditingRowIndex(null);
    };

    // Get status color
    const getStatusBadge = (status: RowStatus) => {
        switch (status) {
            case "new":
                return <span className="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-[#83CD2D]/10 text-[#83CD2D]" style={{ backgroundColor: 'rgba(131, 205, 45, 0.1)', color: '#83CD2D' }}>Neu</span>;
            case "existing":
                return <span className="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-[#5080D8]/10 text-[#5080D8]" style={{ backgroundColor: 'rgba(80, 128, 216, 0.1)', color: '#5080D8' }}>Vorhanden</span>;
            case "error":
                return <span className="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-[#FF3130]/10 text-[#FF3130]" style={{ backgroundColor: 'rgba(255, 49, 48, 0.1)', color: '#FF3130' }}>Fehler</span>;
            case "updated":
                return <span className="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-amber-100 text-amber-700">Aktualisiert</span>;
        }
    };

    // Stats
    const stats = {
        total: csvData.length,
        new: csvData.filter(r => r.status === "new").length,
        existing: csvData.filter(r => r.status === "existing").length,
        errors: csvData.filter(r => r.status === "error").length,
    };

    if (status === "loading") {
        return (
            <ResponsiveLayout>
                <Loading fullPage={false} />
            </ResponsiveLayout>
        );
    }

    return (
        <ResponsiveLayout>
            <div className="w-full space-y-6">
                {/* Info Section */}
                <div className="rounded-xl border border-blue-100 bg-blue-50/30 p-6">
                    <div className="flex items-start gap-4">
                        <div className="flex-shrink-0">
                            <svg className="h-6 w-6 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                            </svg>
                        </div>
                        <div className="flex-1">
                            <h3 className="text-sm font-semibold text-gray-900 mb-2">CSV-Import Anleitung</h3>
                            <ul className="text-sm text-gray-600 space-y-1 list-disc list-inside">
                                <li>Laden Sie die <button onClick={handleDownloadTemplate} className="text-blue-600 hover:text-blue-800 font-medium underline">Muster-CSV</button> herunter</li>
                                <li>Füllen Sie die Datei mit Ihren Schülerdaten aus (Excel oder Texteditor)</li>
                                <li>Speichern Sie die Datei als CSV (Komma-getrennt)</li>
                                <li>Laden Sie die Datei hier hoch und überprüfen Sie die Vorschau</li>
                                <li>Korrigieren Sie eventuelle Fehler direkt in der Vorschau</li>
                                <li>Bestätigen Sie den Import</li>
                            </ul>
                        </div>
                    </div>
                </div>

                {/* Download Template Button */}
                <div className="rounded-xl border border-gray-100 bg-white p-6">
                    <h3 className="text-sm font-semibold text-gray-900 mb-4 flex items-center gap-2">
                        <svg className="h-5 w-5 text-purple-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                        </svg>
                        Schritt 1: Vorlage herunterladen
                    </h3>
                    <button
                        onClick={handleDownloadTemplate}
                        className="flex items-center gap-3 px-6 py-3 bg-gradient-to-br from-purple-500 to-purple-600 text-white rounded-xl shadow-lg hover:shadow-xl transition-all duration-300 hover:scale-105 active:scale-95"
                    >
                        <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                        </svg>
                        <span className="font-semibold">Muster-CSV herunterladen</span>
                    </button>
                </div>

                {/* Upload Section */}
                <div className="rounded-xl border border-gray-100 bg-white p-6">
                    <h3 className="text-sm font-semibold text-gray-900 mb-4 flex items-center gap-2">
                        <svg className="h-5 w-5 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
                        </svg>
                        Schritt 2: CSV-Datei hochladen
                    </h3>

                    {/* Drag & Drop Area */}
                    <div
                        onDragEnter={handleDragEnter}
                        onDragLeave={handleDragLeave}
                        onDragOver={handleDragOver}
                        onDrop={handleDrop}
                        className={`relative border-2 border-dashed rounded-xl p-12 text-center transition-all duration-300 ${
                            isDragging
                                ? 'border-green-500 bg-green-50'
                                : 'border-gray-300 hover:border-gray-400 bg-gray-50'
                        }`}
                    >
                        <div className="flex flex-col items-center gap-4">
                            <svg className={`h-16 w-16 transition-colors ${isDragging ? 'text-green-500' : 'text-gray-400'}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
                            </svg>
                            <div>
                                <p className="text-lg font-medium text-gray-900 mb-1">
                                    {isDragging ? 'Datei hier ablegen...' : 'Datei hierher ziehen'}
                                </p>
                                <p className="text-sm text-gray-500">oder</p>
                            </div>
                            <label className="cursor-pointer">
                                <span className="inline-flex items-center gap-2 px-6 py-3 bg-gray-900 text-white rounded-lg hover:bg-gray-700 transition-colors">
                                    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.172 7l-6.586 6.586a2 2 0 102.828 2.828l6.414-6.586a4 4 0 00-5.656-5.656l-6.415 6.585a6 6 0 108.486 8.486L20.5 13" />
                                    </svg>
                                    Datei auswählen
                                </span>
                                <input
                                    type="file"
                                    accept=".csv"
                                    onChange={(e) => {
                                        const file = e.target.files?.[0];
                                        if (file) handleFileUpload(file);
                                    }}
                                    className="hidden"
                                />
                            </label>
                            {uploadedFile && (
                                <div className="flex items-center gap-2 text-sm text-gray-600 bg-white px-4 py-2 rounded-lg border border-gray-200">
                                    <svg className="h-4 w-4 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                                    </svg>
                                    {uploadedFile.name}
                                </div>
                            )}
                        </div>
                    </div>
                </div>

                {/* Preview Section */}
                {csvData.length > 0 && (
                    <>
                        {/* Statistics */}
                        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                            <div className="rounded-xl border border-gray-100 bg-white p-4">
                                <div className="flex items-center gap-3">
                                    <div className="flex-shrink-0 w-10 h-10 rounded-full bg-gray-100 flex items-center justify-center">
                                        <svg className="h-5 w-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                                        </svg>
                                    </div>
                                    <div>
                                        <p className="text-2xl font-bold text-gray-900">{stats.total}</p>
                                        <p className="text-xs text-gray-600">Gesamt</p>
                                    </div>
                                </div>
                            </div>
                            <div className="rounded-xl border bg-white p-4" style={{ borderColor: 'rgba(131, 205, 45, 0.2)', backgroundColor: 'rgba(131, 205, 45, 0.05)' }}>
                                <div className="flex items-center gap-3">
                                    <div className="flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center" style={{ backgroundColor: 'rgba(131, 205, 45, 0.15)' }}>
                                        <svg className="h-5 w-5" style={{ color: '#83CD2D' }} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
                                        </svg>
                                    </div>
                                    <div>
                                        <p className="text-2xl font-bold" style={{ color: '#70b525' }}>{stats.new}</p>
                                        <p className="text-xs" style={{ color: '#83CD2D' }}>Neu</p>
                                    </div>
                                </div>
                            </div>
                            <div className="rounded-xl border bg-white p-4" style={{ borderColor: 'rgba(80, 128, 216, 0.2)', backgroundColor: 'rgba(80, 128, 216, 0.05)' }}>
                                <div className="flex items-center gap-3">
                                    <div className="flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center" style={{ backgroundColor: 'rgba(80, 128, 216, 0.15)' }}>
                                        <svg className="h-5 w-5" style={{ color: '#5080D8' }} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                                        </svg>
                                    </div>
                                    <div>
                                        <p className="text-2xl font-bold" style={{ color: '#3d67b0' }}>{stats.existing}</p>
                                        <p className="text-xs" style={{ color: '#5080D8' }}>Vorhanden</p>
                                    </div>
                                </div>
                            </div>
                            <div className="rounded-xl border bg-white p-4" style={{ borderColor: 'rgba(255, 49, 48, 0.2)', backgroundColor: 'rgba(255, 49, 48, 0.05)' }}>
                                <div className="flex items-center gap-3">
                                    <div className="flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center" style={{ backgroundColor: 'rgba(255, 49, 48, 0.15)' }}>
                                        <svg className="h-5 w-5" style={{ color: '#FF3130' }} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                                        </svg>
                                    </div>
                                    <div>
                                        <p className="text-2xl font-bold" style={{ color: '#e02020' }}>{stats.errors}</p>
                                        <p className="text-xs" style={{ color: '#FF3130' }}>Fehler</p>
                                    </div>
                                </div>
                            </div>
                        </div>

                        {/* Data List */}
                        <div className="rounded-xl border border-gray-100 bg-white overflow-hidden">
                            <div className="p-4 border-b border-gray-100">
                                <h3 className="text-sm font-semibold text-gray-900 flex items-center gap-2">
                                    <svg className="h-5 w-5 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
                                    </svg>
                                    Schritt 3: Datenvorschau & Korrektur
                                </h3>
                                <p className="text-xs text-gray-600 mt-1">Überprüfen Sie die Daten und korrigieren Sie Fehler durch Klick auf einen Eintrag</p>
                            </div>

                            <div className="p-3 space-y-2">
                                {csvData.map((student, index) => {
                                    const initials = `${student.first_name?.[0] ?? ''}${student.second_name?.[0] ?? ''}`;
                                    return (
                                        <div
                                            key={index}
                                            onClick={() => handleEditRow(index)}
                                            className="group cursor-pointer relative overflow-hidden rounded-2xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-sm transition-all duration-300 hover:scale-[1.005] hover:shadow-md hover:bg-white hover:-translate-y-0.5 active:scale-[0.995] hover:border-blue-200/50"
                                        >
                                            {/* Status-based overlay */}
                                            <div className={`absolute inset-0 rounded-2xl ${
                                                student.status === 'new' ? 'bg-gradient-to-br from-[#83CD2D]/5 to-[#83CD2D]/10 opacity-40' :
                                                student.status === 'existing' ? 'bg-gradient-to-br from-[#5080D8]/5 to-[#5080D8]/10 opacity-40' :
                                                student.status === 'error' ? 'bg-gradient-to-br from-[#FF3130]/5 to-[#FF3130]/10 opacity-60' :
                                                'bg-gradient-to-br from-gray-50/80 to-gray-100/80 opacity-40'
                                            }`}></div>

                                            {/* Inner glow */}
                                            <div className="absolute inset-px rounded-2xl bg-gradient-to-br from-white/80 to-white/20"></div>

                                            {/* Border highlight */}
                                            <div className={`absolute inset-0 rounded-2xl ring-1 ${
                                                student.status === 'error' ? 'ring-[#FF3130]/20' : 'ring-white/20'
                                            } group-hover:ring-blue-200/60 transition-all duration-300`}></div>

                                            <div className="relative flex items-center gap-3 p-3">
                                                {/* Avatar with status indicator */}
                                                <div className="flex-shrink-0 relative">
                                                    <div className={`h-9 w-9 rounded-full flex items-center justify-center text-white text-xs font-semibold shadow-sm group-hover:scale-105 transition-transform duration-300 ${
                                                        student.status === 'new' ? 'bg-gradient-to-br from-[#83CD2D] to-[#70b525]' :
                                                        student.status === 'existing' ? 'bg-gradient-to-br from-[#5080D8] to-[#4070c8]' :
                                                        student.status === 'error' ? 'bg-gradient-to-br from-[#FF3130] to-[#e02020]' :
                                                        'bg-gradient-to-br from-gray-400 to-gray-500'
                                                    }`}>
                                                        {initials}
                                                    </div>
                                                    {/* Row number badge */}
                                                    <div className="absolute -bottom-1 -right-1 h-4 w-4 rounded-full bg-gray-700 flex items-center justify-center text-[8px] font-bold text-white shadow-sm">
                                                        {student.row}
                                                    </div>
                                                </div>

                                                {/* Student Info */}
                                                <div className="flex-1 min-w-0">
                                                    <div className="flex items-center gap-2">
                                                        <h4 className="text-sm font-semibold text-gray-900 group-hover:text-blue-600 transition-colors duration-300">
                                                            {student.first_name} {student.second_name}
                                                        </h4>
                                                        {getStatusBadge(student.status)}
                                                    </div>
                                                    <div className="flex items-center gap-2 mt-0.5 flex-wrap">
                                                        <span className="text-xs text-gray-600">
                                                            {student.school_class}
                                                        </span>
                                                        {student.group_name && (
                                                            <>
                                                                <span className="text-gray-300">•</span>
                                                                <span className="text-xs text-gray-500">{student.group_name}</span>
                                                            </>
                                                        )}
                                                        {student.guardian_name && (
                                                            <>
                                                                <span className="text-gray-300">•</span>
                                                                <span className="text-xs text-gray-400">{student.guardian_name}</span>
                                                            </>
                                                        )}
                                                    </div>
                                                    {/* Error messages */}
                                                    {student.errors.length > 0 && (
                                                        <p className="text-xs text-[#FF3130] mt-1 flex items-center gap-1">
                                                            <svg className="h-3 w-3 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                                                            </svg>
                                                            {student.errors[0]}
                                                            {student.errors.length > 1 && ` (+${student.errors.length - 1})`}
                                                        </p>
                                                    )}
                                                </div>

                                                {/* Edit Icon */}
                                                <div className="flex-shrink-0">
                                                    <svg className="h-5 w-5 text-gray-400 group-hover:text-blue-600 group-hover:translate-x-0.5 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                                    </svg>
                                                </div>
                                            </div>

                                            {/* Hover effect overlay */}
                                            <div className="absolute inset-0 rounded-2xl opacity-0 group-hover:opacity-100 transition-opacity duration-300 bg-gradient-to-r from-transparent via-blue-100/20 to-transparent"></div>
                                        </div>
                                    );
                                })}
                            </div>
                        </div>

                        {/* Error Summary */}
                        {stats.errors > 0 && (
                            <div className="rounded-xl border border-red-200 bg-red-50 p-6">
                                <div className="flex items-start gap-4">
                                    <svg className="h-6 w-6 text-red-600 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                                    </svg>
                                    <div className="flex-1">
                                        <h4 className="text-sm font-semibold text-red-900 mb-2">
                                            {stats.errors} Zeile(n) mit Fehlern gefunden
                                        </h4>
                                        <ul className="text-sm text-red-800 space-y-1">
                                            {csvData.filter(s => s.status === 'error').map((student, idx) => (
                                                <li key={idx}>
                                                    Zeile {student.row}: {student.errors.join(', ')}
                                                </li>
                                            ))}
                                        </ul>
                                        <p className="text-sm text-red-700 mt-3">
                                            Bitte korrigieren Sie die Fehler, bevor Sie fortfahren.
                                        </p>
                                    </div>
                                </div>
                            </div>
                        )}

                        {/* Action Buttons */}
                        <div className="flex flex-col sm:flex-row gap-4 sticky bottom-6 bg-white/95 backdrop-blur-sm rounded-xl border border-gray-200 p-4 shadow-lg">
                            <Link
                                href="/database/students"
                                className="flex-1 px-6 py-3 rounded-lg border border-gray-300 text-sm font-medium text-gray-700 hover:bg-gray-50 hover:border-gray-400 text-center transition-colors"
                            >
                                Abbrechen
                            </Link>
                            <button
                                disabled={stats.errors > 0}
                                className="flex-1 px-6 py-3 rounded-lg text-sm font-medium text-white disabled:cursor-not-allowed shadow-lg hover:shadow-xl transition-all duration-300 hover:scale-105 active:scale-95 disabled:hover:scale-100"
                                style={{
                                    background: stats.errors > 0
                                        ? 'linear-gradient(to bottom right, #d1d5db, #9ca3af)'
                                        : 'linear-gradient(to bottom right, #83CD2D, #70b525)'
                                }}
                                onMouseEnter={(e) => {
                                    if (stats.errors === 0) {
                                        e.currentTarget.style.background = 'linear-gradient(to bottom right, #70b525, #5d9920)';
                                    }
                                }}
                                onMouseLeave={(e) => {
                                    if (stats.errors === 0) {
                                        e.currentTarget.style.background = 'linear-gradient(to bottom right, #83CD2D, #70b525)';
                                    }
                                }}
                            >
                                {stats.new} Schüler importieren
                            </button>
                        </div>
                    </>
                )}
            </div>

            {/* Edit Modal */}
            {editingStudent && (
                <StudentEditModal
                    isOpen={showEditModal}
                    onClose={() => {
                        setShowEditModal(false);
                        setEditingStudent(null);
                        setEditingRowIndex(null);
                    }}
                    student={editingStudent}
                    onSave={handleSaveFromModal}
                    loading={false}
                />
            )}
        </ResponsiveLayout>
    );
}
