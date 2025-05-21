// src/app/api/activities/[id]/supervisors/[supervisorId]/route.ts
import { NextRequest } from "next/server";
import { 
  createPutHandler,
  createDeleteHandler
} from "~/lib/route-wrapper";
import {
  updateSupervisorRole,
  removeSupervisor,
  getActivitySupervisors
} from "~/lib/activity-api";

/**
 * PUT handler for updating a supervisor's role for an activity (primarily is_primary status)
 * @route PUT /api/activities/:id/supervisors/:supervisorId
 * Request body: { is_primary: boolean }
 */
export const PUT = createPutHandler(async (
  _request: NextRequest,
  body: { is_primary: boolean },
  token: string,
  params: Record<string, unknown>
) => {
  // Extract parameters
  const activityId = String(params.id);
  const supervisorId = String(params.supervisorId);
  
  if (!activityId) {
    throw new Error("Activity ID is required");
  }

  if (!supervisorId) {
    throw new Error("Supervisor ID is required");
  }
  
  if (body.is_primary === undefined) {
    throw new Error("is_primary parameter is required");
  }

  // Update the supervisor role
  const success = await updateSupervisorRole(
    activityId, 
    supervisorId, 
    { is_primary: body.is_primary }
  );
  
  if (!success) {
    throw new Error("Failed to update supervisor role");
  }

  // If successful, return the updated supervisors list
  const updatedSupervisors = await getActivitySupervisors(activityId);
  return updatedSupervisors;
});

/**
 * DELETE handler for removing a supervisor from an activity
 * @route DELETE /api/activities/:id/supervisors/:supervisorId
 */
export const DELETE = createDeleteHandler(async (
  _request: NextRequest,
  token: string,
  params: Record<string, unknown>
) => {
  // Extract parameters
  const activityId = String(params.id);
  const supervisorId = String(params.supervisorId);
  
  if (!activityId) {
    throw new Error("Activity ID is required");
  }

  if (!supervisorId) {
    throw new Error("Supervisor ID is required");
  }

  // Remove the supervisor
  const success = await removeSupervisor(activityId, supervisorId);
  
  if (!success) {
    throw new Error("Failed to remove supervisor");
  }

  return { success: true };
});