import {
  fetchActivities,
  getActivitySchedules,
  getActivitySchedule,
  getTimeframes,
} from "./activity-api";
import type {
  Activity,
  ActivityFilter,
  ActivitySchedule,
  Timeframe,
} from "./activity-helpers";

class ActivityService {
  async getActivities(filters?: ActivityFilter): Promise<Activity[]> {
    return fetchActivities(filters);
  }

  // Schedule Management methods
  async getActivitySchedules(activityId: string): Promise<ActivitySchedule[]> {
    return getActivitySchedules(activityId);
  }

  async getActivitySchedule(
    activityId: string,
    scheduleId: string,
  ): Promise<ActivitySchedule | null> {
    return getActivitySchedule(activityId, scheduleId);
  }

  async getTimeframes(): Promise<Timeframe[]> {
    return getTimeframes();
  }
}

export const activityService = new ActivityService();
