"use client";

import { useState, useEffect, useMemo, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { ActiveFilter } from "~/components/ui/page-header/types";
import { SimpleAlert } from "@/components/simple/SimpleAlert";
import { TeacherRoleManagementModal, TeacherPermissionManagementModal } from "@/components/teachers";
import { TeacherDetailModal } from "@/components/teachers/teacher-detail-modal";
import { TeacherEditModal } from "@/components/teachers/teacher-edit-modal";
import { TeacherCreateModal } from "@/components/teachers/teacher-create-modal";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { teachersConfig } from "@/lib/database/configs/teachers.config";
import type { Teacher } from "@/lib/teacher-api";

export default function TeachersPage() {
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [teachers, setTeachers] = useState<Teacher[]>([]);
    const [searchTerm, setSearchTerm] = useState("");
    const [isMobile, setIsMobile] = useState(false);
    const [isFabVisible, setIsFabVisible] = useState(true);
    const [lastScrollY, setLastScrollY] = useState(0);

    // Modal states
    const [showCreateModal, setShowCreateModal] = useState(false);
    const [createLoading, setCreateLoading] = useState(false);

    const [showDetailModal, setShowDetailModal] = useState(false);
    const [showEditModal, setShowEditModal] = useState(false);
    const [selectedTeacher, setSelectedTeacher] = useState<Teacher | null>(null);
    const [detailLoading, setDetailLoading] = useState(false);

    // Role and permission modals
    const [roleModalOpen, setRoleModalOpen] = useState(false);
    const [permissionModalOpen, setPermissionModalOpen] = useState(false);

    const [showSuccessAlert, setShowSuccessAlert] = useState(false);
    const [successMessage, setSuccessMessage] = useState("");

    const { status } = useSession({
        required: true,
        onUnauthenticated() {
            redirect("/");
        },
    });

    // Create service instance
    const service = useMemo(() => createCrudService(teachersConfig), []);

    // Handle mobile detection
    useEffect(() => {
        const checkMobile = () => {
            setIsMobile(window.innerWidth < 768);
        };
        checkMobile();
        window.addEventListener('resize', checkMobile);
        return () => window.removeEventListener('resize', checkMobile);
    }, []);

    // Handle FAB visibility based on scroll
    useEffect(() => {
        const handleScroll = () => {
            const currentScrollY = window.scrollY;

            if (currentScrollY > lastScrollY && currentScrollY > 100) {
                setIsFabVisible(false);
            } else {
                setIsFabVisible(true);
            }

            setLastScrollY(currentScrollY);
        };

        window.addEventListener("scroll", handleScroll, { passive: true });
        return () => window.removeEventListener("scroll", handleScroll);
    }, [lastScrollY]);

    // Fetch teachers
    const fetchTeachers = useCallback(async () => {
        try {
            setLoading(true);
            const data = await service.getList({ page: 1, pageSize: 1000 });
            const teachersArray = Array.isArray(data.data) ? data.data : [];
            setTeachers(teachersArray);
            setError(null);
        } catch (err) {
            console.error("Error fetching teachers:", err);
            setError("Fehler beim Laden der Betreuer. Bitte versuchen Sie es später erneut.");
            setTeachers([]);
        } finally {
            setLoading(false);
        }
    }, [service]);

    // Load teachers on mount
    useEffect(() => {
        void fetchTeachers();
    }, [fetchTeachers]);

    // Apply filters
    const filteredTeachers = useMemo(() => {
        let filtered = [...teachers];

        // Search filter
        if (searchTerm) {
            const searchLower = searchTerm.toLowerCase();
            filtered = filtered.filter(teacher =>
                (teacher.first_name?.toLowerCase().includes(searchLower) ?? false) ||
                (teacher.last_name?.toLowerCase().includes(searchLower) ?? false) ||
                (teacher.name?.toLowerCase().includes(searchLower) ?? false) ||
                (teacher.specialization?.toLowerCase().includes(searchLower) ?? false) ||
                (teacher.email?.toLowerCase().includes(searchLower) ?? false)
            );
        }

        // Sort alphabetically by name
        filtered.sort((a, b) => {
            const nameA = a.name ?? `${a.first_name} ${a.last_name}`;
            const nameB = b.name ?? `${b.first_name} ${b.last_name}`;
            return nameA.localeCompare(nameB, 'de');
        });

        return filtered;
    }, [teachers, searchTerm]);

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

        return filters;
    }, [searchTerm]);

    // Handle teacher selection
    const handleSelectTeacher = async (teacher: Teacher) => {
        setSelectedTeacher(teacher);
        setShowDetailModal(true);

        try {
            setDetailLoading(true);
            // Use staff_id if available, otherwise fall back to id
            const idToFetch = teacher.staff_id ?? teacher.id;
            const freshData = await service.getOne(idToFetch);
            setSelectedTeacher(freshData);
        } catch (err) {
            console.error("Error fetching teacher details:", err);
        } finally {
            setDetailLoading(false);
        }
    };

    // Handle create teacher
    const handleCreateTeacher = async (data: Partial<Teacher> & { password?: string }) => {
        try {
            setCreateLoading(true);
            await service.create(data);
            setShowCreateModal(false);
            setSuccessMessage(getDbOperationMessage('create', teachersConfig.name.singular));
            setShowSuccessAlert(true);
            await fetchTeachers();
        } catch (err) {
            console.error("Error creating teacher:", err);
            throw err;
        } finally {
            setCreateLoading(false);
        }
    };

    // Handle edit teacher
    const handleEditTeacher = async (data: Partial<Teacher> & { password?: string }) => {
        if (!selectedTeacher) return;

        try {
            setDetailLoading(true);
            await service.update(selectedTeacher.id, data);
            setShowEditModal(false);
            setShowDetailModal(false);
            setSuccessMessage(getDbOperationMessage('update', teachersConfig.name.singular));
            setShowSuccessAlert(true);
            await fetchTeachers();
            setSelectedTeacher(null);
        } catch (err) {
            console.error("Error updating teacher:", err);
            throw err;
        } finally {
            setDetailLoading(false);
        }
    };

    // Handle delete teacher
    const handleDeleteTeacher = async () => {
        if (!selectedTeacher) return;

        try {
            setDetailLoading(true);
            await service.delete(selectedTeacher.id);
            setShowDetailModal(false);
            setSuccessMessage(getDbOperationMessage('delete', teachersConfig.name.singular));
            setShowSuccessAlert(true);
            await fetchTeachers();
            setSelectedTeacher(null);
        } catch (err) {
            console.error("Error deleting teacher:", err);
            throw err;
        } finally {
            setDetailLoading(false);
        }
    };

    // Handle edit click from detail modal
    const handleEditClick = () => {
        setShowDetailModal(false);
        setShowEditModal(true);
    };

    if (status === "loading" || loading) {
        return (
            <ResponsiveLayout>
                <div className="flex min-h-[50vh] items-center justify-center">
                    <div className="flex flex-col items-center gap-4">
                        <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-[#F78C10]"></div>
                        <p className="text-gray-600">Betreuer werden geladen...</p>
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
                <div className="relative z-20 mb-4">
                    <PageHeaderWithSearch
                        title={isMobile ? "Betreuer" : ""}
                        badge={{
                            icon: (
                                <svg className="h-5 w-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                                          d="M12 14l9-5-9-5-9 5 9 5z M12 14l6.16-3.422a12.083 12.083 0 01.665 6.479A11.952 11.952 0 0012 20.055a11.952 11.952 0 00-6.824-2.998 12.078 12.078 0 01.665-6.479L12 14z" />
                                </svg>
                            ),
                            count: filteredTeachers.length,
                            label: "Betreuer"
                        }}
                        search={{
                            value: searchTerm,
                            onChange: setSearchTerm,
                            placeholder: "Betreuer suchen..."
                        }}
                        filters={[]}
                        activeFilters={activeFilters}
                        onClearAllFilters={() => {
                            setSearchTerm("");
                        }}
                        actionButton={!isMobile && (
                            <button
                                onClick={() => setShowCreateModal(true)}
                                className="w-10 h-10 bg-gradient-to-br from-[#F78C10] to-[#e57a00] text-white rounded-full shadow-lg hover:shadow-xl transition-all duration-300 flex items-center justify-center group hover:scale-110 active:scale-95"
                                aria-label="Betreuer erstellen"
                            >
                                <div className="absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300"></div>
                                <svg className="relative h-5 w-5 transition-transform duration-300 group-active:rotate-90" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                                    <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
                                </svg>
                                <div className="absolute inset-0 rounded-full bg-white/20 scale-0 group-hover:scale-100 transition-transform duration-500 opacity-0 group-hover:opacity-100"></div>
                            </button>
                        )}
                    />
                </div>

                {/* Mobile FAB Create Button */}
                <button
                    onClick={() => setShowCreateModal(true)}
                    className={`md:hidden fixed right-4 bottom-24 z-40 w-14 h-14 bg-gradient-to-br from-[#F78C10] to-[#e57a00] text-white rounded-full shadow-[0_8px_30px_rgb(0,0,0,0.12)] hover:shadow-[0_8px_40px_rgb(247,140,16,0.3)] flex items-center justify-center group active:scale-95 transition-all duration-300 ease-out ${
                        isFabVisible ? 'translate-y-0 opacity-100 pointer-events-auto' : 'translate-y-32 opacity-0 pointer-events-none'
                    }`}
                    aria-label="Betreuer erstellen"
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

                {/* Teacher List */}
                {filteredTeachers.length === 0 ? (
                    <div className="flex min-h-[300px] items-center justify-center">
                        <div className="text-center">
                            <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M12 14l9-5-9-5-9 5 9 5z M12 14l6.16-3.422a12.083 12.083 0 01.665 6.479A11.952 11.952 0 0012 20.055a11.952 11.952 0 00-6.824-2.998 12.078 12.078 0 01.665-6.479L12 14z" />
                            </svg>
                            <h3 className="mt-4 text-lg font-medium text-gray-900">
                                {searchTerm
                                    ? "Keine Betreuer gefunden"
                                    : "Keine Betreuer vorhanden"}
                            </h3>
                            <p className="mt-2 text-sm text-gray-600">
                                {searchTerm
                                    ? "Versuchen Sie andere Suchkriterien."
                                    : "Es wurden noch keine Betreuer erstellt."}
                            </p>
                        </div>
                    </div>
                ) : (
                    <div className="space-y-3">
                        {filteredTeachers.map((teacher, index) => {
                            // Get initials from first_name and last_name, or from name if those aren't available
                            const initials = (teacher.first_name && teacher.last_name)
                                ? `${teacher.first_name[0]}${teacher.last_name[0]}`
                                : teacher.name
                                    ? teacher.name.split(' ').map(n => n[0]).join('').substring(0, 2)
                                    : 'XX';
                            const displayName = teacher.name || `${teacher.first_name} ${teacher.last_name}`;

                            return (
                                <div
                                    key={teacher.id}
                                    onClick={() => handleSelectTeacher(teacher)}
                                    className="group cursor-pointer relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500 md:hover:scale-[1.01] md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)] md:hover:bg-white md:hover:-translate-y-1 active:scale-[0.99] md:hover:border-orange-200/50"
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
                                    <div className="absolute inset-0 bg-gradient-to-br from-orange-50/80 to-amber-100/80 opacity-[0.03] rounded-3xl"></div>
                                    {/* Subtle inner glow */}
                                    <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                                    {/* Modern border highlight */}
                                    <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 md:group-hover:ring-orange-200/60 transition-all duration-300"></div>

                                    <div className="relative flex items-center gap-4 p-5">
                                        {/* Avatar */}
                                        <div className="flex-shrink-0">
                                            <div className="h-12 w-12 rounded-full bg-gradient-to-br from-[#F78C10] to-[#e57a00] flex items-center justify-center text-white text-sm font-semibold shadow-md md:group-hover:scale-110 transition-transform duration-300">
                                                {initials.toUpperCase()}
                                            </div>
                                        </div>

                                        {/* Teacher Info */}
                                        <div className="flex-1 min-w-0">
                                            <h3 className="text-lg font-semibold text-gray-900 md:group-hover:text-orange-600 transition-colors duration-300">
                                                {displayName}
                                            </h3>
                                            <div className="flex items-center gap-2 mt-1 flex-wrap">
                                                {/* Specialization Badge */}
                                                {teacher.specialization && (
                                                    <span className="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-orange-100 text-orange-800">
                                                        {teacher.specialization}
                                                    </span>
                                                )}
                                            </div>
                                            {/* Email info */}
                                            {teacher.email && (
                                                <p className="text-sm text-gray-500 mt-1">
                                                    <span className="text-gray-400">E-Mail:</span> {teacher.email}
                                                </p>
                                            )}
                                        </div>

                                        {/* Arrow Icon */}
                                        <div className="flex-shrink-0">
                                            <svg className="h-6 w-6 text-gray-400 md:group-hover:text-orange-600 md:group-hover:translate-x-1 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                            </svg>
                                        </div>
                                    </div>

                                    {/* Glowing border effect on hover */}
                                    <div className="absolute inset-0 rounded-3xl opacity-0 md:group-hover:opacity-100 transition-opacity duration-300 bg-gradient-to-r from-transparent via-orange-100/30 to-transparent"></div>
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

            {/* Create Teacher Modal */}
            <TeacherCreateModal
                isOpen={showCreateModal}
                onClose={() => setShowCreateModal(false)}
                onCreate={handleCreateTeacher}
                loading={createLoading}
            />

            {/* Teacher Detail Modal */}
            {selectedTeacher && (
                <TeacherDetailModal
                    isOpen={showDetailModal}
                    onClose={() => {
                        setShowDetailModal(false);
                        setSelectedTeacher(null);
                    }}
                    teacher={selectedTeacher}
                    onEdit={handleEditClick}
                    onDelete={handleDeleteTeacher}
                    loading={detailLoading}
                />
            )}

            {/* Teacher Edit Modal */}
            {selectedTeacher && (
                <TeacherEditModal
                    isOpen={showEditModal}
                    onClose={() => {
                        setShowEditModal(false);
                    }}
                    teacher={selectedTeacher}
                    onSave={handleEditTeacher}
                    loading={detailLoading}
                />
            )}

            {/* Role Management Modal */}
            {selectedTeacher && (
                <TeacherRoleManagementModal
                    isOpen={roleModalOpen}
                    onClose={() => {
                        setRoleModalOpen(false);
                        setSelectedTeacher(null);
                    }}
                    teacher={selectedTeacher}
                    onUpdate={() => void fetchTeachers()}
                />
            )}

            {/* Permission Management Modal */}
            {selectedTeacher && (
                <TeacherPermissionManagementModal
                    isOpen={permissionModalOpen}
                    onClose={() => {
                        setPermissionModalOpen(false);
                        setSelectedTeacher(null);
                    }}
                    teacher={selectedTeacher}
                    onUpdate={() => void fetchTeachers()}
                />
            )}

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
