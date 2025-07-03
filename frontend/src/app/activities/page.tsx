"use client";

import { useState, useEffect, useMemo } from "react";
import { createPortal } from "react-dom";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header/types";
import { fetchActivities, getCategories } from "~/lib/activity-api";
import { 
    formatSupervisorList, 
    isActivityCreator,
    type Activity, 
    type ActivityCategory
} from "~/lib/activity-helpers";
import { ActivityManagementModal } from "~/components/activities/activity-management-modal";
import { QuickCreateActivityModal } from "~/components/activities/quick-create-modal";
import { userContextService } from "~/lib/usercontext-api";
import type { Staff } from "~/lib/usercontext-helpers";
import { SimpleAlert, alertAnimationStyles } from "~/components/simple/SimpleAlert";


export default function ActivitiesPage() {
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [activities, setActivities] = useState<Activity[]>([]);
    const [categories, setCategories] = useState<ActivityCategory[]>([]);
    const [searchTerm, setSearchTerm] = useState("");
    const [categoryFilter, setCategoryFilter] = useState("all");
    const [myActivitiesFilter, setMyActivitiesFilter] = useState(false);
    const [filteredActivities, setFilteredActivities] = useState<Activity[]>([]);
    const [selectedActivity, setSelectedActivity] = useState<Activity | null>(null);
    const [isManagementModalOpen, setIsManagementModalOpen] = useState(false);
    const [isQuickCreateOpen, setIsQuickCreateOpen] = useState(false);
    const [currentStaff, setCurrentStaff] = useState<Staff | null>(null);
    const [isAlertShowing, setIsAlertShowing] = useState(false);
    const [isNavBarHidden, setIsNavBarHidden] = useState(false);
    const [showManagementSuccess, setShowManagementSuccess] = useState(false);
    const [managementSuccessMessage, setManagementSuccessMessage] = useState("");

    // Listen for alert show/hide events to move FAB
    useEffect(() => {
        const handleAlertShow = () => setIsAlertShowing(true);
        const handleAlertHide = () => setIsAlertShowing(false);

        window.addEventListener('alert-show', handleAlertShow);
        window.addEventListener('alert-hide', handleAlertHide);

        return () => {
            window.removeEventListener('alert-show', handleAlertShow);
            window.removeEventListener('alert-hide', handleAlertHide);
        };
    }, []);

    // Track scroll position to detect when mobile nav bar is hidden
    useEffect(() => {
        let lastScrollY = window.scrollY;
        let ticking = false;

        const handleScroll = () => {
            if (!ticking) {
                window.requestAnimationFrame(() => {
                    const currentScrollY = window.scrollY;
                    
                    // Mobile navigation typically hides when scrolling down past a threshold
                    if (currentScrollY > lastScrollY && currentScrollY > 50) {
                        // Scrolling down - nav bar should be hidden
                        setIsNavBarHidden(true);
                    } else if (currentScrollY < lastScrollY || currentScrollY <= 50) {
                        // Scrolling up or at top - nav bar should be visible
                        setIsNavBarHidden(false);
                    }
                    
                    lastScrollY = currentScrollY;
                    ticking = false;
                });
                
                ticking = true;
            }
        };

        window.addEventListener('scroll', handleScroll, { passive: true });

        return () => {
            window.removeEventListener('scroll', handleScroll);
        };
    }, []);

    // Load activities, categories and current user on mount
    useEffect(() => {
        const loadData = async () => {
            try {
                setLoading(true);
                // Load activities, categories and current user in parallel
                const [activitiesData, categoriesData, staffData] = await Promise.all([
                    fetchActivities(),
                    getCategories(),
                    userContextService.getCurrentStaff().catch(() => null) // Fail silently if not a staff member
                ]);
                setActivities(activitiesData);
                setCategories(categoriesData);
                setFilteredActivities(activitiesData);
                setCurrentStaff(staffData);
                setError(null);
            } catch (err) {
                setError("Fehler beim Laden der Aktivitäten");
                console.error("Error loading activities:", err);
            } finally {
                setLoading(false);
            }
        };

        void loadData();
    }, []);

    // Apply filters
    useEffect(() => {
        // Ensure activities is an array before filtering
        const activityList = Array.isArray(activities) ? activities : [];
        let filtered = [...activityList];

        // Apply search filter
        if (searchTerm) {
            const searchLower = searchTerm.toLowerCase();
            filtered = filtered.filter(activity =>
                activity.name.toLowerCase().includes(searchLower) ||
                formatSupervisorList(activity.supervisors).toLowerCase().includes(searchLower) ||
                (activity.category_name?.toLowerCase().includes(searchLower) ?? false)
            );
        }

        // Apply category filter
        if (categoryFilter !== "all") {
            filtered = filtered.filter(activity => activity.ag_category_id === categoryFilter);
        }

        // Apply "My Activities" filter
        if (myActivitiesFilter && currentStaff?.id) {
            filtered = filtered.filter(activity => isActivityCreator(activity, currentStaff.id));
        }

        // Sort activities alphabetically by name
        filtered.sort((a, b) => a.name.localeCompare(b.name, 'de'));

        setFilteredActivities(filtered);
    }, [searchTerm, categoryFilter, myActivitiesFilter, activities, currentStaff]);

    // Handle activity selection - open management modal
    const handleSelectActivity = (activity: Activity) => {
        setSelectedActivity(activity);
        setIsManagementModalOpen(true);
    };

    // Handle edit button click - same as selecting activity
    const handleEditActivity = (e: React.MouseEvent, activity: Activity) => {
        e.stopPropagation(); // Prevent duplicate modal opening
        handleSelectActivity(activity);
    };

    // Handle successful management actions (edit/delete)
    const handleManagementSuccess = async (message?: string) => {
        // Show success message if provided
        if (message) {
            setManagementSuccessMessage(message);
            setShowManagementSuccess(true);
        }
        
        // Reload activities to show updated data
        try {
            const activitiesData = await fetchActivities();
            setActivities(activitiesData);
            setFilteredActivities(activitiesData);
        } catch (err) {
            console.error("Error reloading activities:", err);
        }
    };

    // Prepare filters for PageHeaderWithSearch
    const filters: FilterConfig[] = useMemo(() => {
        const baseFilters: FilterConfig[] = [
            {
                id: 'category',
                label: 'Kategorie',
                type: 'dropdown',
                value: categoryFilter,
                onChange: (value: string | string[]) => setCategoryFilter(value as string),
                options: [
                    { value: "all", label: "Alle Kategorien" },
                    ...categories.map(cat => ({
                        value: cat.id.toString(),
                        label: cat.name
                    }))
                ]
            }
        ];

        // Only show "My Activities" filter if user is a staff member
        if (currentStaff) {
            baseFilters.push({
                id: 'myActivities',
                label: 'Meine Aktivitäten',
                type: 'buttons',
                value: myActivitiesFilter ? 'my' : 'all',
                onChange: (value: string | string[]) => setMyActivitiesFilter(value === 'my'),
                options: [
                    { value: 'all', label: 'Alle' },
                    { value: 'my', label: 'Meine' }
                ]
            });
        }

        return baseFilters;
    }, [categoryFilter, categories, myActivitiesFilter, currentStaff]);

    // Prepare active filters for display
    const activeFilters: ActiveFilter[] = useMemo(() => {
        const filters: ActiveFilter[] = [];
        
        if (searchTerm) {
            filters.push({
                id: 'search',
                label: `"${searchTerm}"`,
                onRemove: () => setSearchTerm("")
            });
        }
        
        if (categoryFilter !== "all") {
            const category = categories.find(cat => cat.id.toString() === categoryFilter);
            filters.push({
                id: 'category',
                label: category?.name ?? 'Kategorie',
                onRemove: () => setCategoryFilter("all")
            });
        }
        
        if (myActivitiesFilter) {
            filters.push({
                id: 'myActivities',
                label: 'Meine Aktivitäten',
                onRemove: () => setMyActivitiesFilter(false)
            });
        }
        
        return filters;
    }, [searchTerm, categoryFilter, myActivitiesFilter, categories]);

    if (loading) {
        return (
            <ResponsiveLayout>
                <div className="flex min-h-[50vh] items-center justify-center">
                    <div className="flex flex-col items-center gap-4">
                        <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-[#5080D8]"></div>
                        <p className="text-gray-600">Aktivitäten werden geladen...</p>
                    </div>
                </div>
            </ResponsiveLayout>
        );
    }

    return (
        <ResponsiveLayout>
            <div className="w-full">
                {/* Custom Page Header with Create Button */}
                <div>
                    {/* Header with Title and Create Button */}
                    <div className="mb-0 md:mb-6">
                        <div className="flex items-center justify-between gap-4">
                            <h1 className="text-[1.625rem] md:text-3xl font-bold text-gray-900">
                                Aktivitäten
                            </h1>
                            
                            {/* Create Activity Button - Desktop */}
                            <button
                                onClick={() => setIsQuickCreateOpen(true)}
                                className="hidden md:flex items-center gap-2.5 px-5 py-2.5 bg-gradient-to-r from-white to-gray-50 hover:from-gray-50 hover:to-gray-100 text-gray-700 hover:text-gray-900 rounded-full border border-gray-200 hover:border-[#83CD2D]/30 shadow-sm hover:shadow-md transition-all duration-300 group relative overflow-hidden"
                            >
                                {/* Subtle gradient overlay on hover */}
                                <div className="absolute inset-0 bg-gradient-to-r from-[#83CD2D]/0 to-[#70B525]/0 group-hover:from-[#83CD2D]/5 group-hover:to-[#70B525]/5 transition-all duration-300"></div>
                                
                                <div className="relative w-5 h-5 rounded-full bg-gradient-to-br from-[#83CD2D] to-[#70B525] flex items-center justify-center flex-shrink-0 transition-all duration-300 group-hover:scale-110 group-hover:shadow-sm group-hover:shadow-[#83CD2D]/30">
                                    <svg className="h-3 w-3 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={3}>
                                        <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
                                    </svg>
                                </div>
                                <span className="relative font-semibold text-sm">Aktivität erstellen</span>
                            </button>
                        </div>
                    </div>

                    {/* Search and Filters using PageHeaderWithSearch components */}
                    <PageHeaderWithSearch
                        title=""  
                        search={{
                            value: searchTerm,
                            onChange: setSearchTerm,
                            placeholder: "Aktivität suchen..."
                        }}
                        filters={filters}
                        activeFilters={activeFilters}
                        onClearAllFilters={() => {
                            setSearchTerm("");
                            setCategoryFilter("all");
                            setMyActivitiesFilter(false);
                        }}
                        className="-mt-3 md:mt-0"
                    />
                </div>

                {/* Mobile FAB Create Button */}
                <button
                    onClick={() => setIsQuickCreateOpen(true)}
                    className={`md:hidden fixed right-4 z-[9999] w-14 h-14 bg-gradient-to-br from-[#83CD2D] to-[#70B525] text-white rounded-full shadow-[0_8px_30px_rgb(0,0,0,0.12)] hover:shadow-[0_8px_40px_rgb(131,205,45,0.3)] transition-all duration-300 flex items-center justify-center group active:scale-95`}
                    style={{
                        bottom: isAlertShowing 
                            ? (isNavBarHidden ? '8rem' : '11rem')  // 32 (128px) when alert showing and nav hidden, 44 (176px) when nav visible
                            : (isNavBarHidden ? '1.5rem' : '6rem') // 6 (24px) when nav hidden, 24 (96px) when nav visible
                    }}
                    aria-label="Aktivität erstellen"
                >
                    {/* Inner glow effect */}
                    <div className="absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300"></div>
                    
                    {/* Plus icon */}
                    <svg className="relative h-6 w-6 transition-transform duration-300 group-active:rotate-90" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
                    </svg>
                    
                    {/* Ripple effect on hover */}
                    <div className="absolute inset-0 rounded-full bg-white/20 scale-0 group-hover:scale-100 transition-transform duration-500 opacity-0 group-hover:opacity-100"></div>
                </button>

                {/* Error Alert */}
                {error && (
                    <div className="mb-6 rounded-lg bg-red-50 border border-red-200 p-4">
                        <p className="text-sm text-red-800">{error}</p>
                    </div>
                )}

                {/* Activity List - Modern Design */}
                {filteredActivities.length > 0 ? (
                    <div className="space-y-3">
                        {filteredActivities.map((activity, index) => {
                            const isGruppenraum = activity.category_name === "Gruppenraum";
                            
                            return (
                                <div
                                    key={activity.id}
                                    onClick={() => handleSelectActivity(activity)}
                                    className="group cursor-pointer relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500 md:hover:scale-[1.01] md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)] md:hover:bg-white md:hover:-translate-y-1 active:scale-[0.99] md:hover:border-blue-200/50"
                                    style={{
                                        animationName: 'fadeInUp',
                                        animationDuration: '0.5s',
                                        animationTimingFunction: 'ease-out',
                                        animationFillMode: 'forwards',
                                        animationDelay: `${index * 0.05}s`,
                                        opacity: 0
                                    }}
                                >
                                    {/* Modern gradient overlay */}
                                    <div className="absolute inset-0 bg-gradient-to-br from-blue-50/80 to-cyan-100/80 opacity-[0.03] rounded-3xl"></div>
                                    {/* Subtle inner glow */}
                                    <div className={`absolute ${isGruppenraum ? 'top-px right-px bottom-px left-1.5' : 'inset-px'} rounded-3xl bg-gradient-to-br from-white/80 to-white/20`}></div>
                                    {/* Modern border highlight */}
                                    <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 md:group-hover:ring-blue-200/60 transition-all duration-300"></div>
                                    
                                    {/* Gruppenraum indicator - accent bar */}
                                    {isGruppenraum && (
                                        <div 
                                            className="absolute left-0 top-0 bottom-0 w-1.5 rounded-l-3xl"
                                            style={{ backgroundColor: "#83CD2D", marginLeft: "-1px" }}
                                        ></div>
                                    )}
                                    
                                    <div className={`relative flex items-center justify-between p-5 ${isGruppenraum ? 'pl-6' : ''}`}>
                                        {/* Left content */}
                                        <div className="flex-1 min-w-0">
                                            {/* Activity Name */}
                                            <h3 className="text-lg font-semibold text-gray-900 md:group-hover:text-blue-600 transition-colors duration-300">
                                                {activity.name}
                                            </h3>
                                            
                                            {/* Meta info row */}
                                            <div className="flex items-center gap-4 mt-1">
                                                {/* Creator info */}
                                                <p className="text-sm text-gray-500">
                                                    <span className="text-gray-400">Erstellt von:</span> {formatSupervisorList(activity.supervisors)}
                                                </p>
                                                
                                                
                                                {/* Gruppenraum indicator */}
                                                {isGruppenraum && (
                                                    <div className="flex items-center gap-1.5">
                                                        <div className="w-2 h-2 rounded-full bg-[#83CD2D] animate-pulse"></div>
                                                        <span className="text-sm font-medium text-[#83CD2D]">
                                                            Gruppenraum
                                                        </span>
                                                    </div>
                                                )}
                                            </div>
                                        </div>
                                        
                                        {/* Right content - Edit button (available for all users) */}
                                        <div className="flex items-center gap-3 ml-4">
                                            {/* Desktop hint */}
                                            <span className="hidden lg:block text-xs text-gray-400 group-hover:text-gray-600 transition-colors">
                                                Bearbeiten
                                            </span>
                                            
                                            {/* Edit icon button */}
                                            <button
                                                onClick={(e) => handleEditActivity(e, activity)}
                                                className="relative"
                                                aria-label="Aktivität bearbeiten"
                                            >
                                                <div className="w-10 h-10 rounded-full bg-gray-100 md:group-hover:bg-blue-100 flex items-center justify-center transition-all duration-200 md:group-hover:scale-110">
                                                    <svg 
                                                        className="w-5 h-5 text-gray-600 md:group-hover:text-blue-600 transition-colors" 
                                                        fill="none" 
                                                        viewBox="0 0 24 24" 
                                                        stroke="currentColor"
                                                    >
                                                        <path 
                                                            strokeLinecap="round" 
                                                            strokeLinejoin="round" 
                                                            strokeWidth={2} 
                                                            d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" 
                                                        />
                                                    </svg>
                                                </div>
                                                
                                                {/* Ripple effect on hover */}
                                                <div className="absolute inset-0 rounded-full bg-blue-200/20 scale-0 md:group-hover:scale-100 transition-transform duration-300"></div>
                                            </button>
                                        </div>
                                    </div>

                                    {/* Glowing border effect */}
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
                ) : (
                    <div className="flex min-h-[300px] items-center justify-center">
                        <div className="text-center">
                            <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01" />
                            </svg>
                            <h3 className="mt-4 text-lg font-medium text-gray-900">
                                {searchTerm || categoryFilter !== "all"
                                    ? "Keine Aktivitäten gefunden"
                                    : "Keine Aktivitäten vorhanden"}
                            </h3>
                            <p className="mt-2 text-sm text-gray-600">
                                {searchTerm || categoryFilter !== "all"
                                    ? "Versuchen Sie andere Suchkriterien oder Filter."
                                    : "Es wurden noch keine Aktivitäten erstellt."}
                            </p>
                        </div>
                    </div>
                )}
            </div>
            
            {/* Activity Management Modal */}
            {selectedActivity && (
                <ActivityManagementModal
                    isOpen={isManagementModalOpen}
                    onClose={() => {
                        setIsManagementModalOpen(false);
                        setSelectedActivity(null);
                    }}
                    onSuccess={handleManagementSuccess}
                    activity={selectedActivity}
                    currentStaffId={currentStaff?.id}
                    readOnly={false}
                />
            )}
            
            {/* Quick Create Activity Modal */}
            <QuickCreateActivityModal
                isOpen={isQuickCreateOpen}
                onClose={() => setIsQuickCreateOpen(false)}
                onSuccess={() => {
                    // Don't close here - let the modal handle its own closing
                    void handleManagementSuccess(); // Reload activities
                }}
            />
            
            {/* Management Success Alert - rendered independently */}
            {showManagementSuccess && typeof document !== 'undefined' && createPortal(
                <>
                    {alertAnimationStyles}
                    <SimpleAlert
                        type="success"
                        message={managementSuccessMessage}
                        autoClose
                        duration={3000}
                        onClose={() => setShowManagementSuccess(false)}
                    />
                </>,
                document.body
            )}
        </ResponsiveLayout>
    );
}