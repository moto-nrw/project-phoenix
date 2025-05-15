// lib/active-helpers.ts
// Type definitions for active entities

// Backend types (from Go structs)
export interface BackendActiveGroup {
    id: number;
    group_id: number;
    room_id: number;
    start_time: string;
    end_time?: string;
    is_active: boolean;
    notes?: string;
    visit_count?: number;
    supervisor_count?: number;
    created_at: string;
    updated_at: string;
}

export interface BackendVisit {
    id: number;
    student_id: number;
    active_group_id: number;
    check_in_time: string;
    check_out_time?: string;
    is_active: boolean;
    notes?: string;
    student_name?: string;
    active_group_name?: string;
    created_at: string;
    updated_at: string;
}

export interface BackendSupervisor {
    id: number;
    staff_id: number;
    active_group_id: number;
    start_time: string;
    end_time?: string;
    is_active: boolean;
    notes?: string;
    staff_name?: string;
    active_group_name?: string;
    created_at: string;
    updated_at: string;
}

export interface BackendCombinedGroup {
    id: number;
    name: string;
    description?: string;
    room_id: number;
    start_time: string;
    end_time?: string;
    is_active: boolean;
    notes?: string;
    group_count?: number;
    created_at: string;
    updated_at: string;
}

export interface BackendGroupMapping {
    id: number;
    active_group_id: number;
    combined_group_id: number;
    group_name?: string;
    combined_name?: string;
}

export interface BackendAnalytics {
    active_groups_count?: number;
    total_visits_count?: number;
    active_visits_count?: number;
    room_utilization?: number;
    attendance_rate?: number;
}

// Frontend types
export interface ActiveGroup {
    id: string;
    groupId: string;
    roomId: string;
    startTime: Date;
    endTime?: Date;
    isActive: boolean;
    notes?: string;
    visitCount?: number;
    supervisorCount?: number;
    createdAt: Date;
    updatedAt: Date;
}

export interface Visit {
    id: string;
    studentId: string;
    activeGroupId: string;
    checkInTime: Date;
    checkOutTime?: Date;
    isActive: boolean;
    notes?: string;
    studentName?: string;
    activeGroupName?: string;
    createdAt: Date;
    updatedAt: Date;
}

export interface Supervisor {
    id: string;
    staffId: string;
    activeGroupId: string;
    startTime: Date;
    endTime?: Date;
    isActive: boolean;
    notes?: string;
    staffName?: string;
    activeGroupName?: string;
    createdAt: Date;
    updatedAt: Date;
}

export interface CombinedGroup {
    id: string;
    name: string;
    description?: string;
    roomId: string;
    startTime: Date;
    endTime?: Date;
    isActive: boolean;
    notes?: string;
    groupCount?: number;
    createdAt: Date;
    updatedAt: Date;
}

export interface GroupMapping {
    id: string;
    activeGroupId: string;
    combinedGroupId: string;
    groupName?: string;
    combinedName?: string;
}

export interface Analytics {
    activeGroupsCount?: number;
    totalVisitsCount?: number;
    activeVisitsCount?: number;
    roomUtilization?: number;
    attendanceRate?: number;
}

// Transformation functions
export function mapActiveGroupResponse(backendActiveGroup: BackendActiveGroup): ActiveGroup {
    return {
        id: String(backendActiveGroup.id),
        groupId: String(backendActiveGroup.group_id),
        roomId: String(backendActiveGroup.room_id),
        startTime: new Date(backendActiveGroup.start_time),
        endTime: backendActiveGroup.end_time ? new Date(backendActiveGroup.end_time) : undefined,
        isActive: backendActiveGroup.is_active,
        notes: backendActiveGroup.notes,
        visitCount: backendActiveGroup.visit_count,
        supervisorCount: backendActiveGroup.supervisor_count,
        createdAt: new Date(backendActiveGroup.created_at),
        updatedAt: new Date(backendActiveGroup.updated_at),
    };
}

export function mapVisitResponse(backendVisit: BackendVisit): Visit {
    return {
        id: String(backendVisit.id),
        studentId: String(backendVisit.student_id),
        activeGroupId: String(backendVisit.active_group_id),
        checkInTime: new Date(backendVisit.check_in_time),
        checkOutTime: backendVisit.check_out_time ? new Date(backendVisit.check_out_time) : undefined,
        isActive: backendVisit.is_active,
        notes: backendVisit.notes,
        studentName: backendVisit.student_name,
        activeGroupName: backendVisit.active_group_name,
        createdAt: new Date(backendVisit.created_at),
        updatedAt: new Date(backendVisit.updated_at),
    };
}

export function mapSupervisorResponse(backendSupervisor: BackendSupervisor): Supervisor {
    return {
        id: String(backendSupervisor.id),
        staffId: String(backendSupervisor.staff_id),
        activeGroupId: String(backendSupervisor.active_group_id),
        startTime: new Date(backendSupervisor.start_time),
        endTime: backendSupervisor.end_time ? new Date(backendSupervisor.end_time) : undefined,
        isActive: backendSupervisor.is_active,
        notes: backendSupervisor.notes,
        staffName: backendSupervisor.staff_name,
        activeGroupName: backendSupervisor.active_group_name,
        createdAt: new Date(backendSupervisor.created_at),
        updatedAt: new Date(backendSupervisor.updated_at),
    };
}

export function mapCombinedGroupResponse(backendCombinedGroup: BackendCombinedGroup): CombinedGroup {
    return {
        id: String(backendCombinedGroup.id),
        name: backendCombinedGroup.name,
        description: backendCombinedGroup.description,
        roomId: String(backendCombinedGroup.room_id),
        startTime: new Date(backendCombinedGroup.start_time),
        endTime: backendCombinedGroup.end_time ? new Date(backendCombinedGroup.end_time) : undefined,
        isActive: backendCombinedGroup.is_active,
        notes: backendCombinedGroup.notes,
        groupCount: backendCombinedGroup.group_count,
        createdAt: new Date(backendCombinedGroup.created_at),
        updatedAt: new Date(backendCombinedGroup.updated_at),
    };
}

export function mapGroupMappingResponse(backendGroupMapping: BackendGroupMapping): GroupMapping {
    return {
        id: String(backendGroupMapping.id),
        activeGroupId: String(backendGroupMapping.active_group_id),
        combinedGroupId: String(backendGroupMapping.combined_group_id),
        groupName: backendGroupMapping.group_name,
        combinedName: backendGroupMapping.combined_name,
    };
}

export function mapAnalyticsResponse(backendAnalytics: BackendAnalytics): Analytics {
    return {
        activeGroupsCount: backendAnalytics.active_groups_count,
        totalVisitsCount: backendAnalytics.total_visits_count,
        activeVisitsCount: backendAnalytics.active_visits_count,
        roomUtilization: backendAnalytics.room_utilization,
        attendanceRate: backendAnalytics.attendance_rate,
    };
}

// Request/Response types
export interface CreateActiveGroupRequest {
    group_id: number;
    room_id: number;
    start_time: string;
    end_time?: string;
    notes?: string;
}

export interface UpdateActiveGroupRequest {
    group_id: number;
    room_id: number;
    start_time: string;
    end_time?: string;
    notes?: string;
}

export interface CreateVisitRequest {
    student_id: number;
    active_group_id: number;
    check_in_time: string;
    check_out_time?: string;
    notes?: string;
}

export interface UpdateVisitRequest {
    student_id: number;
    active_group_id: number;
    check_in_time: string;
    check_out_time?: string;
    notes?: string;
}

export interface CreateSupervisorRequest {
    staff_id: number;
    active_group_id: number;
    start_time: string;
    end_time?: string;
    notes?: string;
}

export interface UpdateSupervisorRequest {
    staff_id: number;
    active_group_id: number;
    start_time: string;
    end_time?: string;
    notes?: string;
}

export interface CreateCombinedGroupRequest {
    name: string;
    description?: string;
    room_id: number;
    start_time: string;
    end_time?: string;
    notes?: string;
    group_ids?: number[];
}

export interface UpdateCombinedGroupRequest {
    name: string;
    description?: string;
    room_id: number;
    start_time: string;
    end_time?: string;
    notes?: string;
}

export interface GroupMappingRequest {
    active_group_id: number;
    combined_group_id: number;
}

// Utility functions to prepare data for backend
export function prepareActiveGroupForBackend(activeGroup: Partial<ActiveGroup>): Partial<CreateActiveGroupRequest> {
    const request: Partial<CreateActiveGroupRequest> = {};

    if (activeGroup.groupId) request.group_id = parseInt(activeGroup.groupId);
    if (activeGroup.roomId) request.room_id = parseInt(activeGroup.roomId);
    if (activeGroup.startTime) request.start_time = activeGroup.startTime.toISOString();
    if (activeGroup.endTime) request.end_time = activeGroup.endTime.toISOString();
    if (activeGroup.notes !== undefined) request.notes = activeGroup.notes;

    return request;
}

export function prepareVisitForBackend(visit: Partial<Visit>): Partial<CreateVisitRequest> {
    const request: Partial<CreateVisitRequest> = {};

    if (visit.studentId) request.student_id = parseInt(visit.studentId);
    if (visit.activeGroupId) request.active_group_id = parseInt(visit.activeGroupId);
    if (visit.checkInTime) request.check_in_time = visit.checkInTime.toISOString();
    if (visit.checkOutTime) request.check_out_time = visit.checkOutTime.toISOString();
    if (visit.notes !== undefined) request.notes = visit.notes;

    return request;
}

export function prepareSupervisorForBackend(supervisor: Partial<Supervisor>): Partial<CreateSupervisorRequest> {
    const request: Partial<CreateSupervisorRequest> = {};

    if (supervisor.staffId) request.staff_id = parseInt(supervisor.staffId);
    if (supervisor.activeGroupId) request.active_group_id = parseInt(supervisor.activeGroupId);
    if (supervisor.startTime) request.start_time = supervisor.startTime.toISOString();
    if (supervisor.endTime) request.end_time = supervisor.endTime.toISOString();
    if (supervisor.notes !== undefined) request.notes = supervisor.notes;

    return request;
}

export function prepareCombinedGroupForBackend(combinedGroup: Partial<CombinedGroup>): Partial<CreateCombinedGroupRequest> {
    const request: Partial<CreateCombinedGroupRequest> = {};

    if (combinedGroup.name) request.name = combinedGroup.name;
    if (combinedGroup.description !== undefined) request.description = combinedGroup.description;
    if (combinedGroup.roomId) request.room_id = parseInt(combinedGroup.roomId);
    if (combinedGroup.startTime) request.start_time = combinedGroup.startTime.toISOString();
    if (combinedGroup.endTime) request.end_time = combinedGroup.endTime.toISOString();
    if (combinedGroup.notes !== undefined) request.notes = combinedGroup.notes;

    return request;
}

export function prepareGroupMappingForBackend(mapping: { activeGroupId: string; combinedGroupId: string }): GroupMappingRequest {
    return {
        active_group_id: parseInt(mapping.activeGroupId),
        combined_group_id: parseInt(mapping.combinedGroupId),
    };
}