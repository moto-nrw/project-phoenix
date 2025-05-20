// lib/activity-helpers.ts
// Type definitions and helper functions for activities

// Backend supervisor response type from API
export interface BackendActivitySupervisor {
    id: number;
    staff_id: number;
    is_primary: boolean;
    first_name?: string;
    last_name?: string;
}

// Backend types (from Go structs)
export interface BackendActivity {
    id: number;
    name: string;
    max_participants: number;
    is_open: boolean;
    category_id: number;
    planned_room_id?: number;
    supervisor_id?: number;         // Primary supervisor from API
    supervisor_ids?: number[];      // Array of supervisors from API
    supervisors?: BackendActivitySupervisor[]; // Detailed supervisor info
    enrollment_count?: number;      // Number of enrolled students
    created_at: string;
    updated_at: string;
    category?: BackendActivityCategory;
    schedules?: BackendActivitySchedule[];
}

export interface BackendActivityCategory {
    id: number;
    name: string;
    description?: string;
    color?: string;
    created_at: string;
    updated_at: string;
}

export interface BackendActivitySchedule {
    id: number;
    weekday: string; // MONDAY, TUESDAY, etc.
    timeframe_id?: number;
    activity_group_id: number;
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

export interface BackendActivityStudent {
    id: number;
    student_id: number;
    activity_id: number;
    name?: string;
    school_class?: string;
    in_house?: boolean;
    created_at: string;
    updated_at: string;
}

// Frontend supervisor type
export interface ActivitySupervisor {
    id: string;
    staff_id: string;
    is_primary: boolean;
    first_name?: string;
    last_name?: string;
    full_name?: string;
}

// Frontend types
export interface Activity {
    id: string;
    name: string;
    max_participant: number;
    is_open_ags: boolean;
    supervisor_id: string;
    supervisor_name?: string;
    supervisors?: ActivitySupervisor[];
    ag_category_id: string;
    category_name?: string;
    planned_room_id?: string;
    created_at: Date;
    updated_at: Date;
    participant_count?: number;
    times?: ActivitySchedule[];
    students?: ActivityStudent[];
}

export interface ActivityCategory {
    id: string;
    name: string;
    description?: string;
    color?: string;
    created_at: Date;
    updated_at: Date;
}

export interface ActivitySchedule {
    id: string;
    activity_id: string;
    weekday: string;
    timeframe_id?: string;
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

// Helper function to map backend supervisor to frontend supervisor
function mapActivitySupervisor(supervisor: BackendActivitySupervisor): ActivitySupervisor {
    const fullName = supervisor.first_name && supervisor.last_name 
        ? `${supervisor.first_name} ${supervisor.last_name}`
        : undefined;
        
    return {
        id: String(supervisor.id),
        staff_id: String(supervisor.staff_id),
        is_primary: supervisor.is_primary,
        first_name: supervisor.first_name,
        last_name: supervisor.last_name,
        full_name: fullName
    };
}

// Mapping functions for backend to frontend types
export function mapActivityResponse(backendActivity: BackendActivity): Activity {
    // Initialize with basic fields
    const activity: Activity = {
        id: String(backendActivity.id),
        name: backendActivity.name,
        max_participant: backendActivity.max_participants,
        is_open_ags: backendActivity.is_open,
        supervisor_id: '',
        ag_category_id: String(backendActivity.category_id),
        created_at: new Date(backendActivity.created_at),
        updated_at: new Date(backendActivity.updated_at),
        participant_count: backendActivity.enrollment_count ?? 0,
        times: [],
        students: [],
    };

    // Add planned room ID if available
    if (backendActivity.planned_room_id) {
        activity.planned_room_id = String(backendActivity.planned_room_id);
    }

    // Add category name if available
    if (backendActivity.category && backendActivity.category.name) {
        activity.category_name = backendActivity.category.name;
    }

    // Handle supervisor information
    if (backendActivity.supervisors && backendActivity.supervisors.length > 0) {
        // Map detailed supervisor information
        activity.supervisors = backendActivity.supervisors.map(mapActivitySupervisor);
        
        // Find primary supervisor for backward compatibility
        const primarySupervisor = backendActivity.supervisors.find(s => s.is_primary);
        if (primarySupervisor) {
            activity.supervisor_id = String(primarySupervisor.staff_id);
            activity.supervisor_name = primarySupervisor.first_name && primarySupervisor.last_name 
                ? `${primarySupervisor.first_name} ${primarySupervisor.last_name}`
                : undefined;
        }
    } else {
        // Fallback to old supervisor ID fields if no detailed info
        if (backendActivity.supervisor_id) {
            activity.supervisor_id = String(backendActivity.supervisor_id);
        } else if (backendActivity.supervisor_ids && backendActivity.supervisor_ids.length > 0) {
            activity.supervisor_id = String(backendActivity.supervisor_ids[0]);
        }
    }

    // Handle schedules if available
    if (backendActivity.schedules && backendActivity.schedules.length > 0) {
        activity.times = backendActivity.schedules.map(schedule => ({
            id: String(schedule.id),
            activity_id: String(schedule.activity_group_id),
            weekday: schedule.weekday.toLowerCase(),
            timeframe_id: schedule.timeframe_id ? String(schedule.timeframe_id) : undefined,
            created_at: new Date(schedule.created_at),
            updated_at: new Date(schedule.updated_at)
        }));
    }

    return activity;
}

export function mapActivityCategoryResponse(backendCategory: BackendActivityCategory): ActivityCategory {
    return {
        id: String(backendCategory.id),
        name: backendCategory.name,
        description: backendCategory.description,
        color: backendCategory.color,
        created_at: new Date(backendCategory.created_at),
        updated_at: new Date(backendCategory.updated_at),
    };
}

export function mapActivityScheduleResponse(backendSchedule: BackendActivitySchedule): ActivitySchedule {
    return {
        id: String(backendSchedule.id),
        activity_id: String(backendSchedule.activity_group_id),
        weekday: backendSchedule.weekday.toLowerCase(),
        timeframe_id: backendSchedule.timeframe_id ? String(backendSchedule.timeframe_id) : undefined,
        created_at: new Date(backendSchedule.created_at),
        updated_at: new Date(backendSchedule.updated_at),
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
        max_participants: activity.max_participant,
        is_open: activity.is_open_ags,
        category_id: activity.ag_category_id ? parseInt(activity.ag_category_id, 10) : undefined,
        planned_room_id: activity.planned_room_id ? parseInt(activity.planned_room_id, 10) : undefined,
        // Convert single supervisor to array if present
        supervisor_ids: activity.supervisor_id ? [parseInt(activity.supervisor_id, 10)] : undefined,
    };
    
    return result;
}

export function prepareActivityScheduleForBackend(schedule: Partial<ActivitySchedule>): Partial<BackendActivitySchedule> {
    return {
        id: schedule.id ? parseInt(schedule.id, 10) : undefined,
        activity_group_id: schedule.activity_id ? parseInt(schedule.activity_id, 10) : undefined,
        weekday: schedule.weekday ? schedule.weekday.toUpperCase() : undefined,
        timeframe_id: schedule.timeframe_id ? parseInt(schedule.timeframe_id, 10) : undefined,
    };
}

// Request/Response types
export interface CreateActivityRequest {
    name: string;
    max_participants: number;
    is_open: boolean;
    category_id: number;
    planned_room_id?: number;
    supervisor_ids?: number[];
    schedules?: {
        weekday: string;
        timeframe_id?: number;
    }[];
}

export interface UpdateActivityRequest {
    name: string;
    max_participants: number;
    is_open: boolean;
    category_id: number;
    planned_room_id?: number;
    supervisor_ids?: number[];
    schedules?: {
        weekday: string;
        timeframe_id?: number;
    }[];
}

// Added: Activity filter type
export interface ActivityFilter {
    search?: string;
    category_id?: string;
    is_open_ags?: boolean;
}

// Helper functions 
export function formatActivityTimes(activity: Activity | ActivitySchedule[]): string {
    // Handle case when activity is an Activity object
    if ('times' in activity && Array.isArray(activity.times)) {
        const times = activity.times;
        if (!times || times.length === 0) return "Keine Zeiten festgelegt";
        
        return times.map(time => {
            const weekday = formatWeekday(time.weekday);
            return `${weekday}`;
        }).join(", ");
    }
    
    // Handle case when activity is an ActivitySchedule array
    if (Array.isArray(activity)) {
        if (activity.length === 0) return "Keine Zeiten festgelegt";
        
        return activity.map(time => {
            const weekday = formatWeekday(time.weekday);
            return `${weekday}`;
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