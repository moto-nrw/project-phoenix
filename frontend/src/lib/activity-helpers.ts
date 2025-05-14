// lib/activity-helpers.ts
// Type definitions and helper functions for activities

// Backend types (from Go structs)
export interface BackendActivity {
    id: number;
    name: string;
    max_participant: number;
    is_open_ags: boolean;
    supervisor_id: number;
    supervisor_name?: string;
    ag_category_id: number;
    category_name?: string;
    created_at: string;
    updated_at: string;
    participant_count?: number;
    times?: BackendActivityTime[];
    students?: BackendActivityStudent[];
}

export interface BackendActivityCategory {
    id: number;
    name: string;
    description?: string;
    created_at: string;
    updated_at: string;
}

export interface BackendActivityTime {
    id: number;
    activity_id: number;
    weekday: string; // e.g., "monday", "tuesday"
    timespan: {
        start_time: string; // HH:MM format
        end_time: string;   // HH:MM format
    };
    created_at: string;
    updated_at: string;
}

export interface BackendActivityStudent {
    id: number;
    activity_id: number;
    student_id: number;
    name?: string;
    school_class?: string;
    in_house?: boolean;
    created_at: string;
    updated_at: string;
}

// Frontend types
export interface Activity {
    id: string;
    name: string;
    max_participant: number;
    is_open_ags: boolean;
    supervisor_id: string;
    supervisor_name?: string;
    ag_category_id: string;
    category_name?: string;
    created_at: Date;
    updated_at: Date;
    participant_count?: number;
    times?: ActivityTime[];
    students?: ActivityStudent[];
}

export interface ActivityCategory {
    id: string;
    name: string;
    description?: string;
    created_at: Date;
    updated_at: Date;
}

export interface ActivityTime {
    id: string;
    activity_id: string;
    weekday: string;
    timespan: {
        start_time: string;
        end_time: string;
    };
    created_at: Date;
    updated_at: Date;
}

export interface ActivityStudent {
    id: string;
    activity_id: string;
    student_id: string;
    name?: string;
    school_class?: string;
    in_house: boolean;
    created_at: Date;
    updated_at: Date;
}

// Mapping functions for backend to frontend types
export function mapActivityResponse(backendActivity: BackendActivity): Activity {
    return {
        id: String(backendActivity.id),
        name: backendActivity.name,
        max_participant: backendActivity.max_participant,
        is_open_ags: backendActivity.is_open_ags,
        supervisor_id: String(backendActivity.supervisor_id),
        supervisor_name: backendActivity.supervisor_name,
        ag_category_id: String(backendActivity.ag_category_id),
        category_name: backendActivity.category_name,
        created_at: new Date(backendActivity.created_at),
        updated_at: new Date(backendActivity.updated_at),
        participant_count: backendActivity.participant_count,
        times: backendActivity.times?.map(mapActivityTimeResponse),
        students: backendActivity.students?.map(mapActivityStudentResponse),
    };
}

export function mapActivityCategoryResponse(backendCategory: BackendActivityCategory): ActivityCategory {
    return {
        id: String(backendCategory.id),
        name: backendCategory.name,
        description: backendCategory.description,
        created_at: new Date(backendCategory.created_at),
        updated_at: new Date(backendCategory.updated_at),
    };
}

export function mapActivityTimeResponse(backendTime: BackendActivityTime): ActivityTime {
    return {
        id: String(backendTime.id),
        activity_id: String(backendTime.activity_id),
        weekday: backendTime.weekday,
        timespan: backendTime.timespan,
        created_at: new Date(backendTime.created_at),
        updated_at: new Date(backendTime.updated_at),
    };
}

export function mapActivityStudentResponse(backendStudent: BackendActivityStudent): ActivityStudent {
    return {
        id: String(backendStudent.id),
        activity_id: String(backendStudent.activity_id),
        student_id: String(backendStudent.student_id),
        name: backendStudent.name,
        school_class: backendStudent.school_class,
        in_house: backendStudent.in_house ?? false, // Default to false if not present
        created_at: new Date(backendStudent.created_at),
        updated_at: new Date(backendStudent.updated_at),
    };
}

// Prepare frontend types for backend requests
export function prepareActivityForBackend(activity: Partial<Activity>): Partial<BackendActivity> {
    // Create the basic activity object without times
    const result: Partial<BackendActivity> = {
        id: activity.id ? parseInt(activity.id, 10) : undefined,
        name: activity.name,
        max_participant: activity.max_participant,
        is_open_ags: activity.is_open_ags,
        supervisor_id: activity.supervisor_id ? parseInt(activity.supervisor_id, 10) : undefined,
        ag_category_id: activity.ag_category_id ? parseInt(activity.ag_category_id, 10) : undefined,
    };
    
    // Add times property only if activity.times exists
    if (activity.times?.length) {
        // Cast the array to BackendActivityTime[] to satisfy the type requirement
        // This is safe because we're ensuring all required fields are present in prepareActivityTimeForBackend
        result.times = activity.times.map(time => {
            const backendTime = prepareActivityTimeForBackend(time);
            // Ensure all required fields are present for BackendActivityTime
            return {
                id: backendTime.id ?? 0,
                activity_id: backendTime.activity_id ?? 0,
                weekday: backendTime.weekday ?? '',
                timespan: backendTime.timespan ?? { start_time: '', end_time: '' },
                created_at: '',
                updated_at: ''
            } as BackendActivityTime;
        });
    }
    
    return result;
}

export function prepareActivityTimeForBackend(time: Partial<ActivityTime>): Partial<BackendActivityTime> {
    return {
        id: time.id ? parseInt(time.id, 10) : undefined,
        activity_id: time.activity_id ? parseInt(time.activity_id, 10) : undefined,
        weekday: time.weekday,
        timespan: time.timespan,
    };
}

// Request/Response types
export interface CreateActivityRequest {
    name: string;
    max_participant: number;
    is_open_ags: boolean;
    supervisor_id: number;
    ag_category_id: number;
    times?: {
        weekday: string;
        timespan: {
            start_time: string;
            end_time: string;
        };
    }[];
}

export interface UpdateActivityRequest {
    name: string;
    max_participant: number;
    is_open_ags: boolean;
    supervisor_id: number;
    ag_category_id: number;
    times?: {
        id?: number;
        weekday: string;
        timespan: {
            start_time: string;
            end_time: string;
        };
    }[];
}

// Helper functions 
export function formatActivityTimes(activity: Activity | ActivityTime[]): string {
    // Handle case when activity is an Activity object
    if ('times' in activity && Array.isArray(activity.times)) {
        const times = activity.times;
        if (!times || times.length === 0) return "Keine Zeiten festgelegt";
        
        return times.map(time => {
            const weekday = formatWeekday(time.weekday);
            const timeRange = `${time.timespan.start_time} - ${time.timespan.end_time}`;
            return `${weekday}: ${timeRange}`;
        }).join(", ");
    }
    
    // Handle case when activity is an ActivityTime array
    if (Array.isArray(activity)) {
        if (activity.length === 0) return "Keine Zeiten festgelegt";
        
        return activity.map(time => {
            const weekday = formatWeekday(time.weekday);
            const timeRange = `${time.timespan.start_time} - ${time.timespan.end_time}`;
            return `${weekday}: ${timeRange}`;
        }).join(", ");
    }
    
    return "Keine Zeiten festgelegt";
}

export function formatWeekday(weekday: string): string {
    const weekdays: Record<string, string> = {
        "monday": "Mo",
        "tuesday": "Di",
        "wednesday": "Mi",
        "thursday": "Do",
        "friday": "Fr",
        "saturday": "Sa",
        "sunday": "So"
    };
    
    return weekdays[weekday.toLowerCase()] ?? weekday;
}

export function formatParticipantStatus(activityOrCurrent: Activity | number, max?: number): string {
    // Handle case when first parameter is an Activity object
    if (typeof activityOrCurrent === 'object' && activityOrCurrent !== null) {
        const activity = activityOrCurrent;
        if (activity.participant_count === undefined || activity.max_participant === undefined) {
            return "Unbekannt";
        }
        return `${activity.participant_count} / ${activity.max_participant} Teilnehmer`;
    }
    
    // Handle case when parameters are numbers (current, max)
    const current = activityOrCurrent;
    if (current === undefined || max === undefined) {
        return "Unbekannt";
    }
    return `${current} / ${max} Teilnehmer`;
}