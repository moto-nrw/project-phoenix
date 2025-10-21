// components/dashboard/header.tsx (Updated with space-between layout)
"use client";

import Link from "next/link";
import Image from "next/image";

import { useState, useEffect } from "react";
import { usePathname } from "next/navigation";
import { HelpButton } from "@/components/ui/help_button";
import { getHelpContent } from "@/lib/help-content";
import { useSession } from "next-auth/react";
import { LogoutModal } from "~/components/ui/logout-modal";

// Function to get page title based on pathname
function getPageTitle(pathname: string): string {
    // Check for /students/search first before other /students/ paths
    if (pathname === "/students/search") {
        return "Kindersuche";
    }

    // Handle specific routes with dynamic segments
    if (pathname.startsWith("/students/") && pathname !== "/students") {
        if (pathname.includes("/feedback_history")) return "Feedback Historie";
        if (pathname.includes("/mensa_history")) return "Mensa Historie";
        if (pathname.includes("/room_history")) return "Raum Historie";
        return "Schüler Details";
    }

    if (pathname.startsWith("/rooms/") && pathname !== "/rooms") {
        return "Raum Details";
    }

    if (pathname.startsWith("/database/")) {
        if (pathname.includes("/activities")) return "Aktivitäten";
        if (pathname.includes("/groups")) return "Gruppen";
        if (pathname.includes("/students")) return "Kinder";
        if (pathname.includes("/teachers")) return "Betreuer";
        if (pathname.includes("/rooms")) return "Räume";
        if (pathname.includes("/roles")) return "Rollen";
        if (pathname.includes("/devices")) return "Geräte";
        if (pathname.includes("/permissions")) return "Berechtigungen";
        return "Datenbank";
    }

    // Handle main routes
    switch (pathname) {
        case "/dashboard":
        case "/":
            return "Home";
        case "/ogs_groups":
            return "Meine Gruppe";
        case "/myroom":
            return "Mein Raum";
        case "/staff":
            return "Mitarbeiter";
        case "/students":
            return "Schüler";
        case "/rooms":
            return "Räume";
        case "/activities":
            return "Aktivitäten";
        case "/statistics":
            return "Statistiken";
        case "/substitutions":
            return "Vertretungen";
        case "/database":
            return "Datenverwaltung";
        case "/settings":
            return "Einstellungen";
        case "/borndal_feedback":
            return "Borndal Feedback";
        default:
            return "Home";
    }
}

// Helper function to get human-readable label for sub-pages in breadcrumbs
function getSubPageLabel(pathname: string): string {
    // Extract the last segment of the path
    const segments = pathname.split('/').filter(Boolean);
    const lastSegment = segments[segments.length - 1];

    // Map known sub-page paths to human-readable labels
    const subPageLabels: Record<string, string> = {
        'csv-import': 'CSV-Import',
        'create': 'Erstellen',
        'edit': 'Bearbeiten',
        'details': 'Details',
        'permissions': 'Berechtigungen',
    };

    // Return mapped label or capitalized last segment as fallback
    return subPageLabels[lastSegment ?? ''] ??
           (lastSegment ? lastSegment.charAt(0).toUpperCase() + lastSegment.slice(1) : 'Unbekannt');
}


interface HeaderProps {
    userName?: string;
    userEmail?: string;
    userRole?: string;
    customPageTitle?: string;
    studentName?: string; // For student detail pages
    roomName?: string; // For room detail pages
    activityName?: string; // For activity detail pages
    referrerPage?: string; // Where the user came from (for contextual breadcrumbs)
}

// Logout Icon als React Component
const LogoutIcon = ({ className }: { className?: string }) => (
    <svg
        xmlns="http://www.w3.org/2000/svg"
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
        className={className}
    >
        <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
        <polyline points="16 17 21 12 16 7" />
        <line x1="21" y1="12" x2="9" y2="12" />
    </svg>
);



export function Header({ userName = "Benutzer", userEmail = "", userRole = "", customPageTitle, studentName, roomName, activityName, referrerPage }: HeaderProps) {
    const [isProfileMenuOpen, setIsProfileMenuOpen] = useState(false);
    const [isLogoutModalOpen, setIsLogoutModalOpen] = useState(false);
    const [isScrolled, setIsScrolled] = useState(false);
    const pathname = usePathname();
    const helpContent = getHelpContent(pathname);
    const pageTitle = customPageTitle ?? getPageTitle(pathname);
    const { data: session } = useSession();

    // Shrinking header on scroll (Instagram/Twitter pattern - mobile only)
    useEffect(() => {
        const handleScroll = () => {
            const scrollPosition = window.scrollY;
            setIsScrolled(scrollPosition > 20);
        };

        window.addEventListener('scroll', handleScroll, { passive: true });
        return () => window.removeEventListener('scroll', handleScroll);
    }, []);

    // Check if we're on a student detail page (main page, not history subpages)
    const isStudentDetailPage = pathname.startsWith("/students/") &&
                                pathname !== "/students" &&
                                pathname !== "/students/search" &&
                                !pathname.includes("/feedback_history") &&
                                !pathname.includes("/mensa_history") &&
                                !pathname.includes("/room_history");

    // Check if we're on a student history subpage
    const isStudentHistoryPage = pathname.startsWith("/students/") &&
                                  (pathname.includes("/feedback_history") ||
                                   pathname.includes("/mensa_history") ||
                                   pathname.includes("/room_history"));

    // Determine which history type
    const historyType = pathname.includes("/feedback_history") ? "Feedback Historie" :
                       pathname.includes("/mensa_history") ? "Mensa Historie" :
                       pathname.includes("/room_history") ? "Raum Historie" : "";

    // Check if we're on a room detail page
    const isRoomDetailPage = pathname.startsWith("/rooms/") && pathname !== "/rooms";

    // Check if we're on an activities detail page
    const isActivityDetailPage = pathname.startsWith("/activities/") && pathname !== "/activities";

    // Determine breadcrumb based on referrer
    const referrer = referrerPage ?? "/students/search";
    const breadcrumbLabel = referrer.startsWith("/ogs_groups") ? "Meine Gruppe" :
                            referrer.startsWith("/myroom") ? "Mein Raum" :
                            "Kindersuche";

    const toggleProfileMenu = () => {
        setIsProfileMenuOpen(!isProfileMenuOpen);
    };

    const closeProfileMenu = () => {
        setIsProfileMenuOpen(false);
    };


    return (
        <header className={`sticky top-0 w-full bg-white z-50 transition-all duration-300 ${
            isScrolled ? 'shadow-sm' : ''
        }`}>
            {/* Gradient line removed for minimalist design */}

            <div className="w-full px-4 sm:px-6 lg:px-8">
                <div className={`flex items-center w-full transition-all duration-300 ${
                    isScrolled ? 'h-12 lg:h-16' : 'h-14 lg:h-16'
                }`}>
                    {/* Left section: Logo + Brand + Context */}
                    <div className="flex items-center space-x-4 flex-shrink-0">
                        <Link href="/dashboard" className="flex items-center space-x-3 group">
                            <div className="relative transition-transform duration-200 group-hover:scale-110">
                                <Image
                                    src="/images/moto_transparent.png"
                                    alt="moto"
                                    width={40}
                                    height={40}
                                    className="w-9 h-9"
                                />
                                {/* Subtle glow effect */}
                                <div className="absolute inset-0 w-9 h-9 rounded-full bg-gradient-to-br from-[#5080d8]/20 to-[#83cd2d]/20 blur-sm -z-10"></div>
                            </div>
                            
                            <div className="flex items-center space-x-3">
                                <span
                                    className={`font-bold tracking-tight transition-all duration-300 group-hover:scale-105 ${
                                        isScrolled ? 'text-lg lg:text-xl' : 'text-xl'
                                    }`}
                                    style={{
                                        background: 'linear-gradient(135deg, #5080d8, #83cd2d)',
                                        WebkitBackgroundClip: 'text',
                                        backgroundClip: 'text',
                                        WebkitTextFillColor: 'transparent',
                                    }}
                                >
                                    moto
                                </span>
                            </div>
                        </Link>
                        
                        {/* Breadcrumb separator */}
                        <div className="hidden md:block w-px h-5 bg-gray-300"></div>

                        {/* Breadcrumb navigation for database pages */}
                        {pathname.startsWith("/database/") && pathname !== "/database" ? (
                            <nav className="hidden md:flex items-center space-x-2 text-base">
                                <Link
                                    href="/database"
                                    className="font-medium text-gray-500 hover:text-gray-900 transition-colors"
                                >
                                    Datenbank
                                </Link>
                                <svg className="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                </svg>

                                {/* Check if we're on a sub-page (e.g., /database/students/csv-import) */}
                                {pathname.split('/').length > 3 ? (
                                    <>
                                        <Link
                                            href={pathname.split('/').slice(0, 3).join('/')}
                                            className="font-medium text-gray-500 hover:text-gray-900 transition-colors"
                                        >
                                            {pageTitle}
                                        </Link>
                                        <svg className="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                        </svg>
                                        <span className="font-medium text-gray-900">
                                            {getSubPageLabel(pathname)}
                                        </span>
                                    </>
                                ) : (
                                    <span className="font-medium text-gray-900">
                                        {pageTitle}
                                    </span>
                                )}
                            </nav>
                        ) : pathname === "/ogs_groups" ? (
                            /* Breadcrumb for OGS Groups page */
                            <span className="hidden md:inline text-base font-medium text-gray-600">
                                Meine Gruppe
                            </span>
                        ) : pathname === "/myroom" ? (
                            /* Breadcrumb for My Room page */
                            <span className="hidden md:inline text-base font-medium text-gray-600">
                                Mein Raum
                            </span>
                        ) : pathname === "/rooms" ? (
                            /* Breadcrumb for Rooms list page */
                            <span className="hidden md:inline text-base font-medium text-gray-600">
                                Räume
                            </span>
                        ) : pathname === "/activities" ? (
                            /* Breadcrumb for Activities list page */
                            <span className="hidden md:inline text-base font-medium text-gray-600">
                                Aktivitäten
                            </span>
                        ) : pathname === "/staff" ? (
                            /* Breadcrumb for Staff page */
                            <span className="hidden md:inline text-base font-medium text-gray-600">
                                Mitarbeiter
                            </span>
                        ) : pathname === "/substitutions" ? (
                            /* Breadcrumb for Substitutions page */
                            <span className="hidden md:inline text-base font-medium text-gray-600">
                                Vertretungen
                            </span>
                        ) : pathname === "/statistics" ? (
                            /* Breadcrumb for Statistics page */
                            <span className="hidden md:inline text-base font-medium text-gray-600">
                                Statistiken
                            </span>
                        ) : isActivityDetailPage && activityName ? (
                            /* Breadcrumb for Activity Detail pages */
                            <nav className="hidden md:flex items-center space-x-2 text-base">
                                <Link
                                    href="/activities"
                                    className="font-medium text-gray-500 hover:text-gray-900 transition-colors"
                                >
                                    Aktivitäten
                                </Link>
                                <svg className="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                </svg>
                                <span className="font-medium text-gray-900">
                                    {activityName}
                                </span>
                            </nav>
                        ) : isRoomDetailPage && roomName ? (
                            /* Breadcrumb for Room Detail pages */
                            <nav className="hidden md:flex items-center space-x-2 text-base">
                                <Link
                                    href="/rooms"
                                    className="font-medium text-gray-500 hover:text-gray-900 transition-colors"
                                >
                                    Räume
                                </Link>
                                <svg className="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                </svg>
                                <span className="font-medium text-gray-900">
                                    {roomName}
                                </span>
                            </nav>
                        ) : isStudentHistoryPage && studentName ? (
                            /* Breadcrumb for Student History subpages (3 levels) */
                            <nav className={`hidden md:flex items-center space-x-2 transition-all duration-300 ${
                                isScrolled ? 'text-sm' : 'text-base'
                            }`}>
                                <Link
                                    href={referrer}
                                    className="font-medium text-gray-500 hover:text-gray-900 transition-colors"
                                >
                                    {breadcrumbLabel}
                                </Link>
                                <svg className="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                </svg>
                                <Link
                                    href={pathname.split('/').slice(0, 3).join('/')}
                                    className="font-medium text-gray-500 hover:text-gray-900 transition-colors"
                                >
                                    {studentName}
                                </Link>
                                <svg className="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                </svg>
                                <span className="font-medium text-gray-900">
                                    {historyType}
                                </span>
                            </nav>
                        ) : isStudentDetailPage && studentName ? (
                            /* Breadcrumb for Student Detail pages - contextual based on referrer */
                            <nav className={`hidden md:flex items-center space-x-2 transition-all duration-300 ${
                                isScrolled ? 'text-sm' : 'text-base'
                            }`}>
                                <Link
                                    href={referrer}
                                    className="font-medium text-gray-500 hover:text-gray-900 transition-colors"
                                >
                                    {breadcrumbLabel}
                                </Link>
                                <svg className="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                </svg>
                                <span className="font-medium text-gray-900">
                                    {studentName}
                                </span>
                            </nav>
                        ) : (
                            /* Context indicator for other pages - shrinks on scroll */
                            <span className={`hidden md:inline font-medium text-gray-600 transition-all duration-300 ${
                                isScrolled ? 'text-sm' : 'text-base'
                            }`}>
                                {pageTitle}
                            </span>
                        )}
                    </div>

                    {/* Right section: Actions + Profile */}
                    <div className="flex items-center space-x-3 ml-auto">{/* ml-auto pushes content to the right */}
                        {/* Quick action buttons (desktop only) */}
                        <div className="hidden lg:flex items-center space-x-2">
                            {/* Session expiry warning */}
                            {session?.error === "RefreshTokenExpired" && (
                                <div className="flex items-center space-x-2 px-4 py-2 bg-red-50 border border-red-200 rounded-lg">
                                    <svg
                                        className="w-5 h-5 text-red-600 flex-shrink-0"
                                        fill="none"
                                        viewBox="0 0 24 24"
                                        stroke="currentColor"
                                    >
                                        <path
                                            strokeLinecap="round"
                                            strokeLinejoin="round"
                                            strokeWidth={2}
                                            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                                        />
                                    </svg>
                                    <span className="text-sm font-medium text-red-800">
                                        Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.
                                    </span>
                                </div>
                            )}

                            {/* Help */}
                            <HelpButton
                                title={helpContent.title}
                                content={helpContent.content}
                                buttonClassName="!w-[40px] !h-[40px] !min-w-[40px] !min-h-[40px] p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 !bg-transparent rounded-lg transition-colors duration-200"
                            />
                        </div>

                        {/* Mobile action buttons */}
                        <div className="lg:hidden flex items-center space-x-2">
                            {/* Session expiry warning for mobile */}
                            {session?.error === "RefreshTokenExpired" && (
                                <div className="p-2 text-red-600">
                                    <svg
                                        className="w-5 h-5"
                                        fill="none"
                                        viewBox="0 0 24 24"
                                        stroke="currentColor"
                                    >
                                        <path
                                            strokeLinecap="round"
                                            strokeLinejoin="round"
                                            strokeWidth={2}
                                            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                                        />
                                    </svg>
                                </div>
                            )}
                        </div>

                        {/* User menu */}
                        <div className="relative">
                            <button 
                                onClick={toggleProfileMenu}
                                className="flex items-center space-x-2 p-1.5 rounded-lg hover:bg-gray-100 active:bg-gray-200 transition-colors duration-200 touch-manipulation"
                            >
                                <div 
                                    className="w-8 h-8 rounded-full flex items-center justify-center text-white text-sm font-semibold ring-2 ring-white shadow-sm"
                                    style={{
                                        background: 'linear-gradient(135deg, #5080d8, #83cd2d)'
                                    }}
                                >
                                    {userName.split(' ').map(n => n[0]).join('').toUpperCase()}
                                </div>
                                
                                <div className="hidden md:block text-left">
                                    <div className="text-sm font-medium text-gray-900">
                                        {userName}
                                    </div>
                                    <div className="text-xs text-gray-500">
                                        {userRole}
                                    </div>
                                </div>
                                
                                <svg className={`w-4 h-4 text-gray-400 transition-all duration-200 ${isProfileMenuOpen ? 'rotate-180 text-gray-600' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                                </svg>
                            </button>
                            
                            {/* Backdrop for mobile */}
                            {isProfileMenuOpen && (
                                <div 
                                    className="fixed inset-0 z-40 md:hidden"
                                    onClick={closeProfileMenu}
                                />
                            )}
                            
                            {/* Enhanced dropdown menu */}
                            <div className={`absolute right-0 top-full mt-2 w-72 bg-white rounded-xl shadow-xl border border-gray-200 transition-all duration-200 z-50 ${
                                isProfileMenuOpen ? 'opacity-100 visible' : 'opacity-0 invisible'
                            }`}>
                                {/* User info header */}
                                <div className="px-4 py-3 border-b border-gray-100">
                                    <div className="flex items-center space-x-3">
                                        <div
                                            className="w-10 h-10 flex-shrink-0 rounded-full flex items-center justify-center text-white font-semibold"
                                            style={{
                                                background: 'linear-gradient(135deg, #5080d8, #83cd2d)'
                                            }}
                                        >
                                            {userName.split(' ').map(n => n[0]).join('').toUpperCase()}
                                        </div>
                                        <div className="flex-1 min-w-0">
                                            <div className="font-medium text-gray-900 truncate">{userName}</div>
                                            <div className="text-sm text-gray-500 truncate" title={userEmail}>{userEmail}</div>
                                        </div>
                                    </div>
                                </div>

                                {/* Menu items */}
                                <div className="py-2">
                                    <Link
                                        href="/settings"
                                        onClick={closeProfileMenu}
                                        className="flex items-center px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 active:bg-gray-100 transition-colors duration-150"
                                    >
                                        <svg className="w-4 h-4 mr-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.50 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                                        </svg>
                                        Einstellungen
                                    </Link>


                                    {/* Help button in profile menu */}
                                    <button 
                                        onClick={(e) => {
                                            e.preventDefault();
                                            closeProfileMenu();
                                            // Trigger help button click after a small delay to ensure menu closes first
                                            setTimeout(() => {
                                                const helpButton = document.querySelector('[aria-label="Hilfe anzeigen"]');
                                                if (helpButton) {
                                                    (helpButton as HTMLButtonElement).click();
                                                }
                                            }, 100);
                                        }}
                                        className="flex items-center px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 active:bg-gray-100 transition-colors duration-150 w-full text-left"
                                    >
                                        <svg className="w-4 h-4 mr-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                                        </svg>
                                        Hilfe & Support
                                    </button>
                                    
                                    <div className="border-t border-gray-100 my-2"></div>
                                    
                                    <button 
                                        onClick={(e) => {
                                            e.preventDefault();
                                            closeProfileMenu();
                                            setIsLogoutModalOpen(true);
                                        }}
                                        className="flex items-center px-4 py-2 text-sm text-red-600 hover:bg-red-50 active:bg-red-100 transition-colors duration-150 w-full text-left"
                                    >
                                        <LogoutIcon className="w-4 h-4 mr-3" />
                                        Abmelden
                                    </button>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            {/* Logout Modal */}
            <LogoutModal
                isOpen={isLogoutModalOpen}
                onClose={() => setIsLogoutModalOpen(false)}
            />
        </header>
    );
}
