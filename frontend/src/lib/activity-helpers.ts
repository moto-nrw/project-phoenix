// lib/activity-helpers.ts
// Type definitions and helper functions for activities

// Backend types (from Go structs)
export interface BackendActivity {
    id: number;
    name: string;
    max_participants: number;  // Fixed: was max_participant
    category_id: number;  // Fixed: was ag_category_id
    supervisor_ids?: number[];  // Fixed: was supervisor_id, now optional array
    created_at: string;
    updated_at: string;
    category?: BackendActivityCategory;  // Optional for responses
}

export interface BackendActivityCategory {
    id: number;
    name: string;
    description?: string;
    created_at: string;
    updated_at: string;
}

// Added: Backend supervisor type
export interface BackendSupervisor {
    id: number;
    person?: {
        first_name: string;
        last_name: string;
    };
    is_teacher: boolean;
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

// Added: Frontend supervisor type
export interface Supervisor {
    id: string;
    name: string;
}

// Mapping functions for backend to frontend types
export function mapActivityResponse(backendActivity: BackendActivity): Activity {
    return {
        id: String(backendActivity.id),
        name: backendActivity.name,
        max_participant: backendActivity.max_participants,  // Fixed field name
        is_open_ags: false,  // Not used for now, default false
        // Take first supervisor from array if exists
        supervisor_id: backendActivity.supervisor_ids?.[0] ? String(backendActivity.supervisor_ids[0]) : '',
        ag_category_id: String(backendActivity.category_id),  // Fixed field name
        category_name: backendActivity.category?.name,
        created_at: new Date(backendActivity.created_at),
        updated_at: new Date(backendActivity.updated_at),
        // Optional fields - set defaults
        participant_count: 0,
        times: [],  // Ignore schedules for now
        students: [],  // Ignore enrollments for now
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

// Added: Map supervisor response
export function mapSupervisorResponse(backendSupervisor: BackendSupervisor): Supervisor {
    return {
        id: String(backendSupervisor.id),
        name: backendSupervisor.person 
            ? `${backendSupervisor.person.first_name} ${backendSupervisor.person.last_name}`
            : `Supervisor ${backendSupervisor.id}`
    };
}

// Prepare frontend types for backend requests
export function prepareActivityForBackend(activity: Partial<Activity>): Partial<BackendActivity> {
    const result: Partial<BackendActivity> = {
        id: activity.id ? parseInt(activity.id, 10) : undefined,
        name: activity.name,
        max_participants: activity.max_participant,  // Map field name
        category_id: activity.ag_category_id ? parseInt(activity.ag_category_id, 10) : undefined,  // Map field name
        // Convert single supervisor to array if present
        supervisor_ids: activity.supervisor_id ? [parseInt(activity.supervisor_id, 10)] : undefined,
    };
    
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
    max_participants: number;  // Fixed field name
    category_id: number;  // Fixed field name
    supervisor_ids?: number[];  // Optional array
}

export interface UpdateActivityRequest {
    name: string;
    max_participants: number;  // Fixed field name
    category_id: number;  // Fixed field name
    supervisor_ids?: number[];  // Optional array
}

// Added: Activity filter type
export interface ActivityFilter {
    search?: string;
    category_id?: string;
    is_open_ags?: boolean;
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