// components/dashboard/sidebar.tsx
"use client";

import { Suspense } from "react";
import Link from "next/link";
import { usePathname, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { useSupervision } from "~/lib/supervision-context";
import { isAdmin } from "~/lib/auth-utils";

// Type für Navigation Items
interface NavItem {
  href: string;
  label: string;
  icon: string;
  requiresAdmin?: boolean;
  requiresSupervision?: boolean;
  alwaysShow?: boolean;
  labelMultiple?: string;
}

// Navigation Items
const NAV_ITEMS: NavItem[] = [
  {
    href: "/dashboard",
    label: "Home",
    icon: "M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z",
    requiresAdmin: true,
  },
  {
    href: "/ogs-groups",
    label: "Meine Gruppe",
    icon: "M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z",
    alwaysShow: true, // Always show, empty state handled on page
    labelMultiple: "Meine Gruppen",
  },
  {
    href: "/active-supervisions",
    label: "Aktuelle Aufsicht",
    icon: "M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z",
    alwaysShow: true, // Always show, empty state handled on page
    labelMultiple: "Aktuelle Aufsichten",
  },
  {
    href: "/students/search",
    label: "Kindersuche",
    icon: "M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z",
    requiresSupervision: true,
  },
  {
    href: "/activities",
    label: "Aktivitäten",
    icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2",
    alwaysShow: true,
  },
  {
    href: "/rooms",
    label: "Räume",
    icon: "M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4",
    alwaysShow: true,
  },
  {
    href: "/staff",
    label: "Mitarbeiter",
    icon: "M10 6H5a2 2 0 00-2 2v9a2 2 0 002 2h14a2 2 0 002-2V8a2 2 0 00-2-2h-5m-4 0V5a2 2 0 114 0v1m-4 0a2 2 0 104 0m-5 8a2 2 0 100-4 2 2 0 000 4zm0 0c1.306 0 2.417.835 2.83 2M9 14a3.001 3.001 0 00-2.83 2M15 11h3m-3 4h2",
    alwaysShow: true,
  },
  // Temporarily disabled - not ready yet
  // {
  //     href: "/statistics",
  //     label: "Statistiken",
  //     icon: "M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z",
  //     requiresAdmin: true
  // },
  {
    href: "/substitutions",
    label: "Vertretungen",
    icon: "M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15",
    requiresAdmin: true,
    // TODO: In the future, this should check for specific substitution permissions
    // requiresPermission: 'substitutions:read'
  },
  {
    href: "/database",
    label: "Datenverwaltung",
    icon: "M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4",
    requiresAdmin: true,
  },
  {
    href: "/settings",
    label: "Einstellungen",
    icon: "M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065zM15 12a3 3 0 11-6 0 3 3 0 016 0z",
    alwaysShow: true,
  },
];

interface SidebarProps {
  readonly className?: string;
}

function SidebarContent({ className = "" }: SidebarProps) {
  // Aktuelle Route ermitteln
  const pathname = usePathname();
  const searchParams = useSearchParams();

  // Quick create activity modal state

  // Get session for role checking
  const { data: session } = useSession();

  // Get supervision state
  const { hasGroups, isSupervising, isLoadingGroups, isLoadingSupervision } =
    useSupervision();

  // Check if user has educational groups (keeping existing hook for compatibility)
  // const { hasEducationalGroups, isLoading } = useHasEducationalGroups();

  // Check if user has any supervision (groups or active room)
  const hasAnySupervision =
    (!isLoadingGroups && hasGroups) || (!isLoadingSupervision && isSupervising);

  // Filter navigation items based on permissions
  const baseFilteredNavItems = NAV_ITEMS.filter((item) => {
    // Always show items marked as alwaysShow
    if (item.alwaysShow) {
      return true;
    }

    // Check admin requirement
    if (item.requiresAdmin && !isAdmin(session)) {
      return false;
    }

    // Check supervision requirement (for student search - groups OR room supervision)
    if (item.requiresSupervision) {
      // Admins always see student search
      if (isAdmin(session)) return true;

      // Other users only see it if they have supervision
      const hasGroupSupervision = !isLoadingGroups && hasGroups;
      const hasRoomSupervision = !isLoadingSupervision && isSupervising;
      return hasGroupSupervision || hasRoomSupervision;
    }

    return true;
  });

  // If user has no supervision and is not admin, add student search at correct position
  let filteredNavItems = baseFilteredNavItems;
  if (!hasAnySupervision && !isAdmin(session)) {
    // Find the index of "Aktuelle Aufsicht" to insert Kindersuche right after it
    const meinRaumIndex = baseFilteredNavItems.findIndex(
      (item) => item.href === "/active-supervisions",
    );
    const insertIndex =
      meinRaumIndex >= 0 ? meinRaumIndex + 1 : baseFilteredNavItems.length;

    filteredNavItems = [
      ...baseFilteredNavItems.slice(0, insertIndex),
      {
        href: "/students/search",
        label: "Kindersuche",
        icon: "M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z",
        alwaysShow: true,
      },
      ...baseFilteredNavItems.slice(insertIndex),
    ];
  }

  // Helper to determine active href for student detail pages based on referrer
  const getStudentDetailActiveHref = (from: string | null): string => {
    if (!from) return "/students/search";
    if (from.startsWith("/ogs-groups")) return "/ogs-groups";
    if (from.startsWith("/active-supervisions")) return "/active-supervisions";
    if (from.startsWith("/students/search")) return "/students/search";
    return "/students/search";
  };

  // Check if a navigation link should be highlighted as active
  const isActiveLink = (href: string) => {
    // Special handling for student detail pages
    const isStudentDetailPage =
      pathname.startsWith("/students/") && pathname !== "/students/search";
    if (isStudentDetailPage) {
      const from = searchParams.get("from");
      return getStudentDetailActiveHref(from) === href;
    }

    // Exact match for Dashboard
    if (href === "/dashboard") return pathname === "/dashboard";

    // For other routes, check if current path starts with link path
    return pathname.startsWith(href);
  };

  const getLinkClasses = (href: string) => {
    const baseClasses =
      "flex items-center px-5 py-3 text-base font-medium rounded-lg transition-colors";
    const activeClasses = "bg-blue-50 text-blue-600 border-l-4 border-blue-600";
    const inactiveClasses =
      "text-gray-700 hover:bg-gray-100 hover:text-blue-600";

    return `${baseClasses} ${isActiveLink(href) ? activeClasses : inactiveClasses}`;
  };

  return (
    <>
      {/* Desktop sidebar */}
      <aside
        className={`min-h-screen w-64 border-r border-gray-200 bg-white ${className}`}
      >
        <div className="sticky top-[73px] p-4">
          <nav className="space-y-2">
            {filteredNavItems.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                className={getLinkClasses(item.href)}
              >
                <svg
                  className="mr-4 h-6 w-6"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d={item.icon}
                  />
                </svg>
                {item.label}
              </Link>
            ))}
          </nav>
        </div>
      </aside>
    </>
  );
}

export function Sidebar({ className = "" }: SidebarProps) {
  return (
    <Suspense
      fallback={
        <aside
          className={`min-h-screen w-64 border-r border-gray-200 bg-white ${className}`}
        >
          <div className="sticky top-[73px] p-4">
            <nav className="space-y-2">
              {/* Skeleton placeholders matching nav item height */}
              <div className="flex items-center px-5 py-3">
                <div className="mr-4 h-6 w-6 animate-pulse rounded bg-gray-200" />
                <div className="h-4 w-24 animate-pulse rounded bg-gray-200" />
              </div>
              <div className="flex items-center px-5 py-3">
                <div className="mr-4 h-6 w-6 animate-pulse rounded bg-gray-200" />
                <div className="h-4 w-28 animate-pulse rounded bg-gray-200" />
              </div>
              <div className="flex items-center px-5 py-3">
                <div className="mr-4 h-6 w-6 animate-pulse rounded bg-gray-200" />
                <div className="h-4 w-20 animate-pulse rounded bg-gray-200" />
              </div>
              <div className="flex items-center px-5 py-3">
                <div className="mr-4 h-6 w-6 animate-pulse rounded bg-gray-200" />
                <div className="h-4 w-24 animate-pulse rounded bg-gray-200" />
              </div>
            </nav>
          </div>
        </aside>
      }
    >
      <SidebarContent className={className} />
    </Suspense>
  );
}
