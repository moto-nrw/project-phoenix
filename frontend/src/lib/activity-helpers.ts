import type { Activity, ActivityCategory, ActivityTime } from './activity-api';

/**
 * Maps a single activity response from the backend to the frontend model
 */
export function mapSingleActivityResponse(data: any): Activity {
  if (!data) {
    throw new Error('Invalid activity data received from API');
  }

  // Create a formatted activity object
  const activity: Activity = {
    id: data.id.toString(),
    name: data.name || '',
    max_participant: data.max_participant || 0,
    is_open_ags: data.is_open_ags || false, // Use is_open_ags directly from the backend
    supervisor_id: data.supervisor_id ? data.supervisor_id.toString() : '',
    ag_category_id: data.ag_category_id ? data.ag_category_id.toString() : '',
    created_at: data.created_at || '',
    updated_at: data.updated_at || data.modified_at || '',
  };

  // Add optional fields if present
  if (data.datespan_id) {
    activity.datespan_id = data.datespan_id.toString();
  }

  // Add supervisor name if available
  if (data.supervisor && data.supervisor.custom_user) {
    const supervisor = data.supervisor;
    const customUser = supervisor.custom_user;
    const firstName = customUser.first_name || '';
    const secondName = customUser.second_name || '';
    activity.supervisor_name = `${firstName} ${secondName}`.trim();
    
    // If both names are empty, don't set a supervisor name so the UI shows "Nicht zugewiesen"
    if (!firstName && !secondName) {
      activity.supervisor_name = '';
    }
  }

  // Add category name if available
  if (data.ag_category) {
    activity.category_name = data.ag_category.name || '';
  }

  // Map time slots if available
  if (data.times && Array.isArray(data.times)) {
    activity.times = data.times.map((time: any) => mapTimeSlot(time));
  }

  // Map students if available
  if (data.students && Array.isArray(data.students)) {
    activity.students = data.students.map((student: any) => ({
      id: student.id.toString(),
      name: student.custom_user ? `${student.custom_user.first_name || ''} ${student.custom_user.second_name || ''}`.trim() : '',
      first_name: student.custom_user ? student.custom_user.first_name : '',
      second_name: student.custom_user ? student.custom_user.second_name : '',
      school_class: student.school_class || '',
      in_house: student.in_house || false,
      group_id: student.group_id ? student.group_id.toString() : '',
      group_name: student.group ? student.group.name : '',
      custom_users_id: student.custom_users_id ? student.custom_users_id.toString() : '',
    }));
  }

  // Calculate participant count and available spots
  if (activity.students) {
    activity.participant_count = activity.students.length;
    activity.available_spots = Math.max(0, activity.max_participant - activity.participant_count);
  } else {
    activity.participant_count = 0;
    activity.available_spots = activity.max_participant;
  }

  return activity;
}

/**
 * Maps an array of activity responses from the backend to frontend models
 */
export function mapActivityResponse(data: any): Activity[] {
  if (!data || !Array.isArray(data)) {
    return [];
  }

  return data.map(mapSingleActivityResponse);
}

/**
 * Maps a time slot from the backend to the frontend model
 */
function mapTimeSlot(data: any): ActivityTime {
  if (!data) {
    throw new Error('Invalid time slot data');
  }

  const timeSlot: ActivityTime = {
    id: data.id.toString(),
    weekday: data.weekday || '',
    timespan_id: data.timespan_id ? data.timespan_id.toString() : '',
    ag_id: data.ag_id ? data.ag_id.toString() : '',
    created_at: data.created_at || '',
  };

  // Add timespan details if available
  if (data.timespan) {
    timeSlot.timespan = {
      start_time: data.timespan.start_time || '',
    };

    if (data.timespan.end_time) {
      timeSlot.timespan.end_time = data.timespan.end_time;
    }
  }

  return timeSlot;
}

/**
 * Maps an array of activity category responses
 */
export function mapCategoryResponse(data: any): ActivityCategory[] {
  if (!data || !Array.isArray(data)) {
    return [];
  }

  return data.map((category) => ({
    id: category.id.toString(),
    name: category.name || '',
    created_at: category.created_at || '',
  }));
}

/**
 * Format activity times into a readable string
 */
export function formatActivityTimes(activity: Activity): string {
  if (!activity.times || activity.times.length === 0) {
    return 'No scheduled times';
  }

  return activity.times.map((time) => {
    let timeStr = time.weekday;
    
    if (time.timespan) {
      timeStr += ` ${time.timespan.start_time}`;
      if (time.timespan.end_time) {
        timeStr += `-${time.timespan.end_time}`;
      }
    }
    
    return timeStr;
  }).join(', ');
}

/**
 * Prepare activity data for backend submission
 */
export function prepareActivityForBackend(activity: Partial<Activity>): any {
  // The Go struct now consistently uses is_open_ags for the field
  // Debug what we're getting from the frontend
  console.log('Raw activity data for backend conversion:', JSON.stringify(activity, null, 2));
  
  const backendActivity: any = {
    name: activity.name,
    max_participant: activity.max_participant,
    is_open_ags: activity.is_open_ags,
    // Add these fields to ensure they're always included, even if undefined
    ag_category_id: undefined,  // Use the correct JSON field name matching Go struct tag
    supervisor_id: undefined
  };

  // Add IDs if present, ensuring they're converted to numbers
  if (activity.id) {
    backendActivity.id = parseInt(activity.id, 10);
  }
  
  // Always include supervisor_id - either from input or as 0
  backendActivity.supervisor_id = activity.supervisor_id ? parseInt(activity.supervisor_id, 10) : 0;
  
  // Map frontend's ag_category_id to the backend's JSON field ag_category_id (not ag_categories_id)
  backendActivity.ag_category_id = activity.ag_category_id ? parseInt(activity.ag_category_id, 10) : 0;
  
  if (activity.datespan_id) {
    backendActivity.datespan_id = parseInt(activity.datespan_id, 10);
  }

  // Handle time slots and student IDs for creation
  if (activity.times && activity.times.length > 0) {
    backendActivity.timeslots = activity.times.map(time => ({
      weekday: time.weekday,
      timespan_id: parseInt(time.timespan_id, 10),
    }));
  }

  if (activity.students && activity.students.length > 0) {
    backendActivity.student_ids = activity.students.map(student => 
      parseInt(student.id, 10)
    );
  }

  return backendActivity;
}

/**
 * Helper function to check if an activity is full
 */
export function isActivityFull(activity: Activity): boolean {
  if (!activity.students) {
    return false;
  }
  return activity.students.length >= activity.max_participant;
}

/**
 * Helper function to check if a student is enrolled in an activity
 */
export function isStudentEnrolled(activity: Activity, studentId: string): boolean {
  if (!activity.students) {
    return false;
  }
  return activity.students.some(student => student.id === studentId);
}

/**
 * Format participant status with count and maximum
 */
export function formatParticipantStatus(activity: Activity): string {
  const count = activity.participant_count || 0;
  return `${count}/${activity.max_participant}`;
}