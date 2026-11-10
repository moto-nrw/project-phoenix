import {
  fetchActivities,
  getActivitySchedules,
  getActivitySchedule,
  getAvailableTimeSlots,
  createActivitySchedule,
  updateActivitySchedule,
  deleteActivitySchedule,
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

  async getAvailableTimeSlots(
    activityId: string,
    date?: string,
  ): Promise<Array<{ weekday: string; timeframe_id?: string }>> {
    return getAvailableTimeSlots(activityId, date);
  }

  async getTimeframes(): Promise<Timeframe[]> {
    return getTimeframes();
  }

  async createActivitySchedule(
    activityId: string,
    scheduleData: Partial<ActivitySchedule>,
  ): Promise<ActivitySchedule | null> {
    return createActivitySchedule(activityId, scheduleData);
  }

  async updateActivitySchedule(
    activityId: string,
    scheduleId: string,
    scheduleData: Partial<ActivitySchedule>,
  ): Promise<ActivitySchedule | null> {
    return updateActivitySchedule(activityId, scheduleId, scheduleData);
  }

  async deleteActivitySchedule(
    activityId: string,
    scheduleId: string,
  ): Promise<boolean> {
    return deleteActivitySchedule(activityId, scheduleId);
  }

  // Alias methods for compatibility with the times/page.tsx
  async deleteTimeSlot(activityId: string, timeId: string): Promise<boolean> {
    return this.deleteActivitySchedule(activityId, timeId);
  }

  async addTimeSlot(
    activityId: string,
    timeData: { weekday: string; startTime: string; endTime: string },
  ): Promise<ActivitySchedule | null> {
    // Format the data as expected by createActivitySchedule
    const scheduleData: Partial<ActivitySchedule> = {
      activity_id: activityId,
      weekday: timeData.weekday.toLowerCase(),
      // You might need to transform timeData.startTime/endTime to a timeframe_id if your API requires it
    };

    return this.createActivitySchedule(activityId, scheduleData);
  }
}

export const activityService = new ActivityService();
