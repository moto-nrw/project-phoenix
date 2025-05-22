'use client';

import { useState } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { HelpButton } from '@/components/ui/help_button';

// Navigation Icons
const HomeIcon = ({ className }: { className?: string }) => (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z" />
    </svg>
);

const GroupIcon = ({ className }: { className?: string }) => (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
    </svg>
);

const SearchIcon = ({ className }: { className?: string }) => (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
    </svg>
);

const RoomsIcon = ({ className }: { className?: string }) => (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
    </svg>
);

const MoreIcon = ({ className }: { className?: string }) => (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
    </svg>
);

const ChevronDownIcon = ({ className }: { className?: string }) => (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
    </svg>
);

// Main navigation items that appear in the bottom bar
const mainNavItems = [
    { href: '/dashboard', icon: HomeIcon, label: 'Home' },
    { href: '/ogs_groups', icon: GroupIcon, label: 'Gruppe' },
    { href: '/students/search', icon: SearchIcon, label: 'Suchen' },
    { href: '/rooms', icon: RoomsIcon, label: 'Räume' },
];

// Additional navigation items that appear in the slide-up menu
const additionalNavItems = [
    { href: '/activities', label: 'Aktivitäten' },
    { href: '/statistics', label: 'Statistiken' },
    { href: '/substitutions', label: 'Vertretungen' },
    { href: '/database', label: 'Datenbank' },
    { href: '/settings', label: 'Einstellungen' },
];

interface BottomNavigationProps {
    className?: string;
}

export function BottomNavigation({ className = '' }: BottomNavigationProps) {
    const [isOverflowMenuOpen, setIsOverflowMenuOpen] = useState(false);
    const pathname = usePathname();

    const isActiveLink = (href: string) => {
        if (href === '/dashboard') {
            return pathname === '/dashboard';
        }
        return pathname.startsWith(href);
    };

    const toggleOverflowMenu = () => {
        setIsOverflowMenuOpen(!isOverflowMenuOpen);
    };

    const closeOverflowMenu = () => {
        setIsOverflowMenuOpen(false);
    };

    return (
        <>
            {/* Backdrop for overflow menu */}
            {isOverflowMenuOpen && (
                <div 
                    className="fixed inset-0 bg-black/20 backdrop-blur-sm z-40 md:hidden"
                    onClick={closeOverflowMenu}
                />
            )}

            {/* Modern bottom sheet overflow menu */}
            <div className={`fixed inset-x-0 bottom-0 bg-white z-50 md:hidden transform transition-transform duration-300 ease-out ${
                isOverflowMenuOpen ? 'translate-y-0' : 'translate-y-full'
            }`}>
                {/* Bottom sheet content */}
                <div className="bg-white rounded-t-3xl shadow-2xl border-t border-gray-100">
                    {/* Header with handle and title */}
                    <div className="flex items-center justify-between px-6 pt-6 pb-4">
                        <h3 className="text-lg font-semibold text-gray-900">Weitere Optionen</h3>
                        <button
                            onClick={closeOverflowMenu}
                            className="p-2 rounded-full bg-gray-100 hover:bg-gray-200 transition-colors"
                            aria-label="Menü schließen"
                        >
                            <svg className="w-5 h-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                            </svg>
                        </button>
                    </div>

                    {/* Navigation grid - modern card-based layout */}
                    <div className="px-6 pb-8">
                        <div className="grid grid-cols-2 gap-4 mb-6">
                            {additionalNavItems.map((item) => (
                                <Link
                                    key={item.href}
                                    href={item.href}
                                    onClick={closeOverflowMenu}
                                    className={`flex flex-col items-center p-6 rounded-2xl border-2 transition-all ${
                                        isActiveLink(item.href)
                                            ? 'border-blue-500 bg-blue-50 shadow-md'
                                            : 'border-gray-200 bg-gray-50 hover:bg-gray-100 hover:border-gray-300'
                                    }`}
                                >
                                    <div className={`w-12 h-12 rounded-full flex items-center justify-center mb-3 ${
                                        isActiveLink(item.href) ? 'bg-blue-500' : 'bg-gray-400'
                                    }`}>
                                        <svg className="w-6 h-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            {item.href === '/activities' && (
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
                                            )}
                                            {item.href === '/statistics' && (
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
                                            )}
                                            {item.href === '/substitutions' && (
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                                            )}
                                            {item.href === '/database' && (
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" />
                                            )}
                                            {item.href === '/settings' && (
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                                            )}
                                        </svg>
                                    </div>
                                    <span className={`text-sm font-medium text-center ${
                                        isActiveLink(item.href) ? 'text-blue-700' : 'text-gray-700'
                                    }`}>
                                        {item.label}
                                    </span>
                                </Link>
                            ))}
                        </div>

                        {/* Help section with modern styling */}
                        <div className="border-t border-gray-200 pt-4">
                            <div className="flex items-center justify-center">
                                <HelpButton
                                    title="Allgemeine Hilfe"
                                    content={
                                        <div>
                                            <p className="mb-4">Willkommen im <strong>Dashboard-Hilfesystem</strong>!</p>
                                            <h3 className="font-semibold text-lg mb-2">Verfügbare Funktionen:</h3>
                                            <ul className="list-disc list-inside space-y-1 mb-4">
                                                <li><strong>Home</strong>: Übersicht über aktuelle Aktivitäten</li>
                                                <li><strong>Gruppe</strong>: Verwalten Sie Ganztagsgruppen</li>
                                                <li><strong>Schüler</strong>: Suchen und verwalten Sie Schülerdaten</li>
                                                <li><strong>Räume</strong>: Raumverwaltung und -zuweisung</li>
                                                <li><strong>Aktivitäten</strong>: Erstellen und bearbeiten Sie Aktivitäten</li>
                                                <li><strong>Statistiken</strong>: Einblick in wichtige Kennzahlen</li>
                                                <li><strong>Vertretungen</strong>: Vertretungsplan verwalten</li>
                                                <li><strong>Einstellungen</strong>: System konfigurieren</li>
                                            </ul>
                                            <p className="text-sm text-gray-600">
                                                Navigieren Sie über die unteren Tabs oder das "Mehr" Menü.
                                            </p>
                                        </div>
                                    }
                                    buttonClassName="flex items-center gap-3 w-full px-4 py-3 bg-gray-100 rounded-xl hover:bg-gray-200 transition-colors"
                                />
                                <span className="ml-3 text-base font-medium text-gray-700">Hilfe & Support</span>
                            </div>
                        </div>
                    </div>

                    {/* Safe area bottom padding for mobile devices */}
                    <div className="h-6 bg-white"></div>
                </div>
            </div>

            {/* Bottom navigation bar */}
            <nav className={`md:hidden fixed bottom-0 left-0 right-0 bg-white border-t border-gray-200 z-30 ${className}`}>
                <div className="flex items-center justify-around py-2">
                    {/* Main navigation items */}
                    {mainNavItems.map((item) => {
                        const Icon = item.icon;
                        return (
                            <Link
                                key={item.href}
                                href={item.href}
                                className={`flex flex-col items-center py-2 px-3 rounded-lg transition-colors ${
                                    isActiveLink(item.href)
                                        ? 'text-blue-600'
                                        : 'text-gray-600 hover:text-blue-600'
                                }`}
                            >
                                <Icon className="w-6 h-6 mb-1" />
                                <span className="text-xs font-medium">{item.label}</span>
                            </Link>
                        );
                    })}

                    {/* More button */}
                    <button
                        onClick={toggleOverflowMenu}
                        className={`flex flex-col items-center py-2 px-3 rounded-lg transition-colors ${
                            isOverflowMenuOpen ? 'text-blue-600' : 'text-gray-600 hover:text-blue-600'
                        }`}
                    >
                        <MoreIcon className="w-6 h-6 mb-1" />
                        <span className="text-xs font-medium">Mehr</span>
                    </button>
                </div>
            </nav>
        </>
    );
}