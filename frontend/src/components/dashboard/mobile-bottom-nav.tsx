// components/dashboard/mobile-bottom-nav.tsx
"use client";

import React from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState, useEffect } from "react";
import { useSession } from "next-auth/react";
import { useSupervision } from "~/lib/supervision-context";
import { isAdmin } from "~/lib/auth-utils";
import { QuickCreateActivityModal } from "~/components/activities/quick-create-modal";

interface NavItem {
  href: string;
  label: string;
  icon: React.ReactNode;
  activeIcon?: React.ReactNode;
  requiresAdmin?: boolean;
  requiresGroups?: boolean;
  requiresSupervision?: boolean;
  requiresActiveSupervision?: boolean;
  alwaysShow?: boolean;
}

// Main navigation items that appear in the bottom bar
const mainNavItems: NavItem[] = [
  {
    href: "/dashboard",
    label: "Home",
    icon: (
      <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z" />
      </svg>
    ),
    activeIcon: (
      <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z" />
      </svg>
    ),
    requiresAdmin: true,
  },
  {
    href: "/ogs_groups",
    label: "Meine Gruppe",
    icon: (
      <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
      </svg>
    ),
    activeIcon: (
      <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
      </svg>
    ),
    requiresGroups: true,
  },
  {
    href: "/myroom",
    label: "Mein Raum",
    icon: (
      <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
      </svg>
    ),
    activeIcon: (
      <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
      </svg>
    ),
    requiresActiveSupervision: true,
  },
];

// Additional navigation items that appear in the overflow menu
interface AdditionalNavItem {
  href: string;
  label: string;
  requiresAdmin?: boolean;
  requiresSupervision?: boolean;
  requiresActiveSupervision?: boolean;
  alwaysShow?: boolean;
}

const additionalNavItems: AdditionalNavItem[] = [
  { href: '/students/search', label: 'Schüler Suche', requiresSupervision: true },
  { href: '/activities', label: 'Aktivitäten', requiresAdmin: true },
  { href: '/statistics', label: 'Statistiken', requiresAdmin: true },
  { href: '/substitutions', label: 'Vertretungen', requiresAdmin: true },
  { href: '/database', label: 'Datenbank', requiresAdmin: true },
  { href: '/settings', label: 'Einstellungen', alwaysShow: true },
];

// More icon
const MoreIcon = ({ className }: { className?: string }) => (
  <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
    <path strokeLinecap="round" strokeLinejoin="round" d="M4 6h16M4 12h16M4 18h16" />
  </svg>
);

interface MobileBottomNavProps {
  className?: string;
}

export function MobileBottomNav({ className = '' }: MobileBottomNavProps) {
  const pathname = usePathname();
  const [isVisible, setIsVisible] = useState(true);
  const [lastScrollY, setLastScrollY] = useState(0);
  const [isOverflowMenuOpen, setIsOverflowMenuOpen] = useState(false);

  // Get session for role checking
  const { data: session } = useSession();

  // Get supervision state
  const { hasGroups, isSupervising, isLoadingGroups, isLoadingSupervision } = useSupervision();

  // Auto-hide functionality for better UX
  useEffect(() => {
    const handleScroll = () => {
      const currentScrollY = window.scrollY;
      
      if (currentScrollY > lastScrollY && currentScrollY > 100) {
        setIsVisible(false); // Hide when scrolling down
      } else {
        setIsVisible(true); // Show when scrolling up
      }
      
      setLastScrollY(currentScrollY);
    };

    window.addEventListener("scroll", handleScroll, { passive: true });
    return () => window.removeEventListener("scroll", handleScroll);
  }, [lastScrollY]);

  // Check if current path matches nav item
  const isActiveRoute = (href: string) => {
    if (href === "/dashboard") {
      return pathname === "/dashboard" || pathname === "/";
    }
    return pathname.startsWith(href);
  };

  const toggleOverflowMenu = () => {
    setIsOverflowMenuOpen(!isOverflowMenuOpen);
  };

  const closeOverflowMenu = () => {
    setIsOverflowMenuOpen(false);
  };

  // Quick create activity modal state
  const [isQuickCreateOpen, setIsQuickCreateOpen] = useState(false);

  // Filter main navigation items based on permissions
  const filteredMainItems = mainNavItems.filter(item => {
    if (item.alwaysShow) return true;
    if (item.requiresAdmin && !isAdmin(session)) return false;
    if (item.requiresGroups) {
      // Only show for users who are actively supervising groups, not admins
      if (isAdmin(session)) return false;
      if (!hasGroups || isLoadingGroups) return false;
    }
    if (item.requiresSupervision) {
      // Show for users supervising groups OR rooms, but not for admins or regular users
      if (isAdmin(session)) return false;
      const hasGroupSupervision = !isLoadingGroups && hasGroups;
      const hasRoomSupervision = !isLoadingSupervision && isSupervising;
      if (!hasGroupSupervision && !hasRoomSupervision) return false;
    }
    if (item.requiresActiveSupervision) {
      // Show only for users actively supervising a room, not for admins or group-only supervisors
      if (isAdmin(session)) return false;
      const hasRoomSupervision = !isLoadingSupervision && isSupervising;
      if (!hasRoomSupervision) return false;
    }
    return true;
  });

  // Filter additional navigation items based on permissions
  const filteredAdditionalItems = additionalNavItems.filter(item => {
    if (item.alwaysShow) return true;
    if (item.requiresAdmin && !isAdmin(session)) return false;
    if (item.requiresSupervision) {
      // Show for users supervising groups OR rooms, but not for admins or regular users
      if (isAdmin(session)) return false;
      const hasGroupSupervision = !isLoadingGroups && hasGroups;
      const hasRoomSupervision = !isLoadingSupervision && isSupervising;
      if (!hasGroupSupervision && !hasRoomSupervision) return false;
    }
    if (item.requiresActiveSupervision) {
      // Show only for users actively supervising a room, not for admins or group-only supervisors
      if (isAdmin(session)) return false;
      const hasRoomSupervision = !isLoadingSupervision && isSupervising;
      if (!hasRoomSupervision) return false;
    }
    return true;
  });

  // Check if any additional nav item is active
  const isAnyAdditionalNavActive = filteredAdditionalItems.some(item => isActiveRoute(item.href));

  // Check if user has any supervision (groups or active room)
  const hasAnySupervision = (!isLoadingGroups && hasGroups) || (!isLoadingSupervision && isSupervising);
  
  // Dynamic layout based on available items and supervision status
  
  // If user has no supervision, show student search and settings in main nav (no overflow needed)
  const shouldShowInMainNav = !hasAnySupervision && !isAdmin(session);
  
  const displayMainItems: NavItem[] = shouldShowInMainNav
    ? [
        ...filteredMainItems,
        {
          href: '/students/search',
          label: 'Suchen',
          icon: (
            <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
          ),
          activeIcon: (
            <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
          ),
          alwaysShow: true
        },
        { 
          href: '/settings', 
          label: 'Einstellungen',
          icon: (
            <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M10.325 4.317c.426-1.756 2.924-1.756 3.5 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            </svg>
          ),
          activeIcon: (
            <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M10.325 4.317c.426-1.756 2.924-1.756 3.5 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            </svg>
          ),
          alwaysShow: true
        }
      ]
    : filteredMainItems;
  
  // Show overflow menu only if user has supervision OR is admin (then they have additional items)
  const showOverflowMenu = hasAnySupervision || isAdmin(session);

  return (
    <>
      {/* Spacer to prevent content from being hidden behind fixed nav */}
      <div className="h-20 lg:hidden" />
      
      {/* Enhanced backdrop with better blur and animation */}
      {isOverflowMenuOpen && (
        <div 
          className={`fixed inset-0 z-40 lg:hidden transition-all duration-300 ease-out ${
            isOverflowMenuOpen 
              ? 'bg-black/25 backdrop-blur-md opacity-100' 
              : 'bg-black/0 backdrop-blur-none opacity-0'
          }`}
          onClick={closeOverflowMenu}
        />
      )}

      {/* Modern overflow menu with glassmorphism design */}
      <div className={`fixed inset-x-0 bottom-0 z-50 lg:hidden transition-all duration-500 ease-out ${
        isOverflowMenuOpen ? 'translate-y-0 opacity-100' : 'translate-y-full opacity-0'
      }`}>
        {/* Main container with glassmorphism effect */}
        <div className="relative bg-white/95 backdrop-blur-xl rounded-t-3xl shadow-2xl">
          {/* Perfect curved gradient border */}
          <div 
            className="absolute -top-1 -left-1 -right-1 h-8 bg-gradient-to-r from-[#5080d8] via-[#83cd2d] to-[#5080d8] pointer-events-none"
            style={{
              borderTopLeftRadius: '1.6rem',
              borderTopRightRadius: '1.6rem'
            }}
          ></div>
          <div 
            className="absolute top-px left-0 right-0 h-7 bg-white pointer-events-none"
            style={{
              borderTopLeftRadius: '1.5rem',
              borderTopRightRadius: '1.5rem'
            }}
          ></div>
          
          {/* Subtle gradient overlay for depth */}
          <div className="absolute inset-0 bg-gradient-to-t from-white/5 to-transparent rounded-t-3xl pointer-events-none" />
          
          {/* Header section */}
          <div className="relative flex items-center justify-between px-6 pt-5 pb-3">
            {/* Title with enhanced typography */}
            <div className="flex items-center space-x-3">
              <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-[#5080d8]/10 to-[#83cd2d]/10 flex items-center justify-center">
                <svg className="w-4 h-4 text-[#5080d8]" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
                </svg>
              </div>
              <h3 className="text-lg font-semibold text-gray-900 tracking-tight">Weitere Optionen</h3>
            </div>
            
            {/* Modern close button matching header design */}
            <button
              onClick={closeOverflowMenu}
              className="group relative flex items-center justify-center w-9 h-9 rounded-xl bg-gray-100/80 hover:bg-gray-200/80 active:bg-gray-300/80 transition-all duration-200 ease-out transform hover:scale-105 active:scale-95"
              aria-label="Menü schließen"
            >
              <div className="absolute inset-0 rounded-xl bg-gradient-to-br from-white/40 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-200" />
              <svg className="relative w-5 h-5 text-gray-600 group-hover:text-gray-800 transition-colors duration-200" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          {/* Quick Activity Button */}
          <div className="px-6 pb-4">
            <button
              onClick={() => {
                setIsQuickCreateOpen(true);
                closeOverflowMenu();
              }}
              className="w-full flex items-center justify-center px-4 py-3 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-xl transition-all duration-200 shadow-sm hover:shadow-md transform active:scale-95"
            >
              <svg className="h-5 w-5 mr-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
              </svg>
              Schnell-Aktivität
            </button>
          </div>

          {/* Modern navigation grid aligned with header and bottom nav design */}
          <div className="relative px-6 pb-8">
            <div className="grid grid-cols-2 gap-3">
              {filteredAdditionalItems.map((item) => (
                <Link
                  key={item.href}
                  href={item.href}
                  onClick={closeOverflowMenu}
                  className={`group relative flex flex-col items-center p-4 rounded-xl transition-all duration-200 ease-out transform active:scale-95 overflow-hidden ${
                    isActiveRoute(item.href)
                      ? 'bg-blue-50/80 text-[#5080d8] shadow-lg backdrop-blur-sm'
                      : 'bg-white/70 hover:bg-white/90 hover:shadow-xl text-gray-700 hover:text-gray-900 backdrop-blur-md'
                  }`}
                  style={isActiveRoute(item.href) ? {
                    boxShadow: '0 8px 32px rgba(80, 128, 216, 0.15), 0 2px 8px rgba(0, 0, 0, 0.1), inset 0 1px 0 rgba(255, 255, 255, 0.8)'
                  } : {
                    boxShadow: '0 4px 16px rgba(0, 0, 0, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.9), inset 0 -1px 0 rgba(0, 0, 0, 0.05)'
                  }}
                >
                  {/* Active state indicator - matching sidebar style */}
                  {isActiveRoute(item.href) && (
                    <div className="absolute left-0 top-2 bottom-2 w-1 bg-[#5080d8] rounded-r-lg"></div>
                  )}
                  
                  {/* Sophisticated glassmorphism hover effect */}
                  <div 
                    className="absolute inset-0 rounded-xl opacity-0 group-hover:opacity-100 transition-all duration-300"
                    style={{
                      background: 'linear-gradient(135deg, rgba(80, 128, 216, 0.08), rgba(131, 205, 45, 0.05))',
                      backdropFilter: 'blur(8px)'
                    }}
                  ></div>
                  
                  {/* Modern icon container with glassmorphism */}
                  <div 
                    className={`relative w-10 h-10 rounded-lg flex items-center justify-center mb-3 transition-all duration-200 ${
                      isActiveRoute(item.href) 
                        ? 'bg-white/90 backdrop-blur-sm' 
                        : 'bg-white/60 group-hover:bg-white/80 backdrop-blur-sm'
                    }`}
                    style={isActiveRoute(item.href) ? {
                      boxShadow: '0 4px 16px rgba(80, 128, 216, 0.2), inset 0 1px 0 rgba(255, 255, 255, 0.9)'
                    } : {
                      boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1), inset 0 1px 0 rgba(255, 255, 255, 0.8), inset 0 -1px 0 rgba(0, 0, 0, 0.05)'
                    }}
                  >
                    <svg className={`w-5 h-5 transition-colors duration-200 ${
                      isActiveRoute(item.href) ? 'text-[#5080d8]' : 'text-gray-600 group-hover:text-[#5080d8]'
                    }`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      {item.href === '/students/search' && (
                        <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                      )}
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
                        <path strokeLinecap="round" strokeLinejoin="round" d="M10.325 4.317c.426-1.756 2.924-1.756 3.5 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                      )}
                    </svg>
                  </div>
                  
                  {/* Clean typography matching header style */}
                  <span className={`relative text-sm font-medium text-center leading-tight transition-colors duration-200 ${
                    isActiveRoute(item.href) ? 'text-[#5080d8] font-semibold' : 'group-hover:text-gray-900'
                  }`}>
                    {item.label}
                  </span>
                </Link>
              ))}
            </div>
          </div>

          {/* Safe area padding with modern gradient */}
          <div className="h-6 bg-gradient-to-t from-white/95 to-transparent" />
        </div>
      </div>
      
      {/* Bottom Navigation */}
      <nav 
        className={`lg:hidden fixed bottom-0 left-0 right-0 z-30 transition-all duration-300 ease-out ${
          isVisible ? 'translate-y-0' : 'translate-y-full'
        } ${className}`}
      >
        {/* Gradient backdrop blur */}
        <div className="absolute inset-0 bg-gradient-to-t from-white/95 via-white/90 to-transparent backdrop-blur-xl" />
        
        {/* Top accent line */}
        <div className="relative h-0.5 bg-gradient-to-r from-[#5080d8]/30 via-gray-200 to-[#83cd2d]/30" />
        
        <div className="relative px-2 py-2">
          <div className="flex items-center justify-around max-w-md mx-auto">
            {/* Main navigation items */}
            {displayMainItems.map((item) => {
              const isActive = isActiveRoute(item.href);
              
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  className={`
                    group relative flex flex-col items-center justify-center min-w-0 flex-1 px-2 py-2 rounded-xl
                    transition-all duration-300 ease-out
                    ${isActive 
                      ? 'transform scale-105' 
                      : 'text-gray-500 hover:text-gray-700 active:scale-95'
                    }
                  `}
                >
                  
                  {/* Hover effect */}
                  {!isActive && (
                    <div className="absolute inset-0 rounded-xl bg-gray-100 opacity-0 group-hover:opacity-50 group-active:opacity-70 transition-opacity duration-200" />
                  )}
                  
                  {/* Icon container */}
                  <div className="relative flex items-center justify-center mb-1">
                    {/* Icon with active state */}
                    <div className={`transition-all duration-300 ${isActive ? 'scale-110' : 'group-active:scale-90'}`}>
                      {React.cloneElement(
                        (isActive && item.activeIcon ? item.activeIcon : item.icon) as React.ReactElement<{className?: string}>,
                        {
                          className: isActive 
                            ? 'w-6 h-6 text-[#5080d8]' 
                            : 'w-6 h-6'
                        }
                      )}
                    </div>
                  </div>
                  
                  {/* Label */}
                  <span 
                    className={`
                      relative text-xs font-medium truncate max-w-full leading-tight
                      transition-all duration-300
                      ${isActive ? 'font-semibold text-[#5080d8]' : ''}
                    `}
                  >
                    {item.label}
                  </span>
                  
                </Link>
              );
            })}

            {/* More button - only show if there are overflow items */}
            {showOverflowMenu && (
              <button
              onClick={toggleOverflowMenu}
              className={`
                group relative flex flex-col items-center justify-center min-w-0 flex-1 px-2 py-2 rounded-xl
                transition-all duration-300 ease-out
                ${isOverflowMenuOpen || isAnyAdditionalNavActive 
                  ? 'transform scale-105' 
                  : 'text-gray-500 hover:text-gray-700 active:scale-95'
                }
              `}
            >
              
              {/* Hover effect */}
              {!isOverflowMenuOpen && !isAnyAdditionalNavActive && (
                <div className="absolute inset-0 rounded-xl bg-gray-100 opacity-0 group-hover:opacity-50 group-active:opacity-70 transition-opacity duration-200" />
              )}
              
              {/* Icon container */}
              <div className="relative flex items-center justify-center mb-1">
                <div className={`transition-all duration-300 ${(isOverflowMenuOpen || isAnyAdditionalNavActive) ? 'scale-110' : 'group-active:scale-90'}`}>
                  <MoreIcon 
                    className={
                      (isOverflowMenuOpen || isAnyAdditionalNavActive)
                        ? 'w-6 h-6 text-[#5080d8]'
                        : 'w-6 h-6'
                    }
                  />
                </div>
              </div>
              
              {/* Label */}
              <span 
                className={`
                  relative text-xs font-medium truncate max-w-full leading-tight
                  transition-all duration-300
                  ${(isOverflowMenuOpen || isAnyAdditionalNavActive) ? 'font-semibold text-[#5080d8]' : ''}
                `}
              >
                Mehr
              </span>
              
              </button>
            )}
          </div>
        </div>
        
        {/* Safe area padding for devices with home indicator */}
        <div className="h-safe-area-inset-bottom bg-white/80" />
      </nav>

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