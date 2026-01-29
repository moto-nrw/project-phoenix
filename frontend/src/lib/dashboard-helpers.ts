// Dashboard Analytics Types
export interface DashboardAnalytics {
  // Student Overview
  studentsPresent: number;
  studentsInTransit: number; // Students present but not in any active visit
  studentsOnPlayground: number;
  studentsInRooms: number; // Students in indoor rooms (excluding playground)

  // Activities & Rooms
  activeActivities: number;
  freeRooms: number;
  totalRooms: number;
  capacityUtilization: number;
  activityCategories: number;

  // OGS Groups
  activeOGSGroups: number;
  studentsInGroupRooms: number;
  supervisorsToday: number;
  studentsInHomeRoom: number;

  // Recent Activity (Privacy-compliant)
  recentActivity: RecentActivity[];

  // Current Activities (No personal data)
  currentActivities: CurrentActivity[];

  // Active Groups Summary
  activeGroupsSummary: ActiveGroupInfo[];

  // Timestamp
  lastUpdated: Date;
}

export interface RecentActivity {
  type: "check_in" | "check_out" | "group_start" | "group_end";
  groupName: string;
  roomName: string;
  count: number;
  timestamp: Date;
}

export interface CurrentActivity {
  name: string;
  category: string;
  participants: number;
  maxCapacity: number;
  status: "active" | "full" | "ending_soon";
}

export interface ActiveGroupInfo {
  name: string;
  type: "ogs_group" | "activity";
  studentCount: number;
  location: string;
  status: "active" | "transitioning" | "preparing";
}

// Backend response types
export interface DashboardAnalyticsResponse {
  students_present: number;
  students_in_transit: number; // Students present but not in any active visit
  students_on_playground: number;
  students_in_rooms: number; // Students in indoor rooms (excluding playground)
  active_activities: number;
  free_rooms: number;
  total_rooms: number;
  capacity_utilization: number;
  activity_categories: number;
  active_ogs_groups: number;
  students_in_group_rooms: number;
  supervisors_today: number;
  students_in_home_room: number;
  recent_activity: Array<{
    type: string;
    group_name: string;
    room_name: string;
    count: number;
    timestamp: string;
  }>;
  current_activities: Array<{
    name: string;
    category: string;
    participants: number;
    max_capacity: number;
    status: string;
  }>;
  active_groups_summary: Array<{
    name: string;
    type: string;
    student_count: number;
    location: string;
    status: string;
  }>;
  last_updated: string;
}

// Mapping function
export function mapDashboardAnalyticsResponse(
  data: DashboardAnalyticsResponse,
): DashboardAnalytics {
  return {
    studentsPresent: data.students_present,
    studentsInTransit: data.students_in_transit,
    studentsOnPlayground: data.students_on_playground,
    studentsInRooms: data.students_in_rooms,
    activeActivities: data.active_activities,
    freeRooms: data.free_rooms,
    totalRooms: data.total_rooms,
    capacityUtilization: data.capacity_utilization,
    activityCategories: data.activity_categories,
    activeOGSGroups: data.active_ogs_groups,
    studentsInGroupRooms: data.students_in_group_rooms,
    supervisorsToday: data.supervisors_today,
    studentsInHomeRoom: data.students_in_home_room,
    recentActivity: data.recent_activity.map((activity) => ({
      type: activity.type as RecentActivity["type"],
      groupName: activity.group_name,
      roomName: activity.room_name,
      count: activity.count,
      timestamp: new Date(activity.timestamp),
    })),
    currentActivities: data.current_activities.map((activity) => ({
      name: activity.name,
      category: activity.category,
      participants: activity.participants,
      maxCapacity: activity.max_capacity,
      status: activity.status as CurrentActivity["status"],
    })),
    activeGroupsSummary: data.active_groups_summary.map((group) => ({
      name: group.name,
      type: group.type as ActiveGroupInfo["type"],
      studentCount: group.student_count,
      location: group.location,
      status: group.status as ActiveGroupInfo["status"],
    })),
    lastUpdated: new Date(data.last_updated),
  };
}

// Helper functions for formatting
export function formatRecentActivityTime(timestamp: Date | string): string {
  const now = new Date();
  const timestampDate =
    typeof timestamp === "string" ? new Date(timestamp) : timestamp;

  // Check if the date is valid
  if (Number.isNaN(timestampDate.getTime())) {
    return "Unbekannt";
  }

  const diffMinutes = Math.floor(
    (now.getTime() - timestampDate.getTime()) / 60000,
  );

  if (diffMinutes < 1) return "gerade eben";
  if (diffMinutes < 60) return `vor ${diffMinutes} min`;

  const diffHours = Math.floor(diffMinutes / 60);
  if (diffHours < 24) return `vor ${diffHours} Std.`;

  return timestampDate.toLocaleDateString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

export function getActivityTypeIcon(type: RecentActivity["type"]): string {
  switch (type) {
    case "check_in":
      return "➡️";
    case "check_out":
      return "⬅️";
    case "group_start":
      return "▶️";
    case "group_end":
      return "⏹️";
    default:
      return "📍";
  }
}

export function getActivityStatusColor(
  status: CurrentActivity["status"],
): string {
  switch (status) {
    case "active":
      return "bg-green-500";
    case "full":
      return "bg-amber-500";
    case "ending_soon":
      return "bg-orange-500";
    default:
      return "bg-gray-500";
  }
}

export function getGroupStatusColor(status: ActiveGroupInfo["status"]): string {
  switch (status) {
    case "active":
      return "bg-green-500";
    case "transitioning":
      return "bg-amber-500";
    case "preparing":
      return "bg-blue-500";
    default:
      return "bg-gray-500";
  }
}
