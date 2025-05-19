import {
  fetchActivities,
  createActivity,
  updateActivity,
  deleteActivity,
  getActivity,
  getCategories,
  getSupervisors
} from './activity-api';
import type { Activity, CreateActivityRequest, UpdateActivityRequest, ActivityFilter, ActivityCategory, ActivityStudent } from './activity-helpers';

class ActivityService {
  async getActivities(filters?: ActivityFilter): Promise<Activity[]> {
    return fetchActivities(filters);
  }

  async getActivity(id: string): Promise<Activity> {
    return getActivity(id);
  }

  async createActivity(data: CreateActivityRequest): Promise<Activity> {
    return createActivity(data);
  }

  async updateActivity(id: string, data: UpdateActivityRequest): Promise<Activity> {
    return updateActivity(id, data);
  }

  async deleteActivity(id: string): Promise<void> {
    return deleteActivity(id);
  }

  async getCategories(): Promise<ActivityCategory[]> {
    return getCategories();
  }

  async getSupervisors(): Promise<Array<{ id: string; name: string }>> {
    return getSupervisors();
  }

  // Student enrollment methods (stubs for now)
  async getEnrolledStudents(_activityId: string): Promise<ActivityStudent[]> {
    // TODO: Implement when backend endpoints are available
    return [];
  }

  async enrollStudent(_activityId: string, _studentData: { studentId: string; }): Promise<{ success: boolean }> {
    // TODO: Implement when backend endpoints are available
    return { success: true };
  }

  async unenrollStudent(_activityId: string, _studentId: string): Promise<void> {
    // TODO: Implement when backend endpoints are available
  }

  // Time management methods (stubs for now)
  async addTimeSlot(_activityId: string, _timeData: { weekday: string; startTime: string; endTime: string }): Promise<{ success: boolean }> {
    // TODO: Implement when backend endpoints are available
    return { success: true };
  }

  async deleteTimeSlot(_activityId: string, _timeId: string): Promise<void> {
    // TODO: Implement when backend endpoints are available
  }
}

export const activityService = new ActivityService();