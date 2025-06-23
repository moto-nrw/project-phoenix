// components/dashboard/header.tsx (Updated with space-between layout)
"use client";

import Link from "next/link";
import Image from "next/image";

import { useState } from "react";
import { usePathname } from "next/navigation";
import { HelpButton } from "@/components/ui/help_button";
import { getHelpContent } from "@/lib/help-content";
import { GlobalSearch } from "./global-search";
import { MobileSearchModal } from "./mobile-search-modal";
import { NotificationCenter } from "./notification-center";
import { MobileNotificationModal } from "./mobile-notification-modal";
import { useSession } from "next-auth/react";

// Function to get page title based on pathname
function getPageTitle(pathname: string): string {
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
        if (pathname.includes("/activities")) return "Aktivitäten Datenbank";
        if (pathname.includes("/groups")) return "Gruppen Datenbank";
        if (pathname.includes("/students")) return "Schüler Datenbank";
        if (pathname.includes("/teachers")) return "Datenbank Pädagogische Fachkräfte";
        if (pathname.includes("/rooms")) return "Räume Datenbank";
        return "Datenbank";
    }

    // Handle main routes
    switch (pathname) {
        case "/dashboard":
        case "/":
            return "Home";
        case "/ogs_groups":
            return "OGS Gruppen";
        case "/students":
            return "Schüler";
        case "/students/search":
            return "Schüler Suche";
        case "/rooms":
            return "Räume";
        case "/activities":
            return "Aktivitäten";
        case "/statistics":
            return "Statistiken";
        case "/substitutions":
            return "Vertretungen";
        case "/database":
            return "Datenbank";
        case "/settings":
            return "Einstellungen";
        default:
            return "Home";
    }
}


interface HeaderProps {
    userName?: string;
    userEmail?: string;
    userRole?: string;
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



export function Header({ userName = "Benutzer", userEmail = "", userRole = "" }: HeaderProps) {
    const [isProfileMenuOpen, setIsProfileMenuOpen] = useState(false);
    const [isMobileSearchOpen, setIsMobileSearchOpen] = useState(false);
    const [isMobileNotificationOpen, setIsMobileNotificationOpen] = useState(false);
    const pathname = usePathname();
    const helpContent = getHelpContent(pathname);
    const pageTitle = getPageTitle(pathname);
    const { data: session } = useSession();

    // Mock notification state for mobile button
    const [hasUnreadNotifications] = useState(true); // This would come from global state

    const toggleProfileMenu = () => {
        setIsProfileMenuOpen(!isProfileMenuOpen);
    };

    const closeProfileMenu = () => {
        setIsProfileMenuOpen(false);
    };

    const openMobileSearch = () => {
        setIsMobileSearchOpen(true);
    };

    const closeMobileSearch = () => {
        setIsMobileSearchOpen(false);
    };

    const openMobileNotification = () => {
        setIsMobileNotificationOpen(true);
    };

    const closeMobileNotification = () => {
        setIsMobileNotificationOpen(false);
    };


    return (
        <header className="sticky top-0 w-full bg-white/80 backdrop-blur-xl border-b border-gray-100 z-50">
            {/* Subtle top accent line */}
            <div className="h-0.5 bg-gradient-to-r from-[#5080d8] via-gray-200 to-[#83cd2d]"></div>
            
            <div className="w-full px-4 sm:px-6 lg:px-8">
                <div className="flex items-center h-16 w-full">
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
                                    className="text-xl font-bold tracking-tight transition-all duration-200 group-hover:scale-105"
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
                        
                        {/* Context indicator */}
                        <span className="hidden md:inline text-sm font-medium text-gray-600">
                            {pageTitle}
                        </span>
                    </div>

                    {/* Search bar centered horizontally - or session expiry warning */}
                    <div className="hidden lg:flex absolute left-1/2 transform -translate-x-1/2">
                        {session?.error === "RefreshTokenExpired" ? (
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
                        ) : (
                            <GlobalSearch className="w-80" />
                        )}
                    </div>

                    {/* Right section: Actions + Profile */}
                    <div className="flex items-center space-x-3 ml-auto">{/* ml-auto pushes content to the right */}
                        {/* Quick action buttons (desktop only) */}
                        <div className="hidden lg:flex items-center space-x-2">
                            {/* Notifications */}
                            <NotificationCenter />

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
                            
                            {/* Mobile notifications button */}
                            <button 
                                onClick={openMobileNotification}
                                className="relative p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded-lg transition-colors duration-200 active:bg-gray-200"
                                aria-label="Benachrichtigungen öffnen"
                            >
                                <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
                                </svg>
                                {/* Mobile notification dot - only show when there are unread notifications */}
                                {hasUnreadNotifications && (
                                    <div className="absolute top-1.5 right-1.5 w-2 h-2 bg-red-500 rounded-full"></div>
                                )}
                            </button>

                            {/* Mobile search button - hide when session expired */}
                            {session?.error !== "RefreshTokenExpired" && (
                                <button 
                                    onClick={openMobileSearch}
                                    className="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded-lg transition-colors duration-200 active:bg-gray-200"
                                    aria-label="Suche öffnen"
                                >
                                    <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                                    </svg>
                                </button>
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
                            <div className={`absolute right-0 top-full mt-2 w-64 bg-white rounded-xl shadow-xl border border-gray-200 transition-all duration-200 z-50 ${
                                isProfileMenuOpen ? 'opacity-100 visible' : 'opacity-0 invisible'
                            }`}>
                                {/* User info header */}
                                <div className="px-4 py-3 border-b border-gray-100">
                                    <div className="flex items-center space-x-3">
                                        <div 
                                            className="w-10 h-10 rounded-full flex items-center justify-center text-white font-semibold"
                                            style={{
                                                background: 'linear-gradient(135deg, #5080d8, #83cd2d)'
                                            }}
                                        >
                                            {userName.split(' ').map(n => n[0]).join('').toUpperCase()}
                                        </div>
                                        <div>
                                            <div className="font-medium text-gray-900">{userName}</div>
                                            <div className="text-sm text-gray-500">{userEmail}</div>
                                        </div>
                                    </div>
                                </div>

                                {/* Menu items */}
                                <div className="py-2">
                                    <Link 
                                        href="/profile" 
                                        onClick={closeProfileMenu}
                                        className="flex items-center px-4 py-2 text-sm text-gray-700 hover:bg-gray-50 active:bg-gray-100 transition-colors duration-150"
                                    >
                                        <svg className="w-4 h-4 mr-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                                        </svg>
                                        Profil anzeigen
                                    </Link>
                                    
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
                                    
                                    <Link 
                                        href="/logout" 
                                        onClick={closeProfileMenu}
                                        className="flex items-center px-4 py-2 text-sm text-red-600 hover:bg-red-50 active:bg-red-100 transition-colors duration-150"
                                    >
                                        <LogoutIcon className="w-4 h-4 mr-3" />
                                        Abmelden
                                    </Link>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
            
            {/* Mobile Search Modal */}
            <MobileSearchModal 
                isOpen={isMobileSearchOpen} 
                onClose={closeMobileSearch} 
            />
            
            {/* Mobile Notification Modal */}
            <MobileNotificationModal 
                isOpen={isMobileNotificationOpen} 
                onClose={closeMobileNotification} 
            />
        </header>
    );
}