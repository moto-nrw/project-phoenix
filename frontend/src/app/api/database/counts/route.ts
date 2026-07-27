import { proxyGet } from "~/lib/route-proxy.server";

// Database stats response type matching backend
interface DatabaseStats {
  students: number;
  teachers: number;
  rooms: number;
  activities: number;
  groups: number;
  roles: number;
  devices: number;
  permissionCount: number;
  permissions: {
    canViewStudents: boolean;
    canViewTeachers: boolean;
    canViewRooms: boolean;
    canViewActivities: boolean;
    canViewGroups: boolean;
    canViewRoles: boolean;
    canViewDevices: boolean;
    canViewPermissions: boolean;
    canViewTimetables: boolean;
    canViewGradeTransitions: boolean;
  };
}

export const GET = proxyGet<DatabaseStats>(`/api/database/stats`, {
  raw: true,
});
