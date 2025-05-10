// components/dashboard/sidebar.tsx
"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

// Type für Navigation Items
interface NavItem {
    href: string;
    label: string;
    icon: string;
}

// Navigation Items als konstante Daten
const NAV_ITEMS: NavItem[] = [
    {
        href: "/dashboard",
        label: "Home",
        icon: "M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z"
    },
    {
        href: "/database",
        label: "Datenbank",
        icon: "M4 7v10c0 2 1.5 3 3.5 3s3.5-1 3.5-3V7c0-2-1.5-3-3.5-3S4 5 4 7zm14-1v12c0 1.1-.9 2-2 2H9.5"
    },
    {
        href: "/rooms",
        label: "Räume",
        icon: "M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
    },
    {
        href: "/students/search",
        label: "Schüler",
        icon: "M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
    },
    {
        href: "/database/activities",
        label: "Aktivitäten",
        icon: "M17 14v6m-3-3h6M6 10h2a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v2a2 2 0 002 2zm10 0h2a2 2 0 002-2V6a2 2 0 00-2-2h-2a2 2 0 00-2 2v2a2 2 0 002 2zM6 20h2a2 2 0 002-2v-2a2 2 0 00-2-2H6a2 2 0 00-2 2v2a2 2 0 002 2z"
    },
    {
        href: "/metrics",
        label: "Statistiken",
        icon: "M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"
    },
    {
        href: "/settings",
        label: "Einstellungen",
        icon: "M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
    },
];

interface SidebarProps {
    className?: string;
}

export function Sidebar({ className = "" }: SidebarProps) {
    // Aktuelle Route ermitteln
    const pathname = usePathname();

    // Funktion zur Überprüfung, ob ein Link aktiv ist
    const isActiveLink = (href: string) => {
        // Exakter Match für Dashboard
        if (href === "/dashboard") {
            return pathname === "/dashboard";
        }

        // Für andere Routen prüfen, ob der aktuelle Pfad mit dem Link-Pfad beginnt
        return pathname.startsWith(href);
    };

    // CSS-Klassen für aktive und normale Links
    const getLinkClasses = (href: string) => {
        const baseClasses = "flex items-center px-3 py-2 text-sm font-medium rounded-lg transition-colors";
        const activeClasses = "bg-blue-50 text-blue-600 border-l-4 border-blue-600";
        const inactiveClasses = "text-gray-700 hover:bg-gray-100 hover:text-blue-600";

        return `${baseClasses} ${isActiveLink(href) ? activeClasses : inactiveClasses}`;
    };

    return (
        <aside className={`w-64 bg-white border-r border-gray-200 min-h-screen ${className}`}>
            <div className="p-4">
                <nav className="space-y-1">
                    {NAV_ITEMS.map((item) => (
                        <Link
                            key={item.href}
                            href={item.href}
                            className={getLinkClasses(item.href)}
                        >
                            <svg className="h-5 w-5 mr-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={item.icon} />
                            </svg>
                            {item.label}
                        </Link>
                    ))}
                </nav>
            </div>
        </aside>
    );
}