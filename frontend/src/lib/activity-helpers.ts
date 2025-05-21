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

export interface Timeframe {
    id: string;
    name: string;
    start_time: string;
    end_time: string;
    description?: string;
}

export interface BackendTimeframe {
    id: number;
    name: string;
    start_time: string;
    end_time: string;
    description?: string;
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

// Map an array of activity supervisors
export function mapActivitySupervisorsResponse(supervisors: BackendActivitySupervisor[]): ActivitySupervisor[] {
    return supervisors.map(mapActivitySupervisor);
}

// Prepare a supervisor assignment for backend
export function prepareSupervisorAssignmentForBackend(assignment: {
    staff_id: string;
    is_primary?: boolean;
}): {
    staff_id: number;
    is_primary?: boolean;
} {
    return {
        staff_id: parseInt(assignment.staff_id, 10),
        is_primary: assignment.is_primary
    };
}

// Format a list of supervisors as a comma-separated string
export function formatSupervisorList(supervisors: ActivitySupervisor[] | undefined): string {
    if (!supervisors || supervisors.length === 0) {
        return 'Keine Betreuer zugewiesen';
    }
    
    return supervisors.map(supervisor => 
        supervisor.full_name || 
        (supervisor.first_name && supervisor.last_name ? 
            `${supervisor.first_name} ${supervisor.last_name}` : 
            `Betreuer ${supervisor.id}`)
    ).join(', ');
}

// Get primary supervisor from a list of supervisors
export function getPrimarySupervisor(supervisors: ActivitySupervisor[] | undefined): ActivitySupervisor | undefined {
    if (!supervisors || supervisors.length === 0) {
        return undefined;
    }
    
    return supervisors.find(s => s.is_primary);
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

// Map an array of activity students
export function mapActivityStudentsResponse(students: BackendActivityStudent[]): ActivityStudent[] {
    return students.map(mapActivityStudentResponse);
}

// Format an array of students as a comma-separated string
export function formatStudentList(students: ActivityStudent[] | undefined): string {
    if (!students || students.length === 0) {
        return 'Keine Teilnehmer eingeschrieben';
    }
    
    return students.map(student => 
        student.name || `Student ${student.student_id}`
    ).join(', ');
}

// Group students by school class
export function groupStudentsByClass(students: ActivityStudent[]): Record<string, ActivityStudent[]> {
    const grouped: Record<string, ActivityStudent[]> = {};
    
    students.forEach(student => {
        const schoolClass = student.school_class || 'Keine Klasse';
        if (!grouped[schoolClass]) {
            grouped[schoolClass] = [];
        }
        grouped[schoolClass].push(student);
    });
    
    return grouped;
}

// Prepare batch enrollment data for backend
export function prepareBatchEnrollmentForBackend(studentIds: string[]): { student_ids: number[] } {
    return {
        student_ids: studentIds.map(id => parseInt(id, 10))
    };
}

// Filter students by search term
export function filterStudentsBySearchTerm(students: ActivityStudent[], searchTerm: string): ActivityStudent[] {
    if (!searchTerm) {
        return students;
    }
    
    const term = searchTerm.toLowerCase();
    return students.filter(student => 
        (student.name && student.name.toLowerCase().includes(term)) ||
        (student.school_class && student.school_class.toLowerCase().includes(term)) ||
        (student.student_id && student.student_id.toLowerCase().includes(term))
    );
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

// Map a timeframe from backend to frontend format
export function mapTimeframeResponse(backendTimeframe: BackendTimeframe): Timeframe {
    return {
        id: String(backendTimeframe.id),
        name: backendTimeframe.name,
        start_time: backendTimeframe.start_time,
        end_time: backendTimeframe.end_time,
        description: backendTimeframe.description
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

// Schedule filter type
export interface ActivityScheduleFilter {
    weekday?: string;
    has_timeframe?: boolean;
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

export function getWeekdayFullName(weekday: string): string {
    const weekdays: Record<string, string> = {
        "monday": "Montag",
        "tuesday": "Dienstag",
        "wednesday": "Mittwoch",
        "thursday": "Donnerstag",
        "friday": "Freitag",
        "saturday": "Samstag",
        "sunday": "Sonntag",
        "mo": "Montag",
        "di": "Dienstag",
        "mi": "Mittwoch",
        "do": "Donnerstag",
        "fr": "Freitag",
        "sa": "Samstag",
        "so": "Sonntag"
    };
    
    return weekdays[weekday.toLowerCase()] ?? weekday;
}

export function getWeekdayOrder(weekday: string): number {
    const order: Record<string, number> = {
        "monday": 1,
        "montag": 1,
        "mo": 1,
        "tuesday": 2,
        "dienstag": 2,
        "di": 2,
        "wednesday": 3,
        "mittwoch": 3,
        "mi": 3,
        "thursday": 4,
        "donnerstag": 4,
        "do": 4,
        "friday": 5,
        "freitag": 5,
        "fr": 5,
        "saturday": 6,
        "samstag": 6, 
        "sa": 6,
        "sunday": 7,
        "sonntag": 7,
        "so": 7
    };
    
    return order[weekday.toLowerCase()] ?? 99;
}

export function sortSchedulesByWeekday(schedules: ActivitySchedule[]): ActivitySchedule[] {
    return [...schedules].sort((a, b) => getWeekdayOrder(a.weekday) - getWeekdayOrder(b.weekday));
}

export function formatScheduleTime(schedule: ActivitySchedule, timeframes?: Array<{ id: string; start_time: string; end_time: string }>): string {
    if (!schedule.timeframe_id || !timeframes) {
        return formatWeekday(schedule.weekday);
    }
    
    const timeframe = timeframes.find(tf => tf.id === schedule.timeframe_id);
    if (!timeframe) {
        return formatWeekday(schedule.weekday);
    }
    
    return `${formatWeekday(schedule.weekday)} ${timeframe.start_time}-${timeframe.end_time}`;
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

// Check if a time slot is available for an activity
export function isTimeSlotAvailable(
    weekday: string, 
    timeframeId: string, 
    existingSchedules: ActivitySchedule[], 
    excludeScheduleId?: string
): boolean {
    // Filter out the schedule we're currently editing (if provided)
    const relevantSchedules = excludeScheduleId 
        ? existingSchedules.filter(s => s.id !== excludeScheduleId) 
        : existingSchedules;
    
    // Check if there's any schedule that overlaps with the requested time
    return !relevantSchedules.some(schedule => 
        schedule.weekday.toLowerCase() === weekday.toLowerCase() && 
        schedule.timeframe_id === timeframeId
    );
}

// Check if a supervisor is available for an activity
export function isSupervisorAvailable(
    supervisorId: string,
    weekday: string,
    timeframeId: string,
    existingSupervisorSchedules: Array<{
        activity_id: string;
        weekday: string;
        timeframe_id?: string;
    }>
): boolean {
    // Check if supervisor has any conflicting schedules
    return !existingSupervisorSchedules.some(schedule => 
        schedule.weekday.toLowerCase() === weekday.toLowerCase() && 
        schedule.timeframe_id === timeframeId
    );
}