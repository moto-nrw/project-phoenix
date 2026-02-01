// Helper functions for staff data transformation and status determination

import type { Staff } from "./staff-api";

// Location status type matching the pattern from OGS groups
export interface LocationStatus {
  label: string;
  badgeColor: string;
  cardGradient: string;
  customBgColor: string;
  customShadow: string;
}

// Get location status for a staff member based on their supervision status
export function getStaffLocationStatus(staff: Staff): LocationStatus {
  const location = staff.currentLocation ?? "Zuhause";

  // Match the location status pattern from ogs_groups
  if (location === "Zuhause") {
    return {
      label: "Zuhause",
      badgeColor: "text-white backdrop-blur-sm",
      cardGradient: "from-red-50/80 to-rose-100/80",
      customBgColor: "#FF3130",
      customShadow: "0 8px 25px rgba(255, 49, 48, 0.4)",
    };
  } else if (location === "Anwesend") {
    // Staff who was present today but not currently supervising
    // Uses same green as student "Anwesend" badge (LOCATION_COLORS.GROUP_ROOM from location-helper.ts)
    return {
      label: "Anwesend",
      badgeColor: "text-white backdrop-blur-sm",
      cardGradient: "from-green-50/80 to-emerald-100/80",
      customBgColor: "#83CD2D",
      customShadow: "0 8px 25px rgba(131, 205, 45, 0.4)",
    };
  } else if (location === "Schulhof") {
    return {
      label: "Schulhof",
      badgeColor: "text-white backdrop-blur-sm",
      cardGradient: "from-amber-50/80 to-yellow-100/80",
      customBgColor: "#F78C10",
      customShadow: "0 8px 25px rgba(247, 140, 16, 0.4)",
    };
  } else if (location === "Unterwegs") {
    return {
      label: "Unterwegs",
      badgeColor: "text-white backdrop-blur-sm",
      cardGradient: "from-fuchsia-50/80 to-pink-100/80",
      customBgColor: "#D946EF",
      customShadow: "0 8px 25px rgba(217, 70, 239, 0.4)",
    };
  } else if (location === "Homeoffice") {
    return {
      label: "Homeoffice",
      badgeColor: "text-white backdrop-blur-sm",
      cardGradient: "from-sky-50/80 to-sky-100/80",
      customBgColor: "#0EA5E9",
      customShadow: "0 8px 25px rgba(14, 165, 233, 0.4)",
    };
  } else if (
    location === "Krank" ||
    location === "Urlaub" ||
    location === "Fortbildung" ||
    location === "Abwesend"
  ) {
    // All absence types use a unified gray badge (text label differentiates)
    return {
      label: location,
      badgeColor: "text-white backdrop-blur-sm",
      cardGradient: "from-gray-50/80 to-slate-100/80",
      customBgColor: "#6B7280",
      customShadow: "0 8px 25px rgba(107, 114, 128, 0.4)",
    };
  } else {
    // Specific room - use blue/cyan color
    return {
      label: location,
      badgeColor: "text-white backdrop-blur-sm",
      cardGradient: "from-blue-50/80 to-cyan-100/80",
      customBgColor: "#5080D8",
      customShadow: "0 8px 25px rgba(80, 128, 216, 0.4)",
    };
  }
}

// Get a display-friendly role/type for staff
export function getStaffDisplayType(staff: Staff): string {
  // Check role first (Admin/Betreuer/Extern)
  if (staff.role) {
    return staff.role;
  }
  // Fall back to specialization if available
  if (staff.isTeacher && staff.specialization) {
    return staff.specialization;
  }
  return "Betreuer";
}

// Get additional info to display on card
export function getStaffCardInfo(staff: Staff): string[] {
  const info: string[] = [];

  // Add qualifications if available
  if (staff.qualifications) {
    info.push(staff.qualifications);
  }

  // Add supervision role if currently supervising
  if (staff.isSupervising && staff.supervisionRole) {
    if (staff.supervisionRole === "primary") {
      info.push("Hauptbetreuer");
    } else if (staff.supervisionRole === "assistant") {
      info.push("Assistenz");
    }
  }

  return info;
}

// Format staff notes for display (truncate if needed)
export function formatStaffNotes(
  notes?: string,
  maxLength = 100,
): string | undefined {
  if (!notes || notes.trim().length === 0) {
    return undefined;
  }

  const trimmed = notes.trim();
  if (trimmed.length <= maxLength) {
    return trimmed;
  }

  return trimmed.substring(0, maxLength - 3) + "...";
}

// Sort staff by supervision status and name
export function sortStaff(staff: Staff[]): Staff[] {
  return [...staff].sort((a, b) => {
    // First sort by supervision status (supervising staff first)
    if (a.isSupervising && !b.isSupervising) return -1;
    if (!a.isSupervising && b.isSupervising) return 1;

    // Then sort alphabetically by last name
    return a.lastName.localeCompare(b.lastName, "de");
  });
}
