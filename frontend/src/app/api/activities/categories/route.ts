// app/api/activities/categories/route.ts
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";
import type { BackendActivityCategory } from "~/lib/activity-helpers";
import { mapActivityCategoryResponse } from "~/lib/activity-helpers";

// Mock categories for testing
const MOCK_CATEGORIES: BackendActivityCategory[] = [
  { 
    id: 1, 
    name: "Sport", 
    description: "Sportliche Aktivitäten",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  { 
    id: 2, 
    name: "Musik", 
    description: "Musikalische Aktivitäten",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  { 
    id: 3, 
    name: "Kunst", 
    description: "Kreative Aktivitäten",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  { 
    id: 4, 
    name: "Wissenschaft", 
    description: "Wissenschaftliche Aktivitäten",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  { 
    id: 5, 
    name: "Sprachen", 
    description: "Sprachkurse",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
];

/**
 * Handler for GET /api/activities/categories
 * Returns a list of activity categories
 */
export const GET = createGetHandler(async (request: NextRequest, token: string) => {
  try {
    const response = await apiGet<{ status: string; data: BackendActivityCategory[] }>('/api/activities/categories', token);
    
    // Handle response structure
    if (response && response.status === "success" && Array.isArray(response.data)) {
      return response.data.map(mapActivityCategoryResponse);
    }
    
    // If we get here, we have a response but it's not in the expected format
    console.error('Unexpected response structure:', response);
    throw new Error('Unexpected response structure from categories API');
  } catch (error) {
    console.error('Error fetching categories:', error);
    
    // For now, we'll return mock data to ensure frontend doesn't break
    // In the future, this should be removed when API is stable
    console.warn('Falling back to mock categories data');
    return MOCK_CATEGORIES.map(mapActivityCategoryResponse);
  }
});