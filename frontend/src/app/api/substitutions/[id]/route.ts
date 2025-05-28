import type { NextRequest } from "next/server";
import { apiGet, apiPut, apiDelete } from "~/lib/api-helpers";
import { createGetHandler, createPutHandler, createDeleteHandler } from "~/lib/route-wrapper";
import type { BackendSubstitution } from "~/lib/substitution-helpers";

// Context type is used implicitly by the route handlers

/**
 * Handler for GET /api/substitutions/[id]
 * Returns a single substitution by ID
 */
export const GET = createGetHandler(async (request: NextRequest, token: string, params: Record<string, unknown>) => {
  const id = params.id as string;
  
  if (!id) {
    throw new Error('Substitution ID is required');
  }
  
  const endpoint = `/api/substitutions/${id}`;
  
  // Fetch substitution from the API
  return await apiGet<BackendSubstitution>(endpoint, token);
});

/**
 * Handler for PUT /api/substitutions/[id]
 * Updates an existing substitution
 */
export const PUT = createPutHandler(async (req: NextRequest, body: Partial<BackendSubstitution>, token: string, params: Record<string, unknown>) => {
  const id = params.id as string;
  
  if (!id) {
    throw new Error('Substitution ID is required');
  }
  
  const endpoint = `/api/substitutions/${id}`;
  
  // Update substitution via the API
  return await apiPut<BackendSubstitution>(endpoint, token, body);
});

/**
 * Handler for DELETE /api/substitutions/[id]
 * Deletes a substitution
 */
export const DELETE = createDeleteHandler(async (request: NextRequest, token: string, params: Record<string, unknown>) => {
  const id = params.id as string;
  
  if (!id) {
    throw new Error('Substitution ID is required');
  }
  
  const endpoint = `/api/substitutions/${id}`;
  
  // Delete substitution via the API
  await apiDelete(endpoint, token);
  
  // Return success response
  return { success: true };
});