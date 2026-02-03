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

// Badge representation for a single supervision or status
export interface SupervisionBadge {
  key: string;
  label: string;
  locationStatus: LocationStatus;
}

/**
 * Returns an array of badges for a staff member's supervisions.
 * - Non-supervising staff: single badge (Zuhause/Anwesend)
 * - Supervising staff: one badge per room with appropriate color
 */
export function getStaffSupervisionBadges(staff: Staff): SupervisionBadge[] {
  // Non-supervising staff - single badge
  const supervisions = staff.supervisions ?? [];
  if (!staff.isSupervising || supervisions.length === 0) {
    const location = staff.currentLocation ?? "Zuhause";
    return [
      {
        key: "status",
        label: location,
        locationStatus: getStaffLocationStatus(staff),
      },
    ];
  }

  // Supervising staff - one badge per room
  return supervisions.map((supervision) => {
    // Create a locationStatus for this specific room
    const roomName = supervision.roomName;
    let locationStatus: LocationStatus;

    if (roomName === "Schulhof") {
      locationStatus = {
        label: "Schulhof",
        badgeColor: "text-white backdrop-blur-sm",
        cardGradient: "from-amber-50/80 to-yellow-100/80",
        customBgColor: "#F78C10",
        customShadow: "0 8px 25px rgba(247, 140, 16, 0.4)",
      };
    } else {
      // Default room color (blue)
      locationStatus = {
        label: roomName,
        badgeColor: "text-white backdrop-blur-sm",
        cardGradient: "from-blue-50/80 to-cyan-100/80",
        customBgColor: "#5080D8",
        customShadow: "0 8px 25px rgba(80, 128, 216, 0.4)",
      };
    }

    return {
      key: `room-${supervision.roomId}`,
      label: roomName,
      locationStatus,
    };
  });
}
