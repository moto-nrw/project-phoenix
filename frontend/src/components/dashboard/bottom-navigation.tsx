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

                    {/* Navigation grid - ultra-modern design */}
                    <div className="px-6 pb-6">
                        <div className="grid grid-cols-2 gap-3 mb-8">
                            {additionalNavItems.map((item) => (
                                <Link
                                    key={item.href}
                                    href={item.href}
                                    onClick={closeOverflowMenu}
                                    className={`group relative flex flex-col items-center p-5 rounded-3xl transition-all duration-200 transform active:scale-95 ${
                                        isActiveLink(item.href)
                                            ? 'bg-gradient-to-br from-blue-500 to-blue-600 shadow-lg shadow-blue-500/25'
                                            : 'bg-white border border-gray-200 hover:border-gray-300 hover:shadow-lg hover:shadow-gray-200/50 hover:-translate-y-1'
                                    }`}
                                >
                                    {/* Subtle background pattern for inactive state */}
                                    {!isActiveLink(item.href) && (
                                        <div className="absolute inset-0 rounded-3xl bg-gradient-to-br from-gray-50 to-gray-100 opacity-0 group-hover:opacity-100 transition-opacity duration-200"></div>
                                    )}
                                    
                                    <div className={`relative w-14 h-14 rounded-2xl flex items-center justify-center mb-3 transition-all duration-200 ${
                                        isActiveLink(item.href) 
                                            ? 'bg-white/20 backdrop-blur-sm' 
                                            : 'bg-gradient-to-br from-gray-100 to-gray-200 group-hover:from-blue-50 group-hover:to-blue-100'
                                    }`}>
                                        <svg className={`w-7 h-7 transition-colors duration-200 ${
                                            isActiveLink(item.href) ? 'text-white' : 'text-gray-600 group-hover:text-blue-600'
                                        }`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                                            {item.href === '/activities' && (
                                                <path strokeLinecap="round" strokeLinejoin="round" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
                                            )}
                                            {item.href === '/statistics' && (
                                                <path strokeLinecap="round" strokeLinejoin="round" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
                                            )}
                                            {item.href === '/substitutions' && (
                                                <path strokeLinecap="round" strokeLinejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                                            )}
                                            {item.href === '/database' && (
                                                <path strokeLinecap="round" strokeLinejoin="round" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" />
                                            )}
                                            {item.href === '/settings' && (
                                                <path strokeLinecap="round" strokeLinejoin="round" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                                            )}
                                        </svg>
                                    </div>
                                    <span className={`relative text-sm font-semibold text-center leading-tight transition-colors duration-200 ${
                                        isActiveLink(item.href) ? 'text-white' : 'text-gray-800 group-hover:text-blue-700'
                                    }`}>
                                        {item.label}
                                    </span>
                                </Link>
                            ))}
                        </div>

                        {/* Modern help section */}
                        <div className="bg-gradient-to-r from-gray-50 to-gray-100 rounded-3xl p-6 border border-gray-200">
                            <div className="flex items-start space-x-4">
                                {/* Help icon */}
                                <div className="flex-shrink-0 w-12 h-12 bg-gradient-to-br from-indigo-500 to-purple-600 rounded-2xl flex items-center justify-center shadow-lg shadow-indigo-500/25">
                                    <svg className="w-6 h-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                                        <path strokeLinecap="round" strokeLinejoin="round" d="M9.879 7.519c0-1.626 1.247-3.019 2.871-3.019s2.871 1.393 2.871 3.019c0 1.626-1.247 3.019-2.871 3.019s-2.871-1.393-2.871-3.019zM12 16v-4M12 20h.01" />
                                    </svg>
                                </div>
                                
                                {/* Help content */}
                                <div className="flex-1 min-w-0">
                                    <h4 className="text-base font-semibold text-gray-900 mb-2">Hilfe benötigt?</h4>
                                    <p className="text-sm text-gray-600 mb-4 leading-relaxed">
                                        Erhalten Sie Unterstützung bei der Navigation und den verfügbaren Funktionen.
                                    </p>
                                    
                                    {/* Help button */}
                                    <HelpButton
                                        title="Dashboard Hilfe"
                                        content={
                                            <div className="space-y-4">
                                                <div>
                                                    <h3 className="text-lg font-semibold text-gray-900 mb-3">📱 Mobile Navigation</h3>
                                                    <p className="text-sm text-gray-600 mb-3">
                                                        Verwenden Sie die Tabs am unteren Bildschirmrand für schnellen Zugriff auf die Hauptfunktionen.
                                                    </p>
                                                </div>
                                                
                                                <div>
                                                    <h4 className="font-semibold text-gray-800 mb-2">🏠 Hauptbereiche</h4>
                                                    <div className="grid grid-cols-2 gap-2 text-xs">
                                                        <div className="bg-blue-50 p-2 rounded-lg">
                                                            <strong className="text-blue-700">Home:</strong> Dashboard-Übersicht
                                                        </div>
                                                        <div className="bg-green-50 p-2 rounded-lg">
                                                            <strong className="text-green-700">Gruppe:</strong> OGS-Verwaltung
                                                        </div>
                                                        <div className="bg-purple-50 p-2 rounded-lg">
                                                            <strong className="text-purple-700">Suchen:</strong> Schüler finden
                                                        </div>
                                                        <div className="bg-orange-50 p-2 rounded-lg">
                                                            <strong className="text-orange-700">Räume:</strong> Raumverwaltung
                                                        </div>
                                                    </div>
                                                </div>

                                                <div>
                                                    <h4 className="font-semibold text-gray-800 mb-2">⚡ Weitere Funktionen</h4>
                                                    <p className="text-xs text-gray-600">
                                                        Aktivitäten, Statistiken, Vertretungen und weitere Tools finden Sie im "Mehr" Menü.
                                                    </p>
                                                </div>
                                            </div>
                                        }
                                        buttonClassName="inline-flex items-center px-4 py-2 bg-white border border-gray-300 rounded-xl text-sm font-medium text-gray-700 hover:bg-gray-50 hover:border-gray-400 transition-all duration-200 shadow-sm"
                                    />
                                </div>
                            </div>
                        </div>
                    </div>

                    {/* Safe area bottom padding for mobile devices */}
                    <div className="h-6 bg-white"></div>
                </div>
            </div>

            {/* Bottom navigation bar */}
            <nav className={`md:hidden fixed bottom-0 left-0 right-0 bg-white/95 backdrop-blur-md border-t border-gray-200/50 z-30 ${className}`}>
                <div className="relative">
                    {/* Subtle gradient overlay */}
                    <div className="absolute inset-0 bg-gradient-to-r from-blue-100/90 via-white/50 to-green-100/90"></div>
                    <div className="relative flex items-center justify-around py-2">
                    {/* Main navigation items */}
                    {mainNavItems.map((item) => {
                        const Icon = item.icon;
                        return (
                            <Link
                                key={item.href}
                                href={item.href}
                                className={`flex flex-col items-center py-2 px-3 rounded-xl transition-all duration-200 ${
                                    isActiveLink(item.href)
                                        ? 'text-blue-600 bg-gradient-to-br from-blue-100 to-green-100 shadow-sm'
                                        : 'text-gray-600 hover:text-blue-600 hover:bg-gray-50/50'
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
                        className={`flex flex-col items-center py-2 px-3 rounded-xl transition-all duration-200 ${
                            isOverflowMenuOpen ? 'text-blue-600 bg-gradient-to-br from-blue-100 to-green-100 shadow-sm' : 'text-gray-600 hover:text-blue-600 hover:bg-gray-50/50'
                        }`}
                    >
                        <MoreIcon className="w-6 h-6 mb-1" />
                        <span className="text-xs font-medium">Mehr</span>
                    </button>
                    </div>
                </div>
            </nav>
        </>
    );
}