import {
  fetchActivities,
  createActivity,
  updateActivity,
  deleteActivity,
  getActivity,
  getCategories,
  getSupervisors,
  getEnrolledStudents,
  enrollStudent,
  unenrollStudent,
  getActivitySchedules,
  getActivitySchedule,
  getActivitySupervisors,
  getAvailableSupervisors,
  assignSupervisor,
  updateSupervisorRole,
  removeSupervisor,
  getAvailableStudents,
  updateGroupEnrollments,
  getTimeframes,
} from "./activity-api";
import type {
  Activity,
  CreateActivityRequest,
  UpdateActivityRequest,
  ActivityFilter,
  ActivityCategory,
  ActivityStudent,
  ActivitySchedule,
  Timeframe,
} from "./activity-helpers";

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

  async updateActivity(
    id: string,
    data: UpdateActivityRequest,
  ): Promise<Activity> {
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

  // Student enrollment methods
  async getEnrolledStudents(activityId: string): Promise<ActivityStudent[]> {
    return getEnrolledStudents(activityId);
  }

  async getAvailableStudents(
    activityId: string,
    filters?: { search?: string; group_id?: string },
  ): Promise<Array<{ id: string; name: string; school_class: string }>> {
    return getAvailableStudents(activityId, filters);
  }

  async enrollStudent(
    activityId: string,
    studentData: { studentId: string },
  ): Promise<{ success: boolean }> {
    return enrollStudent(activityId, studentData);
  }

  async unenrollStudent(activityId: string, studentId: string): Promise<void> {
    return unenrollStudent(activityId, studentId);
  }

  async updateGroupEnrollments(
    activityId: string,
    data: { student_ids: string[] },
  ): Promise<boolean> {
    return updateGroupEnrollments(activityId, data);
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

  // Supervisor Assignment methods
  async getActivitySupervisors(
    activityId: string,
  ): Promise<
    Array<{ id: string; staff_id: string; is_primary: boolean; name: string }>
  > {
    return getActivitySupervisors(activityId);
  }

  async getAvailableSupervisors(
    activityId: string,
  ): Promise<Array<{ id: string; name: string }>> {
    return getAvailableSupervisors(activityId);
  }

  async assignSupervisor(
    activityId: string,
    supervisorData: { staff_id: string; is_primary?: boolean },
  ): Promise<boolean> {
    return assignSupervisor(activityId, supervisorData);
  }

  async updateSupervisorRole(
    activityId: string,
    supervisorId: string,
    roleData: { is_primary: boolean },
  ): Promise<boolean> {
    return updateSupervisorRole(activityId, supervisorId, roleData);
  }

  async removeSupervisor(
    activityId: string,
    supervisorId: string,
  ): Promise<boolean> {
    return removeSupervisor(activityId, supervisorId);
  }
}

export const activityService = new ActivityService();
