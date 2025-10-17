"use client";

import { useState, useEffect, useMemo, useCallback, useRef } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import Link from "next/link";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header/types";
import { SimpleAlert } from "@/components/simple/SimpleAlert";
import { StudentDetailModal } from "@/components/students/student-detail-modal";
import { StudentEditModal } from "@/components/students/student-edit-modal";
import { StudentCreateModal } from "@/components/students/student-create-modal";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { studentsConfig } from "@/lib/database/configs/students.config";
import type { Student } from "@/lib/api";

export default function StudentsPage() {
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [students, setStudents] = useState<Student[]>([]);
    const [searchTerm, setSearchTerm] = useState("");
    const [groupFilter, setGroupFilter] = useState("all");
    const [isMobile, setIsMobile] = useState(false);
    const [isFabVisible, setIsFabVisible] = useState(true);
    const [lastScrollY, setLastScrollY] = useState(0);

    // Modal states
    const [showCreateModal, setShowCreateModal] = useState(false);
    const [createLoading, setCreateLoading] = useState(false);

    const [showDetailModal, setShowDetailModal] = useState(false);
    const [showEditModal, setShowEditModal] = useState(false);
    const [selectedStudent, setSelectedStudent] = useState<Student | null>(null);
    const [detailLoading, setDetailLoading] = useState(false);

    const [showSuccessAlert, setShowSuccessAlert] = useState(false);
    const [successMessage, setSuccessMessage] = useState("");

    // Track mounted state to prevent race conditions
    const isMountedRef = useRef(true);

    const { status } = useSession({
        required: true,
        onUnauthenticated() {
            redirect("/");
        },
    });

    // Create service instance
    const service = useMemo(() => createCrudService(studentsConfig), []);

    // Cleanup on unmount
    useEffect(() => {
        return () => {
            isMountedRef.current = false;
        };
    }, []);

    // Handle mobile detection
    useEffect(() => {
        const checkMobile = () => {
            setIsMobile(window.innerWidth < 768);
        };
        checkMobile();
        window.addEventListener('resize', checkMobile);
        return () => window.removeEventListener('resize', checkMobile);
    }, []);

    // Handle FAB visibility based on scroll (same logic as mobile bottom nav)
    useEffect(() => {
        const handleScroll = () => {
            const currentScrollY = window.scrollY;

            if (currentScrollY > lastScrollY && currentScrollY > 100) {
                setIsFabVisible(false); // Hide when scrolling down
            } else {
                setIsFabVisible(true); // Show when scrolling up
            }

            setLastScrollY(currentScrollY);
        };

        window.addEventListener("scroll", handleScroll, { passive: true });
        return () => window.removeEventListener("scroll", handleScroll);
    }, [lastScrollY]);

    // Fetch students
    const fetchStudents = useCallback(async () => {
        try {
            setLoading(true);
            const data = await service.getList({ page: 1, pageSize: 1000 });
            const studentsArray = Array.isArray(data.data) ? data.data : [];
            setStudents(studentsArray);
            setError(null);
        } catch (err) {
            console.error("Error fetching students:", err);
            setError("Fehler beim Laden der Schüler. Bitte versuchen Sie es später erneut.");
            setStudents([]);
        } finally {
            setLoading(false);
        }
    }, [service]);

    // Load students on mount
    useEffect(() => {
        void fetchStudents();
    }, [fetchStudents]);

    // Apply filters
    const filteredStudents = useMemo(() => {
        let filtered = [...students];

        // Search filter
        if (searchTerm) {
            const searchLower = searchTerm.toLowerCase();
            filtered = filtered.filter(student =>
                (student.first_name?.toLowerCase().includes(searchLower) ?? false) ||
                (student.second_name?.toLowerCase().includes(searchLower) ?? false) ||
                (student.school_class?.toLowerCase().includes(searchLower) ?? false) ||
                (student.group_name?.toLowerCase().includes(searchLower) ?? false) ||
                (student.name_lg?.toLowerCase().includes(searchLower) ?? false)
            );
        }

        // Group filter
        if (groupFilter !== "all") {
            filtered = filtered.filter(student => student.group_id === groupFilter);
        }

        // Sort alphabetically by name
        filtered.sort((a, b) => {
            const nameA = `${a.first_name} ${a.second_name}`;
            const nameB = `${b.first_name} ${b.second_name}`;
            return nameA.localeCompare(nameB, 'de');
        });

        return filtered;
    }, [students, searchTerm, groupFilter]);

    // Get unique values for filters
    const uniqueGroups = useMemo(() => {
        const groupMap = new Map<string, string>();
        students.forEach(student => {
            if (student.group_id && student.group_name) {
                groupMap.set(student.group_id, student.group_name);
            }
        });
        return Array.from(groupMap.entries())
            .sort((a, b) => a[1].localeCompare(b[1]))
            .map(([value, label]) => ({ value, label }));
    }, [students]);

    // Prepare filters for PageHeaderWithSearch
    const filters: FilterConfig[] = useMemo(() => [
        {
            id: 'group',
            label: 'Gruppe',
            type: 'dropdown',
            value: groupFilter,
            onChange: (value) => setGroupFilter(value as string),
            options: [
                { value: "all", label: "Alle Gruppen" },
                ...uniqueGroups
            ]
        }
    ], [groupFilter, uniqueGroups]);

    // Prepare active filters
    const activeFilters: ActiveFilter[] = useMemo(() => {
        const filters: ActiveFilter[] = [];

        if (searchTerm) {
            filters.push({
                id: 'search',
                label: `"${searchTerm}"`,
                onRemove: () => setSearchTerm("")
            });
        }

        if (groupFilter !== "all") {
            const group = uniqueGroups.find(g => g.value === groupFilter);
            filters.push({
                id: 'group',
                label: group?.label ?? 'Gruppe',
                onRemove: () => setGroupFilter("all")
            });
        }

        return filters;
    }, [searchTerm, groupFilter, uniqueGroups]);

    // Handle student selection
    const handleSelectStudent = async (student: Student) => {
        setSelectedStudent(student);
        setShowDetailModal(true);

        try {
            setDetailLoading(true);
            const freshData = await service.getOne(student.id);

            // Only update state if still mounted
            if (!isMountedRef.current) return;

            setSelectedStudent(freshData);
        } catch (err) {
            console.error("Error fetching student details:", err);
        } finally {
            if (isMountedRef.current) {
                setDetailLoading(false);
            }
        }
    };

    // Handle create student
    const handleCreateStudent = async (studentData: Partial<Student>) => {
        setCreateLoading(true);

        try {
            if (studentsConfig.form.transformBeforeSubmit) {
                studentData = studentsConfig.form.transformBeforeSubmit(studentData);
            }

            const newStudent = await service.create(studentData);

            // Only update state if still mounted
            if (!isMountedRef.current) return;

            const displayName = studentsConfig.list.item.title(newStudent);
            setSuccessMessage(getDbOperationMessage('create', studentsConfig.name.singular, displayName));
            setShowSuccessAlert(true);

            setShowCreateModal(false);
            await fetchStudents();
        } finally {
            if (isMountedRef.current) {
                setCreateLoading(false);
            }
        }
    };

    // Handle update student
    const handleUpdateStudent = async (studentData: Partial<Student>) => {
        if (!selectedStudent) return;

        try {
            setDetailLoading(true);

            if (studentsConfig.form.transformBeforeSubmit) {
                studentData = studentsConfig.form.transformBeforeSubmit(studentData);
            }

            await service.update(selectedStudent.id, studentData);

            // Only update state if still mounted
            if (!isMountedRef.current) return;

            const displayName = studentsConfig.list.item.title(selectedStudent);
            setSuccessMessage(getDbOperationMessage('update', studentsConfig.name.singular, displayName));
            setShowSuccessAlert(true);

            // Refresh student data
            const refreshedStudent = await service.getOne(selectedStudent.id);

            // Check again before updating state after second async operation
            if (!isMountedRef.current) return;

            setSelectedStudent(refreshedStudent);

            // Close edit modal and show updated detail modal
            setShowEditModal(false);
            setShowDetailModal(true);

            await fetchStudents();
        } catch (err) {
            console.error("Error updating student:", err);
            throw err;
        } finally {
            if (isMountedRef.current) {
                setDetailLoading(false);
            }
        }
    };

    // Handle delete student
    const handleDeleteStudent = async () => {
        if (!selectedStudent) return;

        try {
            setDetailLoading(true);
            await service.delete(selectedStudent.id);

            // Only update state if still mounted
            if (!isMountedRef.current) return;

            const displayName = studentsConfig.list.item.title(selectedStudent);
            setSuccessMessage(getDbOperationMessage('delete', studentsConfig.name.singular, displayName));
            setShowSuccessAlert(true);

            setShowDetailModal(false);
            setSelectedStudent(null);
            await fetchStudents();
        } catch (err) {
            console.error("Error deleting student:", err);
        } finally {
            if (isMountedRef.current) {
                setDetailLoading(false);
            }
        }
    };

    // Handle edit button click
    const handleEditClick = () => {
        setShowDetailModal(false);
        setShowEditModal(true);
    };

    if (status === "loading" || loading) {
        return (
            <ResponsiveLayout>
                <div className="flex min-h-[50vh] items-center justify-center">
                    <div className="flex flex-col items-center gap-4">
                        <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-[#5080D8]"></div>
                        <p className="text-gray-600">Schüler werden geladen...</p>
                    </div>
                </div>
            </ResponsiveLayout>
        );
    }

    return (
        <ResponsiveLayout>
            <div className="w-full -mt-1.5">
                {/* Mobile Back Button */}
                {isMobile && (
                    <button
                        onClick={() => window.location.href = '/database'}
                        className="flex items-center gap-2 text-gray-600 hover:text-gray-900 mb-3 transition-colors duration-200 relative z-10"
                        aria-label="Zurück zur Datenverwaltung"
                    >
                        <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
                        </svg>
                        <span className="text-sm font-medium">Zurück</span>
                    </button>
                )}

                {/* PageHeaderWithSearch - Title only on mobile */}
                <div className="mb-4">
                    <PageHeaderWithSearch
                        title={isMobile ? "Schüler" : ""}
                        badge={{
                            icon: (
                                <svg className="h-5 w-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                                          d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                                </svg>
                            ),
                            count: filteredStudents.length,
                            label: "Schüler"
                        }}
                        search={{
                            value: searchTerm,
                            onChange: setSearchTerm,
                            placeholder: "Schüler suchen..."
                        }}
                        filters={filters}
                        activeFilters={activeFilters}
                        onClearAllFilters={() => {
                            setSearchTerm("");
                            setGroupFilter("all");
                        }}
                        actionButton={!isMobile && (
                            <div className="flex items-center gap-3">
                                <Link
                                    href="/database/students/csv-import"
                                    className="relative flex items-center gap-2 px-4 h-10 bg-gradient-to-br from-purple-500 to-purple-600 text-white rounded-full shadow-lg hover:shadow-xl transition-all duration-300 group hover:scale-105 active:scale-95"
                                    aria-label="CSV Import"
                                >
                                    <div className="absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"></div>
                                    <svg className="relative h-5 w-5 transition-transform duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                                        <path strokeLinecap="round" strokeLinejoin="round" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
                                    </svg>
                                    <span className="relative text-sm font-semibold">CSV Import</span>
                                    <div className="absolute inset-0 rounded-full bg-white/20 scale-0 group-hover:scale-100 transition-transform duration-500 opacity-0 group-hover:opacity-100 pointer-events-none"></div>
                                </Link>
                                <button
                                    onClick={() => setShowCreateModal(true)}
                                    className="relative w-10 h-10 bg-gradient-to-br from-[#5080D8] to-[#4070c8] text-white rounded-full shadow-lg hover:shadow-xl transition-all duration-300 flex items-center justify-center group hover:scale-110 active:scale-95"
                                    aria-label="Schüler erstellen"
                                >
                                    <div className="absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"></div>
                                    <svg className="relative h-5 w-5 transition-transform duration-300 group-active:rotate-90" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                                        <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
                                    </svg>
                                    <div className="absolute inset-0 rounded-full bg-white/20 scale-0 group-hover:scale-100 transition-transform duration-500 opacity-0 group-hover:opacity-100 pointer-events-none"></div>
                                </button>
                            </div>
                        )}
                    />
                </div>

                {/* Mobile FAB Create Button */}
                <button
                    onClick={() => setShowCreateModal(true)}
                    className={`md:hidden fixed right-4 bottom-24 z-40 w-14 h-14 bg-gradient-to-br from-[#5080D8] to-[#4070c8] text-white rounded-full shadow-[0_8px_30px_rgb(0,0,0,0.12)] hover:shadow-[0_8px_40px_rgb(80,128,216,0.3)] flex items-center justify-center group active:scale-95 transition-all duration-300 ease-out ${
                        isFabVisible ? 'translate-y-0 opacity-100 pointer-events-auto' : 'translate-y-32 opacity-0 pointer-events-none'
                    }`}
                    aria-label="Schüler erstellen"
                >
                    <div className="absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"></div>
                    <svg className="relative h-6 w-6 transition-transform duration-300 group-active:rotate-90 pointer-events-none" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
                    </svg>
                    <div className="absolute inset-0 rounded-full bg-white/20 scale-0 group-hover:scale-100 transition-transform duration-500 opacity-0 group-hover:opacity-100 pointer-events-none"></div>
                </button>

                {/* Error Display */}
                {error && (
                    <div className="mb-6 rounded-lg bg-red-50 border border-red-200 p-4">
                        <p className="text-sm text-red-800">{error}</p>
                    </div>
                )}

                {/* Student List */}
                {filteredStudents.length === 0 ? (
                    <div className="flex min-h-[300px] items-center justify-center">
                        <div className="text-center">
                            <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                            </svg>
                            <h3 className="mt-4 text-lg font-medium text-gray-900">
                                {searchTerm || groupFilter !== "all"
                                    ? "Keine Schüler gefunden"
                                    : "Keine Schüler vorhanden"}
                            </h3>
                            <p className="mt-2 text-sm text-gray-600">
                                {searchTerm || groupFilter !== "all"
                                    ? "Versuchen Sie andere Suchkriterien oder Filter."
                                    : "Es wurden noch keine Schüler erstellt."}
                            </p>
                        </div>
                    </div>
                ) : (
                    <div className="space-y-3">
                        {filteredStudents.map((student, index) => {
                            const initials = `${student.first_name?.[0] ?? ''}${student.second_name?.[0] ?? ''}`;

                            return (
                                <div
                                    key={student.id}
                                    onClick={() => handleSelectStudent(student)}
                                    className="group cursor-pointer relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500 md:hover:scale-[1.01] md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)] md:hover:bg-white md:hover:-translate-y-1 active:scale-[0.99] md:hover:border-blue-200/50"
                                    style={{
                                        animationName: 'fadeInUp',
                                        animationDuration: '0.5s',
                                        animationTimingFunction: 'ease-out',
                                        animationFillMode: 'forwards',
                                        animationDelay: `${index * 0.03}s`,
                                        opacity: 0
                                    }}
                                >
                                    {/* Modern gradient overlay */}
                                    <div className="absolute inset-0 bg-gradient-to-br from-blue-50/80 to-cyan-100/80 opacity-[0.03] rounded-3xl"></div>
                                    {/* Subtle inner glow */}
                                    <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                                    {/* Modern border highlight */}
                                    <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 md:group-hover:ring-blue-200/60 transition-all duration-300"></div>

                                    <div className="relative flex items-center gap-4 p-5">
                                        {/* Avatar */}
                                        <div className="flex-shrink-0">
                                            <div className="h-12 w-12 rounded-full bg-gradient-to-br from-[#5080D8] to-[#4070c8] flex items-center justify-center text-white font-semibold shadow-md md:group-hover:scale-110 transition-transform duration-300">
                                                {initials}
                                            </div>
                                        </div>

                                        {/* Student Info */}
                                        <div className="flex-1 min-w-0">
                                            <h3 className="text-lg font-semibold text-gray-900 md:group-hover:text-blue-600 transition-colors duration-300">
                                                {student.first_name} {student.second_name}
                                            </h3>
                                            <div className="flex items-center gap-2 mt-1 flex-wrap">
                                                {/* Group Badge */}
                                                {student.group_name && (
                                                    <span className="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-[#83CD2D]/10 text-[#5A8B1F]">
                                                        {student.group_name}
                                                    </span>
                                                )}
                                            </div>
                                            {/* Guardian info */}
                                            {student.name_lg && (
                                                <p className="text-sm text-gray-500 mt-1">
                                                    <span className="text-gray-400">Erziehungsberechtigter:</span> {student.name_lg}
                                                </p>
                                            )}
                                        </div>

                                        {/* Arrow Icon */}
                                        <div className="flex-shrink-0">
                                            <svg className="h-6 w-6 text-gray-400 md:group-hover:text-blue-600 md:group-hover:translate-x-1 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                            </svg>
                                        </div>
                                    </div>

                                    {/* Glowing border effect on hover */}
                                    <div className="absolute inset-0 rounded-3xl opacity-0 md:group-hover:opacity-100 transition-opacity duration-300 bg-gradient-to-r from-transparent via-blue-100/30 to-transparent"></div>
                                </div>
                            );
                        })}

                        {/* Add fadeInUp animation */}
                        <style jsx>{`
                            @keyframes fadeInUp {
                                from {
                                    opacity: 0;
                                    transform: translateY(20px);
                                }
                                to {
                                    opacity: 1;
                                    transform: translateY(0);
                                }
                            }
                        `}</style>
                    </div>
                )}
            </div>

            {/* Create Modal */}
            <StudentCreateModal
                isOpen={showCreateModal}
                onClose={() => setShowCreateModal(false)}
                onCreate={handleCreateStudent}
                loading={createLoading}
                groups={uniqueGroups}
            />

            {/* Detail Modal */}
            <StudentDetailModal
                isOpen={showDetailModal}
                onClose={() => {
                    setShowDetailModal(false);
                    setSelectedStudent(null);
                }}
                student={selectedStudent}
                onEdit={handleEditClick}
                onDelete={() => void handleDeleteStudent()}
                loading={detailLoading}
            />

            {/* Edit Modal */}
            <StudentEditModal
                isOpen={showEditModal}
                onClose={() => {
                    setShowEditModal(false);
                }}
                student={selectedStudent}
                onSave={handleUpdateStudent}
                loading={detailLoading}
                groups={uniqueGroups}
            />

            {/* Success Alert */}
            {showSuccessAlert && (
                <SimpleAlert
                    type="success"
                    message={successMessage}
                    autoClose
                    duration={3000}
                    onClose={() => setShowSuccessAlert(false)}
                />
            )}
        </ResponsiveLayout>
    );
}
