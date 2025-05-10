// components/dashboard/sidebar.tsx
"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { HelpButton } from "@/components/ui/help_button";

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
        icon: "M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z"
    },
    {
        href: "/ogs-groups",
        label: "OGS Gruppe",
        icon: "M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
    },
    {
        href: "/students/search",
        label: "Schüler",
        icon: "M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
    },
    {
        href: "/rooms",
        label: "Räume",
        icon: "M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
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
        href: "/substitutions",
        label: "Vertretungen",
        icon: "M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
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
        const baseClasses = "flex items-center px-5 py-3 text-base font-medium rounded-lg transition-colors";
        const activeClasses = "bg-blue-50 text-blue-600 border-l-4 border-blue-600";
        const inactiveClasses = "text-gray-700 hover:bg-gray-100 hover:text-blue-600";

        return `${baseClasses} ${isActiveLink(href) ? activeClasses : inactiveClasses}`;
    };

    return (
        <>
            <aside className={`w-64 bg-white border-r border-gray-200 min-h-screen overflow-y-auto ${className}`}>
                <div className="p-4">
                    <nav className="space-y-2">
                        {NAV_ITEMS.map((item) => (
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
                </div>
            </aside>

            {/* Hilfe-Button fixiert am unteren linken Bildschirmrand - gleiches Breite wie Sidebar */}
            <div className="fixed bottom-0 left-0 z-50 w-64 p-4 bg-white border-t border-r border-gray-200">
                <div className="flex items-center justify-start">
                    <HelpButton
                        title="Allgemeine Hilfe"
                        content={
                            <div>
                                <p className="mb-4">Willkommen im <strong>Dashboard-Hilfesystem</strong>!</p>

                                <h3 className="font-semibold text-lg mb-2">Verfügbare Funktionen:</h3>
                                <ul className="list-disc list-inside space-y-1 mb-4">
                                    <li><strong>Home</strong>: Übersicht über aktuelle Aktivitäten</li>
                                    <li><strong>OGS Gruppe</strong>: Verwalten Sie Ganztagsgruppen</li>
                                    <li><strong>Schüler</strong>: Suchen und verwalten Sie Schülerdaten</li>
                                    <li><strong>Räume</strong>: Raumverwaltung und -zuweisung</li>
                                    <li><strong>Aktivitäten</strong>: Erstellen und bearbeiten Sie Aktivitäten</li>
                                    <li><strong>Statistiken</strong>: Einblick in wichtige Kennzahlen</li>
                                    <li><strong>Vertretungen</strong>: Vertretungsplan verwalten</li>
                                    <li><strong>Einstellungen</strong>: System konfigurieren</li>
                                </ul>

                                <p className="text-sm text-gray-600">
                                    Klicken Sie auf einen <strong>Menüpunkt</strong>, um loszulegen.
                                </p>
                            </div>
                        }
                        buttonClassName="mr-2"
                    />
                    <span className="text-sm text-gray-600">Hilfe</span>
                </div>
            </div>
        </>
    );
}