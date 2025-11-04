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
    weekday: number; // ISO 8601 integers 1-7 (1=Monday, 7=Sunday)
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
    // Direct properties for supervisors from activities API
    first_name?: string;
    last_name?: string;
    staff_id?: number;
    is_primary?: boolean;
    is_teacher?: boolean;
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

// Actual backend structure for enrolled students from /api/activities/{id}/students
// Backend returns a simplified StudentResponse with id, first_name, last_name
export interface BackendStudentEnrollment {
    id: number;  // This is the student ID
    student_id?: number;  // Backend doesn't actually return this field
    activity_group_id?: number;
    enrollment_date?: string;
    attendance_status?: string;
    created_at?: string;
    updated_at?: string;
    // Direct person fields (backend processed)
    first_name?: string;
    last_name?: string;
    school_class?: string;
    in_house?: boolean;
    // Fallback: Flattened fields in case backend changes
    student__school_class?: string;
    student__in_house?: boolean;
    person__first_name?: string;
    person__last_name?: string;
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
    display_name?: string;
}

export interface BackendTimeframe {
    id: number;
    name: string;
    start_time: string;
    end_time: string;
    description?: string;
    display_name?: string;
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
        supervisor.full_name ?? 
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
    if (backendActivity.category?.name) {
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
            weekday: String(schedule.weekday),
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
        weekday: String(backendSchedule.weekday),
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

// Map enrolled students from backend enrollment structure
export function mapStudentEnrollmentResponse(enrollment: BackendStudentEnrollment): ActivityStudent {
    // Handle both direct and flattened person fields
    const firstName = enrollment.first_name ?? enrollment.person__first_name ?? '';
    const lastName = enrollment.last_name ?? enrollment.person__last_name ?? '';
    const fullName = `${firstName} ${lastName}`.trim() || 'Unnamed Student';
    
    return {
        id: String(enrollment.id),
        activity_id: enrollment.activity_group_id ? String(enrollment.activity_group_id) : '',
        // Backend returns 'id' as the student ID, not 'student_id'
        student_id: String(enrollment.id),
        name: fullName,
        school_class: enrollment.school_class ?? enrollment.student__school_class ?? '',
        in_house: enrollment.in_house ?? enrollment.student__in_house ?? false,
        created_at: enrollment.created_at ? new Date(enrollment.created_at) : new Date(),
        updated_at: enrollment.updated_at ? new Date(enrollment.updated_at) : new Date(),
    };
}

// Map an array of enrolled students
export function mapStudentEnrollmentsResponse(enrollments: BackendStudentEnrollment[]): ActivityStudent[] {
    return enrollments.map(mapStudentEnrollmentResponse);
}

// Format an array of students as a comma-separated string
export function formatStudentList(students: ActivityStudent[] | undefined): string {
    if (!students || students.length === 0) {
        return 'Keine Teilnehmer eingeschrieben';
    }
    
    return students.map(student => 
        student.name ?? `Student ${student.student_id}`
    ).join(', ');
}

// Group students by school class
export function groupStudentsByClass(students: ActivityStudent[]): Record<string, ActivityStudent[]> {
    const grouped: Record<string, ActivityStudent[]> = {};
    
    students.forEach(student => {
        const schoolClass = student.school_class ?? 'Keine Klasse';
        grouped[schoolClass] ??= [];
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
        (student.name?.toLowerCase().includes(term) ?? false) ||
        (student.school_class?.toLowerCase().includes(term) ?? false) ||
        (student.student_id?.toLowerCase().includes(term) ?? false)
    );
}

// Check if a user (staff member) is the creator of an activity
export function isActivityCreator(activity: Activity, staffId: string | null | undefined): boolean {
    if (!staffId || !activity.supervisors || activity.supervisors.length === 0) {
        return false;
    }
    
    // Find the primary supervisor (creator)
    const primarySupervisor = activity.supervisors.find(s => s.is_primary);
    
    // If no primary supervisor is explicitly marked, consider the first supervisor as creator
    const creator = primarySupervisor ?? activity.supervisors[0];
    
    return creator?.staff_id === staffId;
}

// Added: Map supervisor response
export function mapSupervisorResponse(backendSupervisor: unknown): Supervisor {
    // Handle null or undefined input
    if (!backendSupervisor) {
        return {
            id: "0",
            name: "Unknown Supervisor"
        };
    }
    
    // Extract the ID safely
    const rawId = typeof backendSupervisor === 'object' && 'id' in backendSupervisor ? backendSupervisor.id : undefined;
    const id = rawId !== undefined && rawId !== null && (typeof rawId === 'string' || typeof rawId === 'number') ? String(rawId) : "0";
    
    // Handle different response formats we might get
    if (typeof backendSupervisor === 'object' && 'person' in backendSupervisor && 
        typeof backendSupervisor.person === 'object' && backendSupervisor.person &&
        'first_name' in backendSupervisor.person && 'last_name' in backendSupervisor.person) {
        // Standard staff format with person property
        const firstName = backendSupervisor.person.first_name;
        const lastName = backendSupervisor.person.last_name;
        return {
            id: id,
            name: `${typeof firstName === 'string' ? firstName : ''} ${typeof lastName === 'string' ? lastName : ''}`.trim()
        };
    } else if (typeof backendSupervisor === 'object' && 'first_name' in backendSupervisor && 
               'last_name' in backendSupervisor && backendSupervisor.first_name && backendSupervisor.last_name) {
        // Response format from activities/supervisors/available endpoint
        const firstName = backendSupervisor.first_name;
        const lastName = backendSupervisor.last_name;
        return {
            id: id,
            name: `${typeof firstName === 'string' ? firstName : ''} ${typeof lastName === 'string' ? lastName : ''}`.trim()
        };
    } else if (typeof backendSupervisor === 'object' && 'name' in backendSupervisor && backendSupervisor.name) {
        // Object already has a name property
        const name = backendSupervisor.name;
        return {
            id: id,
            name: typeof name === 'string' ? name : (typeof name === 'number' ? String(name) : 'Unknown')
        }; 
    } else {
        // Fallback if we can't determine the name
        return {
            id: id,
            name: `Supervisor ${id}`
        };
    }
}

// Map a timeframe from backend to frontend format
export function mapTimeframeResponse(backendTimeframe: BackendTimeframe): Timeframe {
    return {
        id: String(backendTimeframe.id),
        name: backendTimeframe.name,
        start_time: backendTimeframe.start_time,
        end_time: backendTimeframe.end_time,
        description: backendTimeframe.description,
        display_name: backendTimeframe.display_name
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
        weekday: schedule.weekday ? parseInt(schedule.weekday, 10) : undefined,
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
        weekday: number;
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
        weekday: number;
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
        if (times?.length === 0) return "Keine Zeiten festgelegt";
        
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
        "1": "Mo",
        "2": "Di",
        "3": "Mi",
        "4": "Do",
        "5": "Fr",
        "6": "Sa",
        "7": "So"
    };
    
    return weekdays[weekday] ?? weekday;
}

export function getWeekdayFullName(weekday: string): string {
    const weekdays: Record<string, string> = {
        "1": "Montag",
        "2": "Dienstag",
        "3": "Mittwoch",
        "4": "Donnerstag",
        "5": "Freitag",
        "6": "Samstag",
        "7": "Sonntag"
    };
    
    return weekdays[weekday] ?? weekday;
}

export function getWeekdayOrder(weekday: string): number {
    const order: Record<string, number> = {
        "1": 1,
        "2": 2,
        "3": 3,
        "4": 4,
        "5": 5,
        "6": 6,
        "7": 7
    };
    
    return order[weekday] ?? 99;
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

// Get appropriate Tailwind gradient classes based on category name
export function getActivityCategoryColor(categoryName?: string | null): string {
    if (!categoryName) return "from-gray-500 to-gray-600";
    
    const categoryColors: Record<string, string> = {
        "Sport": "from-blue-500 to-indigo-600",
        "Kreativ": "from-purple-500 to-pink-600",
        "Musik": "from-pink-500 to-rose-600",
        "Spiele": "from-green-500 to-emerald-600",
        "Lernen": "from-yellow-500 to-orange-600",
        "Hausaufgaben": "from-red-500 to-pink-600",
        "Draußen": "from-green-600 to-teal-600",
        "Gruppenraum": "from-slate-500 to-gray-600",
        "Mensa": "from-orange-500 to-amber-600",
    };
    
    return categoryColors[categoryName] ?? "from-gray-500 to-gray-600";
}