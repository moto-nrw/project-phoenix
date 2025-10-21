// components/dashboard/mobile-bottom-nav.tsx
// Ultra-minimalist mobile navigation following Instagram/Twitter/Uber patterns
"use client";

import React from "react";
import Link from "next/link";
import { usePathname, useSearchParams } from "next/navigation";
import { useState, useEffect } from "react";
import { useSession } from "next-auth/react";
import { useSupervision } from "~/lib/supervision-context";
import { isAdmin } from "~/lib/auth-utils";
import { navigationIcons } from '~/lib/navigation-icons';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "~/components/ui/sheet";

// Icon component for consistent SVG rendering
const Icon = ({ path, className }: { path: string; className?: string }) => (
  <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
    <path strokeLinecap="round" strokeLinejoin="round" d={path} />
  </svg>
);

interface NavItem {
  href: string;
  label: string;
  iconKey: keyof typeof navigationIcons;
  requiresAdmin?: boolean;
  requiresGroups?: boolean;
  requiresSupervision?: boolean;
  requiresActiveSupervision?: boolean;
  alwaysShow?: boolean;
}

// Main navigation items that appear in the bottom bar (match desktop sidebar)
const mainNavItems: NavItem[] = [
  {
    href: "/dashboard",
    label: "Home",
    iconKey: "home",
    requiresAdmin: true,
  },
  {
    href: "/ogs_groups",
    label: "Gruppe",
    iconKey: "group",
    requiresGroups: true,
  },
  {
    href: "/myroom",
    label: "Raum",
    iconKey: "room",
    requiresActiveSupervision: true,
  },
  {
    href: "/activities",
    label: "Aktivitäten",
    iconKey: "activities",
    alwaysShow: true,
  },
];

// Additional navigation items that appear in the overflow menu
interface AdditionalNavItem {
  href: string;
  label: string;
  iconKey: keyof typeof navigationIcons;
  requiresAdmin?: boolean;
  requiresSupervision?: boolean;
  requiresActiveSupervision?: boolean;
  alwaysShow?: boolean;
}

const additionalNavItems: AdditionalNavItem[] = [
  { href: '/students/search', label: 'Kindersuche', iconKey: 'search', requiresSupervision: true },
  { href: '/staff', label: 'Mitarbeiter', iconKey: 'staff', alwaysShow: true },
  { href: '/rooms', label: 'Räume', iconKey: 'rooms', alwaysShow: true },
  { href: '/substitutions', label: 'Vertretungen', iconKey: 'substitutions', requiresAdmin: true },
  { href: '/database', label: 'Datenverwaltung', iconKey: 'database', requiresAdmin: true },
  { href: '/settings', label: 'Einstellungen', iconKey: 'settings', alwaysShow: true },
];

interface MobileBottomNavProps {
  className?: string;
}

export function MobileBottomNav({ className = '' }: MobileBottomNavProps) {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const [isVisible, setIsVisible] = useState(true);
  const [lastScrollY, setLastScrollY] = useState(0);
  const [isOverflowMenuOpen, setIsOverflowMenuOpen] = useState(false);

  // Get session for role checking
  const { data: session } = useSession();

  // Get supervision state
  const { hasGroups, isSupervising, isLoadingGroups, isLoadingSupervision } = useSupervision();

  // Auto-hide functionality (Instagram/Uber pattern - KEEP)
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
    // Check if we came from this page via the 'from' query parameter
    if (pathname.startsWith("/students/") && searchParams.get("from")?.startsWith(href)) {
      return true;
    }
    return pathname.startsWith(href);
  };

  const closeOverflowMenu = () => {
    setIsOverflowMenuOpen(false);
  };

  // Filter main navigation items based on permissions (same logic as desktop sidebar)
  const filteredMainItems = mainNavItems.filter(item => {
    if (item.alwaysShow) return true;
    if (item.requiresAdmin && !isAdmin(session)) return false;
    if (item.requiresGroups) {
      if (isAdmin(session)) return false;
      if (!hasGroups || isLoadingGroups) return false;
    }
    if (item.requiresSupervision) {
      if (isAdmin(session)) return false;
      const hasGroupSupervision = !isLoadingGroups && hasGroups;
      const hasRoomSupervision = !isLoadingSupervision && isSupervising;
      if (!hasGroupSupervision && !hasRoomSupervision) return false;
    }
    if (item.requiresActiveSupervision) {
      if (isAdmin(session)) return false;
      const hasRoomSupervision = !isLoadingSupervision && isSupervising;
      if (!hasRoomSupervision) return false;
    }
    return true;
  });

  // Check if user has any supervision (groups or active room)
  const hasAnySupervision = (!isLoadingGroups && hasGroups) || (!isLoadingSupervision && isSupervising);

  // Filter additional navigation items based on permissions
  const filteredAdditionalItems = additionalNavItems.filter(item => {
    if (item.alwaysShow) return true;
    if (item.requiresAdmin && !isAdmin(session)) return false;
    if (item.requiresSupervision) {
      if (isAdmin(session)) return false;
      const hasGroupSupervision = !isLoadingGroups && hasGroups;
      const hasRoomSupervision = !isLoadingSupervision && isSupervising;
      if (!hasGroupSupervision && !hasRoomSupervision) return false;
    }
    if (item.requiresActiveSupervision) {
      if (isAdmin(session)) return false;
      const hasRoomSupervision = !isLoadingSupervision && isSupervising;
      if (!hasRoomSupervision) return false;
    }
    return true;
  });

  // Dynamic layout based on available items and supervision status
  const shouldShowInMainNav = !hasAnySupervision && !isAdmin(session);

  const displayMainItems: NavItem[] = shouldShowInMainNav
    ? [
        ...filteredMainItems,
        {
          href: '/students/search',
          label: 'Suchen',
          iconKey: 'search',
          alwaysShow: true
        }
      ]
    : filteredMainItems;

  // Always show overflow menu
  const showOverflowMenu = true;

  // For users without supervision, ensure settings always appears in overflow menu
  const displayAdditionalItems = filteredAdditionalItems;

  // Check if any additional nav item is active
  const isAnyAdditionalNavActive = displayAdditionalItems.some(item => isActiveRoute(item.href));

  return (
    <>
      {/* Spacer to prevent content from being hidden behind fixed nav */}
      <div className="h-16 lg:hidden" />

      {/* Ultra-minimal "Mehr" modal (shadcn/UI Sheet - Instagram/Twitter pattern) */}
      <Sheet open={isOverflowMenuOpen} onOpenChange={setIsOverflowMenuOpen}>
        <SheetContent
          side="bottom"
          className="bg-white rounded-t-2xl border-t border-gray-200 px-0 py-0"
        >
          {/* Header */}
          <SheetHeader className="px-6 py-4 border-b border-gray-100">
            <SheetTitle className="text-[17px] font-semibold text-gray-900">
              Weitere Optionen
            </SheetTitle>
          </SheetHeader>

          {/* Clean list (Twitter/Uber pattern - NOT grid) */}
          <div className="divide-y divide-gray-100">
            {displayAdditionalItems.map((item) => {
              const isActive = isActiveRoute(item.href);

              return (
                <Link
                  key={item.href}
                  href={item.href}
                  onClick={closeOverflowMenu}
                  className="flex items-center gap-4 px-6 py-4 hover:bg-gray-50 active:bg-gray-100 transition-colors min-h-[44px]"
                >
                  {/* Icon (minimal - same as desktop sidebar) */}
                  <Icon
                    path={navigationIcons[item.iconKey]}
                    className={`w-5 h-5 flex-shrink-0 ${isActive ? 'text-[#5080D8]' : 'text-gray-600'}`}
                  />

                  {/* Label */}
                  <span className={`text-base flex-1 ${isActive ? 'font-semibold text-[#5080D8]' : 'font-medium text-gray-900'}`}>
                    {item.label}
                  </span>

                  {/* Chevron right */}
                  <Icon
                    path={navigationIcons.chevronRight}
                    className="w-4 h-4 text-gray-400"
                  />
                </Link>
              );
            })}
          </div>

          {/* Safe area padding */}
          <div className="h-safe-area-inset-bottom" />
        </SheetContent>
      </Sheet>

      {/* Ultra-minimal Bottom Navigation (Instagram/Twitter pattern) */}
      <nav
        className={`
          lg:hidden fixed bottom-0 left-0 right-0 z-30
          bg-white border-t border-gray-200
          transition-transform duration-300 ease-in-out
          ${isVisible ? 'translate-y-0' : 'translate-y-full'}
          ${className}
        `}
      >
        <div className="flex items-center justify-around px-2 py-2">
          {/* Main navigation items */}
          {displayMainItems.map((item) => {
            const isActive = isActiveRoute(item.href);

            return (
              <Link
                key={item.href}
                href={item.href}
                className="flex flex-col items-center justify-center py-2 px-3 min-h-[44px] min-w-[44px] transition-colors"
              >
                {/* Icon - same as desktop sidebar */}
                <Icon
                  path={navigationIcons[item.iconKey]}
                  className={`w-6 h-6 transition-colors ${isActive ? 'text-[#5080D8]' : 'text-gray-500'}`}
                />

                {/* Small label */}
                <span className={`text-xs mt-1 transition-colors ${
                  isActive ? 'text-[#5080D8] font-semibold' : 'text-gray-500 font-medium'
                }`}>
                  {item.label}
                </span>
              </Link>
            );
          })}

          {/* More button - only show if there are overflow items */}
          {showOverflowMenu && (
            <button
              onClick={() => setIsOverflowMenuOpen(true)}
              className="flex flex-col items-center justify-center py-2 px-3 min-h-[44px] min-w-[44px] transition-colors"
            >
              {/* Icon - same as desktop sidebar */}
              <Icon
                path={navigationIcons.more}
                className={`w-6 h-6 transition-colors ${
                  isOverflowMenuOpen || isAnyAdditionalNavActive ? 'text-[#5080D8]' : 'text-gray-500'
                }`}
              />

              {/* Label */}
              <span className={`text-xs mt-1 transition-colors ${
                isOverflowMenuOpen || isAnyAdditionalNavActive ? 'text-[#5080D8] font-semibold' : 'text-gray-500 font-medium'
              }`}>
                Mehr
              </span>
            </button>
          )}
        </div>

        {/* Safe area padding for devices with home indicator */}
        <div className="h-safe-area-inset-bottom" />
      </nav>
    </>
  );
}
