// components/dashboard/sidebar.tsx
"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { useSession } from "next-auth/react";
import { UserContextProvider } from "~/lib/usercontext-context";
import { QuickCreateActivityModal } from "~/components/activities/quick-create-modal";
import { useSupervision } from "~/lib/supervision-context";
import { isAdmin } from "~/lib/auth-utils";

// Type für Navigation Items
interface NavItem {
    href: string;
    label: string;
    icon: string;
    requiresAdmin?: boolean;
    requiresGroups?: boolean;
    requiresSupervision?: boolean;
    requiresActiveSupervision?: boolean;
    alwaysShow?: boolean;
}

// Navigation Items
const NAV_ITEMS: NavItem[] = [
    {
        href: "/dashboard",
        label: "Home",
        icon: "M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z",
        requiresAdmin: true
    },
    {
        href: "/ogs_groups",
        label: "Meine Gruppe",
        icon: "M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z",
        requiresGroups: true
    },
    {
        href: "/myroom",
        label: "Mein Raum",
        icon: "M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4",
        requiresActiveSupervision: true
    },
    {
        href: "/students/search",
        label: "Schüler",
        icon: "M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z",
        requiresSupervision: true
    },
    {
        href: "/rooms",
        label: "Räume",
        icon: "M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4",
        requiresAdmin: true
    },
    {
        href: "/activities",
        label: "Aktivitäten",
        icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2",
        requiresAdmin: true
    },
    {
        href: "/statistics",
        label: "Statistiken",
        icon: "M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z",
        requiresAdmin: true
    },
    {
        href: "/substitutions",
        label: "Vertretungen",
        icon: "M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15",
        requiresAdmin: true
    },
    {
        href: "/database",
        label: "Datenverwaltung",
        icon: "M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4",
        requiresAdmin: true
    },
    {
        href: "/settings",
        label: "Einstellungen",
        icon: "M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z",
        alwaysShow: true
    }
];

interface SidebarProps {
    className?: string;
}

function SidebarContent({ className = "" }: SidebarProps) {
    // Aktuelle Route ermitteln
    const pathname = usePathname();

    // Quick create activity modal state
    const [isQuickCreateOpen, setIsQuickCreateOpen] = useState(false);

    // Get session for role checking
    const { data: session } = useSession();

    // Get supervision state
    const { hasGroups, isSupervising, isLoadingGroups, isLoadingSupervision } = useSupervision();

    // Check if user has educational groups (keeping existing hook for compatibility)
    // const { hasEducationalGroups, isLoading } = useHasEducationalGroups();

    // Check if user has any supervision (groups or active room)
    const hasAnySupervision = (!isLoadingGroups && hasGroups) || (!isLoadingSupervision && isSupervising);
    
    // Filter navigation items based on permissions
    const baseFilteredNavItems = NAV_ITEMS.filter(item => {
        // Always show items marked as alwaysShow
        if (item.alwaysShow) {
            return true;
        }

        // Check admin requirement
        if (item.requiresAdmin && !isAdmin(session)) {
            return false;
        }

        // Check group requirement
        if (item.requiresGroups) {
            // Only show for users who are actively supervising groups, not admins
            if (isAdmin(session)) {
                return false;
            }
            // Use the new supervision context
            return !isLoadingGroups && hasGroups;
        }

        // Check supervision requirement (for student search - groups OR room supervision)
        if (item.requiresSupervision) {
            // Only show if user has supervision (will be handled by dynamic logic below)
            if (isAdmin(session)) return false;
            const hasGroupSupervision = !isLoadingGroups && hasGroups;
            const hasRoomSupervision = !isLoadingSupervision && isSupervising;
            return hasGroupSupervision || hasRoomSupervision;
        }

        // Check active supervision requirement (for room menu - only active room supervision)
        if (item.requiresActiveSupervision) {
            // Show only for users actively supervising a room, not for admins or group-only supervisors
            if (isAdmin(session)) return false;
            const hasRoomSupervision = !isLoadingSupervision && isSupervising;
            return hasRoomSupervision;
        }

        return true;
    });

    // If user has no supervision and is not admin, add student search to main nav
    const filteredNavItems = !hasAnySupervision && !isAdmin(session)
        ? [
            ...baseFilteredNavItems,
            {
                href: "/students/search",
                label: "Schüler",
                icon: "M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z",
                alwaysShow: true
            }
          ]
        : baseFilteredNavItems;

    // Funktion zur Überprüfung, ob ein Link aktiv ist
    const isActiveLink = (href: string) => {
        // Exakter Match für Dashboard
        if (href === "/dashboard") {
            return pathname === "/dashboard";
        }

        // Für andere Routen prüfen, ob der aktuelle Pfad mit dem Link-Pfad beginnt
        return pathname.startsWith(href);
    };


    const getLinkClasses = (href: string) => {
        const baseClasses = "flex items-center px-5 py-3 text-base font-medium rounded-lg transition-colors";
        const activeClasses = "bg-blue-50 text-blue-600 border-l-4 border-blue-600";
        const inactiveClasses = "text-gray-700 hover:bg-gray-100 hover:text-blue-600";

        return `${baseClasses} ${isActiveLink(href) ? activeClasses : inactiveClasses}`;
    };

    return (
        <>
            {/* Desktop sidebar */}
            <aside className={`w-64 bg-white border-r border-gray-200 min-h-screen overflow-y-auto ${className}`}>
                <div className="p-4">
                    <nav className="space-y-2">
                        {filteredNavItems.map((item) => (
                            <Link
                                key={item.href}
                                href={item.href}
                                className={getLinkClasses(item.href)}
                            >
                                <svg className="h-6 w-6 mr-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={item.icon} />
                                </svg>
                                {item.label}
                            </Link>
                        ))}
                    </nav>
                    
                    {/* Quick Create Activity Button */}
                    <div className="mt-6 pt-4 border-t border-gray-200">
                        <button
                            onClick={() => setIsQuickCreateOpen(true)}
                            className="w-full flex items-center px-4 py-3 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg transition-colors duration-200 shadow-sm hover:shadow-md"
                        >
                            <svg className="h-5 w-5 mr-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
                            </svg>
                            Schnell-Aktivität
                        </button>
                    </div>
                </div>
            </aside>

            {/* Quick Create Activity Modal */}
            <QuickCreateActivityModal
                isOpen={isQuickCreateOpen}
                onClose={() => setIsQuickCreateOpen(false)}
                onSuccess={() => {
                    setIsQuickCreateOpen(false);
                    // Optional: Show success notification or refresh data
                }}
            />
        </>
    );
}

export function Sidebar({ className = "" }: SidebarProps) {
    return (
        <UserContextProvider>
            <SidebarContent className={className} />
        </UserContextProvider>
    );
}