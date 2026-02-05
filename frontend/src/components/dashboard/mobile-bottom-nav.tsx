// components/dashboard/mobile-bottom-nav.tsx
// Ultra-minimalist mobile navigation following Instagram/Twitter/Uber patterns
"use client";

import React, { useRef, useCallback, useState, useEffect } from "react";
import Link from "next/link";
import { usePathname, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { useSupervision } from "~/lib/supervision-context";
import { useShellAuth } from "~/lib/shell-auth-context";
import { isAdmin } from "~/lib/auth-utils";
import { navigationIcons } from "~/lib/navigation-icons";
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "~/components/ui/drawer";

// Icon component for consistent SVG rendering
const Icon = ({ path, className }: { path: string; className?: string }) => (
  <svg
    className={className}
    fill="none"
    viewBox="0 0 24 24"
    stroke="currentColor"
    strokeWidth={2}
  >
    <path strokeLinecap="round" strokeLinejoin="round" d={path} />
  </svg>
);

// Animation timing constant for initial mount transition delay
// This ensures the sliding indicator position is set before enabling smooth transitions
const INITIAL_MOUNT_DELAY_MS = 100;

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

// Static base definitions; actual main items are computed per session
// Admins don't have assigned groups or supervision duties (#608)
const ADMIN_MAIN_ITEMS: NavItem[] = [
  { href: "/dashboard", label: "Home", iconKey: "home", alwaysShow: true },
  {
    href: "/students/search",
    label: "Suchen",
    iconKey: "search",
    alwaysShow: true,
  },
  {
    href: "/activities",
    label: "Aktivitäten",
    iconKey: "activities",
    alwaysShow: true,
  },
  { href: "/rooms", label: "Räume", iconKey: "rooms", alwaysShow: true },
];

const STAFF_MAIN_ITEMS: NavItem[] = [
  { href: "/ogs-groups", label: "Gruppe", iconKey: "group", alwaysShow: true },
  {
    href: "/active-supervisions",
    label: "Aufsicht",
    iconKey: "supervision",
    alwaysShow: true,
  },
  {
    href: "/students/search",
    label: "Suchen",
    iconKey: "search",
    alwaysShow: true,
  },
  {
    href: "/activities",
    label: "Aktivitäten",
    iconKey: "activities",
    alwaysShow: true,
  },
];

const OPERATOR_MAIN_ITEMS: NavItem[] = [
  {
    href: "/operator/suggestions",
    label: "Feedback",
    iconKey: "feedback",
    alwaysShow: true,
  },
  {
    href: "/operator/announcements",
    label: "Ankündigungen",
    iconKey: "bell",
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
  hideForAdmin?: boolean; // Hide from admin users (for caregiver-specific features)
  comingSoon?: boolean; // Show as grayed out "coming soon" feature
}

const additionalNavItems: AdditionalNavItem[] = [
  {
    href: "/activities",
    label: "Aktivitäten",
    iconKey: "activities",
    alwaysShow: true,
  },
  { href: "/staff", label: "Mitarbeiter", iconKey: "staff", alwaysShow: true },
  { href: "/rooms", label: "Räume", iconKey: "rooms", alwaysShow: true },
  {
    href: "/substitutions",
    label: "Vertretungen",
    iconKey: "substitutions",
    requiresAdmin: true,
  },
  {
    href: "/database",
    label: "Datenverwaltung",
    iconKey: "database",
    requiresAdmin: true,
  },
  {
    href: "/settings",
    label: "Einstellungen",
    iconKey: "settings",
    alwaysShow: true,
  },
  {
    href: "/suggestions",
    label: "Feedback",
    iconKey: "feedback",
    alwaysShow: true,
  },
  {
    href: "/time-tracking",
    label: "Zeiterfassung",
    iconKey: "clock",
    alwaysShow: true,
  },
  // Coming soon features - shown to all users
  {
    href: "#",
    label: "Nachrichten",
    iconKey: "chat",
    alwaysShow: true,
    comingSoon: true,
  },
  {
    href: "#",
    label: "Mittagessen",
    iconKey: "utensils",
    alwaysShow: true,
    comingSoon: true,
  },
  // Coming soon features - caregivers only
  {
    href: "#",
    label: "Erinnerungen",
    iconKey: "bell",
    alwaysShow: true,
    hideForAdmin: true,
    comingSoon: true,
  },
  // Coming soon features - admin only
  {
    href: "#",
    label: "Dienstpläne",
    iconKey: "calendar",
    requiresAdmin: true,
    comingSoon: true,
  },
  {
    href: "#",
    label: "Berichte",
    iconKey: "chart",
    requiresAdmin: true,
    comingSoon: true,
  },
];

interface MobileBottomNavProps {
  readonly className?: string;
}

export function MobileBottomNav({ className = "" }: MobileBottomNavProps) {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const [isOverflowMenuOpen, setIsOverflowMenuOpen] = useState(false);

  // Refs for sliding indicator
  const navRefs = useRef<(HTMLAnchorElement | null)[]>([]);
  const moreButtonRef = useRef<HTMLButtonElement | null>(null);
  const [indicatorStyle, setIndicatorStyle] = useState({ width: 0, left: 0 });
  const [indicatorVisible, setIndicatorVisible] = useState(false);
  const isInitialMount = useRef(true);

  // Get session for role checking
  const { data: session } = useSession();

  // Get supervision state
  const { hasGroups, isSupervising, isLoadingGroups, isLoadingSupervision } =
    useSupervision();

  // Get shell auth mode
  const { mode } = useShellAuth();

  // Check if current path matches nav item
  const isActiveRoute = useCallback(
    (href: string) => {
      if (href === "/dashboard") {
        return pathname === "/dashboard" || pathname === "/";
      }
      // Check if we came from this page via the 'from' query parameter
      if (
        pathname.startsWith("/students/") &&
        searchParams.get("from")?.startsWith(href)
      ) {
        return true;
      }
      return pathname.startsWith(href);
    },
    [pathname, searchParams],
  );

  const closeOverflowMenu = () => {
    setIsOverflowMenuOpen(false);
  };

  // Compute main navigation items per role and mode
  const baseMain =
    mode === "operator"
      ? OPERATOR_MAIN_ITEMS
      : isAdmin(session)
        ? ADMIN_MAIN_ITEMS
        : STAFF_MAIN_ITEMS;
  const filteredMainItems = baseMain;

  // Pre-compute permission flags to reduce complexity in filter
  const userIsAdmin = isAdmin(session);
  const hasGroupSupervision = !isLoadingGroups && hasGroups;
  const hasRoomSupervision = !isLoadingSupervision && isSupervising;

  // Filter additional navigation items based on permissions
  const filteredAdditionalItems = additionalNavItems.filter((item) => {
    // Hide items marked as hideForAdmin for admin users
    if (item.hideForAdmin && userIsAdmin) {
      return false;
    }
    if (item.alwaysShow) return true;
    if (item.requiresAdmin) return userIsAdmin;
    if (item.requiresSupervision && !userIsAdmin) {
      return hasGroupSupervision || hasRoomSupervision;
    }
    if (item.requiresActiveSupervision && !userIsAdmin) {
      return hasRoomSupervision;
    }
    return true;
  });

  // Static navigation - 4 main items + overflow menu (operator mode: 2 items, no overflow)
  const displayMainItems: NavItem[] = filteredMainItems;
  const showOverflowMenu = mode !== "operator";
  // Avoid duplicates between main and additional
  const mainHrefs = new Set(displayMainItems.map((i) => i.href));
  const displayAdditionalItems =
    mode === "operator"
      ? []
      : filteredAdditionalItems.filter((i) => !mainHrefs.has(i.href));

  // Check if any additional nav item is active
  const isAnyAdditionalNavActive = displayAdditionalItems.some((item) =>
    isActiveRoute(item.href),
  );

  // Update sliding indicator position when route changes
  useEffect(() => {
    const updateIndicator = () => {
      // Find active nav item index
      const activeIndex = displayMainItems.findIndex((item) =>
        isActiveRoute(item.href),
      );

      if (activeIndex !== -1 && navRefs.current[activeIndex]) {
        const activeElement = navRefs.current[activeIndex];
        if (activeElement) {
          const { offsetLeft, offsetWidth } = activeElement;
          setIndicatorStyle({ left: offsetLeft, width: offsetWidth });
          setIndicatorVisible(true);
        }
      } else if (isAnyAdditionalNavActive && moreButtonRef.current) {
        // "Mehr" button is active
        const { offsetLeft, offsetWidth } = moreButtonRef.current;
        setIndicatorStyle({ left: offsetLeft, width: offsetWidth });
        setIndicatorVisible(true);
      } else {
        // No active item found - hide indicator
        setIndicatorVisible(false);
      }
    };

    // Small delay to ensure DOM is ready
    const timer = setTimeout(updateIndicator, 10);
    return () => clearTimeout(timer);
  }, [pathname, displayMainItems, isAnyAdditionalNavActive, isActiveRoute]);

  // Enable transitions after initial position is set and rendered
  useEffect(() => {
    const timer = setTimeout(() => {
      isInitialMount.current = false;
    }, INITIAL_MOUNT_DELAY_MS);
    return () => clearTimeout(timer);
  }, []);

  // Force indicator update on mount and when refs change
  useEffect(() => {
    const timer = setTimeout(() => {
      const activeIndex = displayMainItems.findIndex((item) =>
        isActiveRoute(item.href),
      );

      if (activeIndex !== -1 && navRefs.current[activeIndex]) {
        const activeElement = navRefs.current[activeIndex];
        if (activeElement) {
          const { offsetLeft, offsetWidth } = activeElement;
          setIndicatorStyle({ left: offsetLeft, width: offsetWidth });
          setIndicatorVisible(true);
        }
      }
    }, 50);

    return () => clearTimeout(timer);
  }, [displayMainItems, isActiveRoute]);

  return (
    <>
      {/* Spacer to prevent content from being hidden behind fixed nav */}
      <div className="h-16 lg:hidden" />

      {/* shadcn/UI Drawer - Full-width on mobile */}
      <Drawer open={isOverflowMenuOpen} onOpenChange={setIsOverflowMenuOpen}>
        <DrawerContent className="bg-white">
          <div className="w-full">
            {/* Hidden header for accessibility only */}
            <DrawerHeader className="sr-only">
              <DrawerTitle>Navigation</DrawerTitle>
              <DrawerDescription>Wähle eine Seite</DrawerDescription>
            </DrawerHeader>
            <div className="px-4 pt-6 pb-4">
              <div className="space-y-2">
                {displayAdditionalItems.map((item) => {
                  const isActive = isActiveRoute(item.href);

                  // Coming soon items are not clickable
                  if (item.comingSoon) {
                    return (
                      <div
                        key={item.label}
                        className="flex items-center gap-3 rounded-xl bg-gray-50 px-4 py-3 opacity-50"
                      >
                        <Icon
                          path={
                            navigationIcons[item.iconKey] ??
                            navigationIcons.home
                          }
                          className="h-5 w-5 text-gray-400"
                        />
                        <span className="flex-1 text-base font-medium text-gray-400">
                          {item.label}
                        </span>
                        <span className="rounded bg-gray-200 px-2 py-0.5 text-xs text-gray-500">
                          Bald verfügbar
                        </span>
                      </div>
                    );
                  }

                  return (
                    <Link
                      key={item.href}
                      href={item.href}
                      onClick={closeOverflowMenu}
                      className={`flex items-center gap-3 rounded-xl px-4 py-3 transition-all ${
                        isActive
                          ? "bg-gray-900 text-white"
                          : "bg-gray-50 text-gray-900 hover:bg-gray-100 active:bg-gray-200"
                      } `}
                    >
                      <Icon
                        path={
                          navigationIcons[item.iconKey] ?? navigationIcons.home
                        }
                        className={`h-5 w-5 ${isActive ? "text-white" : "text-gray-600"}`}
                      />
                      <span className="text-base font-medium">
                        {item.label}
                      </span>
                    </Link>
                  );
                })}
              </div>
            </div>
            <div className="pb-8" />
          </div>
        </DrawerContent>
      </Drawer>

      {/* Modern Pill-Style Bottom Navigation (shadcn-inspired) */}
      <nav
        className={`fixed right-0 bottom-0 left-0 z-30 translate-y-0 transition-transform duration-150 ease-in-out lg:hidden ${className} `}
      >
        {/* Pill container with margins */}
        <div className="px-4 pb-4">
          <div className="rounded-full border border-gray-200/50 bg-white/95 px-3 py-2 shadow-[0_-2px_20px_rgba(0,0,0,0.08)] backdrop-blur-md">
            <div className="relative flex items-center justify-around gap-1">
              {/* Sliding background indicator */}
              {indicatorVisible && (
                <div
                  className={`absolute top-0 h-full rounded-full bg-gray-900 shadow-md ${
                    isInitialMount.current
                      ? ""
                      : "transition-all duration-150 ease-out"
                  }`}
                  style={{
                    left: `${indicatorStyle.left}px`,
                    width: `${indicatorStyle.width}px`,
                  }}
                />
              )}

              {/* Main navigation items */}
              {displayMainItems.map((item, index) => {
                const isActive = isActiveRoute(item.href);

                return (
                  <Link
                    key={item.href}
                    href={item.href}
                    ref={(el) => {
                      navRefs.current[index] = el;
                    }}
                    className={`relative z-10 flex min-h-[44px] items-center justify-center gap-2.5 rounded-full px-3 py-2.5 transition-colors duration-200 ${
                      isActive
                        ? "bg-gray-900 text-white"
                        : "text-gray-400 hover:text-gray-600"
                    } `}
                  >
                    {/* Icon */}
                    <Icon
                      path={
                        navigationIcons[item.iconKey] ?? navigationIcons.home
                      }
                      className="h-5 w-5 flex-shrink-0"
                    />

                    {/* Label - ONLY show when active */}
                    {isActive && (
                      <span className="text-sm font-semibold whitespace-nowrap">
                        {item.label}
                      </span>
                    )}
                  </Link>
                );
              })}

              {/* More button */}
              {showOverflowMenu && (
                <button
                  ref={moreButtonRef}
                  onClick={() => setIsOverflowMenuOpen(true)}
                  className={`relative z-10 flex min-h-[44px] items-center justify-center gap-2.5 rounded-full px-3 py-2.5 transition-colors duration-200 ${
                    isOverflowMenuOpen || isAnyAdditionalNavActive
                      ? "bg-gray-900 text-white"
                      : "text-gray-400 hover:text-gray-600"
                  } `}
                >
                  {/* Icon */}
                  <Icon
                    path={navigationIcons.more}
                    className="h-5 w-5 flex-shrink-0"
                  />

                  {/* Label - ONLY show when active */}
                  {(isOverflowMenuOpen || isAnyAdditionalNavActive) && (
                    <span className="text-sm font-semibold whitespace-nowrap">
                      Mehr
                    </span>
                  )}
                </button>
              )}
            </div>
          </div>
        </div>

        {/* Safe area padding */}
        <div className="h-safe-area-inset-bottom bg-transparent" />
      </nav>
    </>
  );
}
