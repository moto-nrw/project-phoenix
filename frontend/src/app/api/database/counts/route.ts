import { createGetHandler } from '~/lib/route-wrapper';
import { apiGet } from '~/lib/api-client';

// Database stats response type matching backend
interface DatabaseStats {
  students: number;
  teachers: number;
  rooms: number;
  activities: number;
  groups: number;
  permissions: {
    canViewStudents: boolean;
    canViewTeachers: boolean;
    canViewRooms: boolean;
    canViewActivities: boolean;
    canViewGroups: boolean;
  };
}

export const GET = createGetHandler(async (_request, token) => {
  const response = await apiGet<DatabaseStats>(`/api/database/stats`, token);
  return response.data;
});