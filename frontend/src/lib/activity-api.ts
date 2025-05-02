import axios from 'axios';
import type { AxiosError } from 'axios';
import { getSession } from 'next-auth/react';
import { env } from '~/env';
import { mapActivityResponse, mapSingleActivityResponse, mapCategoryResponse, prepareActivityForBackend } from './activity-helpers';
import { handleAuthFailure } from './auth-api';
import type { Student } from './api';

// Activity interfaces
export interface Activity {
  id: string;
  name: string;
  max_participant: number;
  is_open_ags: boolean;
  supervisor_id: string;
  supervisor_name?: string;
  ag_category_id: string;
  category_name?: string;
  datespan_id?: string;
  created_at: string;
  updated_at: string;
  times?: ActivityTime[];
  students?: Student[];
  available_spots?: number;
  participant_count?: number;
}

export interface ActivityCategory {
  id: string;
  name: string;
  created_at: string;
}

export interface ActivityTime {
  id: string;
  weekday: string;
  timespan_id: string;
  ag_id: string;
  created_at: string;
  timespan?: {
    start_time: string;
    end_time?: string;
  }
}

// API service for Activity-related operations
export const activityService = {
  // Get all activities with optional filtering
  getActivities: async (filters?: { 
    search?: string, 
    category_id?: string, 
    supervisor_id?: string,
    is_open?: boolean
  }): Promise<Activity[]> => {
    // Build query parameters
    const params = new URLSearchParams();
    if (filters?.search) params.append('search', filters.search);
    if (filters?.category_id) params.append('category_id', filters.category_id);
    if (filters?.supervisor_id) params.append('supervisor_id', filters.supervisor_id);
    if (filters?.is_open !== undefined) params.append('is_open', filters.is_open.toString());
    
    // Use the nextjs api route which handles auth token properly
    const useProxyApi = typeof window !== 'undefined';
    let url = useProxyApi ? '/api/database/activities' : `${env.NEXT_PUBLIC_API_URL}/activities`;
    
    try {
      // Build query string for API route
      const queryString = params.toString();
      if (queryString) {
        url += `?${queryString}`;
      }
      
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          credentials: 'include',
          headers: session?.user?.token ? {
            'Authorization': `Bearer ${session.user.token}`,
            'Content-Type': 'application/json'
          } : undefined
        });
        
        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          
          // Try token refresh on 401 errors
          if (response.status === 401) {
            const refreshSuccessful = await handleAuthFailure();
            
            if (refreshSuccessful) {
              // Try the request again after token refresh
              const newSession = await getSession();
              const retryResponse = await fetch(url, {
                credentials: 'include',
                headers: newSession?.user?.token ? {
                  'Authorization': `Bearer ${newSession.user.token}`,
                  'Content-Type': 'application/json'
                } : undefined
              });
              
              if (retryResponse.ok) {
                return mapActivityResponse(await retryResponse.json());
              }
            }
          }
          
          throw new Error(`API error: ${response.status}`);
        }
        
        return mapActivityResponse(await response.json());
      } else {
        // Server-side: use axios with the API URL directly
        const api = axios.create({
          baseURL: env.NEXT_PUBLIC_API_URL,
          headers: { 'Content-Type': 'application/json' },
          withCredentials: true,
        });
        const response = await api.get(url, { params });
        return mapActivityResponse(response.data);
      }
    } catch (error) {
      console.error("Error fetching activities:", error);
      throw error;
    }
  },
  
  // Get a specific activity by ID
  getActivity: async (id: string): Promise<Activity> => {
    const useProxyApi = typeof window !== 'undefined';
    const url = useProxyApi ? `/api/database/activities/${id}` : `${env.NEXT_PUBLIC_API_URL}/activities/${id}`;
    
    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          credentials: 'include',
          headers: session?.user?.token ? {
            'Authorization': `Bearer ${session.user.token}`,
            'Content-Type': 'application/json'
          } : undefined
        });
        
        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          
          // Try token refresh on 401 errors
          if (response.status === 401) {
            const refreshSuccessful = await handleAuthFailure();
            
            if (refreshSuccessful) {
              // Try the request again after token refresh
              const newSession = await getSession();
              const retryResponse = await fetch(url, {
                credentials: 'include',
                headers: newSession?.user?.token ? {
                  'Authorization': `Bearer ${newSession.user.token}`,
                  'Content-Type': 'application/json'
                } : undefined
              });
              
              if (retryResponse.ok) {
                const data = await retryResponse.json();
                return mapSingleActivityResponse(data);
              }
            }
          }
          
          throw new Error(`API error: ${response.status}`);
        }
        
        const data = await response.json();
        return mapSingleActivityResponse(data);
      } else {
        // Server-side: use axios with the API URL directly
        const api = axios.create({
          baseURL: env.NEXT_PUBLIC_API_URL,
          headers: { 'Content-Type': 'application/json' },
          withCredentials: true,
        });
        const response = await api.get(url);
        return mapSingleActivityResponse(response.data);
      }
    } catch (error) {
      console.error(`Error fetching activity ${id}:`, error);
      throw error;
    }
  },
  
  // Create a new activity
  createActivity: async (activity: Omit<Activity, 'id'>): Promise<Activity> => {
    // Transform from frontend model to backend model
    const backendActivity = prepareActivityForBackend(activity);
    
    // Basic validation
    if (!backendActivity.name) {
      throw new Error('Missing required field: name');
    }
    if (!backendActivity.max_participant || backendActivity.max_participant < 1) {
      throw new Error('Maximum participants must be at least 1');
    }
    if (!backendActivity.supervisor_id) {
      throw new Error('Missing required field: supervisor_id');
    }
    if (!backendActivity.ag_category_id) {
      throw new Error('Missing required field: ag_category_id');
    }
    
    const useProxyApi = typeof window !== 'undefined';
    const url = useProxyApi ? `/api/database/activities` : `${env.NEXT_PUBLIC_API_URL}/activities`;
    
    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          method: 'POST',
          credentials: 'include',
          headers: session?.user?.token ? {
            'Authorization': `Bearer ${session.user.token}`,
            'Content-Type': 'application/json'
          } : undefined,
          body: JSON.stringify(backendActivity)
        });
        
        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          // Try to parse error for more detailed message
          try {
            const errorJson = JSON.parse(errorText);
            if (errorJson.error) {
              throw new Error(`API error: ${errorJson.error}`);
            }
          } catch (e) {
            // If parsing fails, use status code
          }
          throw new Error(`API error: ${response.status}`);
        }
        
        const data = await response.json();
        return mapSingleActivityResponse(data);
      } else {
        // Server-side: use axios with the API URL directly
        const api = axios.create({
          baseURL: env.NEXT_PUBLIC_API_URL,
          headers: { 'Content-Type': 'application/json' },
          withCredentials: true,
        });
        const response = await api.post(url, backendActivity);
        return mapSingleActivityResponse(response.data);
      }
    } catch (error) {
      console.error(`Error creating activity:`, error);
      throw error;
    }
  },
  
  // Update an activity
  updateActivity: async (id: string, activity: Partial<Activity>): Promise<Activity> => {
    // Log the original activity data
    console.log('Updating activity with data:', JSON.stringify(activity, null, 2));
    
    // Transform from frontend model to backend model updates
    const backendUpdates = prepareActivityForBackend(activity);
    
    // Log the transformed backend model updates
    console.log('Backend updates prepared:', JSON.stringify(backendUpdates, null, 2));
    
    const useProxyApi = typeof window !== 'undefined';
    const url = useProxyApi ? `/api/database/activities/${id}` : `${env.NEXT_PUBLIC_API_URL}/activities/${id}`;
    
    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        
        // Log the actual request that will be sent
        console.log('Sending request to:', url);
        console.log('Request body:', JSON.stringify(backendUpdates, null, 2));
        
        const response = await fetch(url, {
          method: 'PUT',
          credentials: 'include',
          headers: session?.user?.token ? {
            'Authorization': `Bearer ${session.user.token}`,
            'Content-Type': 'application/json'
          } : undefined,
          body: JSON.stringify(backendUpdates)
        });
        
        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          
          // Try to parse error text as JSON for more detailed error
          try {
            const errorJson = JSON.parse(errorText);
            if (errorJson.error) {
              throw new Error(`API error ${response.status}: ${errorJson.error}`);
            }
          } catch (e) {
            // If parsing fails, use status code + error text
            throw new Error(`API error ${response.status}: ${errorText.substring(0, 100)}`);
          }
        }
        
        const data = await response.json();
        return mapSingleActivityResponse(data);
      } else {
        // Server-side: use axios with the API URL directly
        const api = axios.create({
          baseURL: env.NEXT_PUBLIC_API_URL,
          headers: { 'Content-Type': 'application/json' },
          withCredentials: true,
        });
        const response = await api.put(url, backendUpdates);
        return mapSingleActivityResponse(response.data);
      }
    } catch (error) {
      console.error(`Error updating activity ${id}:`, error);
      throw error;
    }
  },
  
  // Delete an activity
  deleteActivity: async (id: string): Promise<void> => {
    const useProxyApi = typeof window !== 'undefined';
    const url = useProxyApi ? `/api/database/activities/${id}` : `${env.NEXT_PUBLIC_API_URL}/activities/${id}`;
    
    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          method: 'DELETE',
          credentials: 'include',
          headers: session?.user?.token ? {
            'Authorization': `Bearer ${session.user.token}`,
            'Content-Type': 'application/json'
          } : undefined
        });
        
        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }
        
        return;
      } else {
        // Server-side: use axios with the API URL directly
        const api = axios.create({
          baseURL: env.NEXT_PUBLIC_API_URL,
          headers: { 'Content-Type': 'application/json' },
          withCredentials: true,
        });
        await api.delete(url);
        return;
      }
    } catch (error) {
      console.error(`Error deleting activity ${id}:`, error);
      throw error;
    }
  },
  
  // Get all categories
  getCategories: async (): Promise<ActivityCategory[]> => {
    const useProxyApi = typeof window !== 'undefined';
    const url = useProxyApi ? '/api/database/activities/categories' : `${env.NEXT_PUBLIC_API_URL}/activities/categories`;
    
    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          credentials: 'include',
          headers: session?.user?.token ? {
            'Authorization': `Bearer ${session.user.token}`,
            'Content-Type': 'application/json'
          } : undefined
        });
        
        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }
        
        return mapCategoryResponse(await response.json());
      } else {
        // Server-side: use axios with the API URL directly
        const api = axios.create({
          baseURL: env.NEXT_PUBLIC_API_URL,
          headers: { 'Content-Type': 'application/json' },
          withCredentials: true,
        });
        const response = await api.get(url);
        return mapCategoryResponse(response.data);
      }
    } catch (error) {
      console.error("Error fetching activity categories:", error);
      throw error;
    }
  },
  
  // Create a new category
  createCategory: async (category: { name: string }): Promise<ActivityCategory> => {
    const useProxyApi = typeof window !== 'undefined';
    const url = useProxyApi ? '/api/database/activities/categories' : `${env.NEXT_PUBLIC_API_URL}/activities/categories`;
    
    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          method: 'POST',
          credentials: 'include',
          headers: session?.user?.token ? {
            'Authorization': `Bearer ${session.user.token}`,
            'Content-Type': 'application/json'
          } : undefined,
          body: JSON.stringify(category)
        });
        
        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }
        
        return (await response.json()) as ActivityCategory;
      } else {
        // Server-side: use axios with the API URL directly
        const api = axios.create({
          baseURL: env.NEXT_PUBLIC_API_URL,
          headers: { 'Content-Type': 'application/json' },
          withCredentials: true,
        });
        const response = await api.post(url, category);
        return response.data as ActivityCategory;
      }
    } catch (error) {
      console.error(`Error creating activity category:`, error);
      throw error;
    }
  },
  
  // Enroll a student in an activity
  enrollStudent: async (activityId: string, studentId: string): Promise<void> => {
    const useProxyApi = typeof window !== 'undefined';
    const url = useProxyApi 
      ? `/api/database/activities/${activityId}/enroll/${studentId}` 
      : `${env.NEXT_PUBLIC_API_URL}/activities/${activityId}/enroll/${studentId}`;
    
    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          method: 'POST',
          credentials: 'include',
          headers: session?.user?.token ? {
            'Authorization': `Bearer ${session.user.token}`,
            'Content-Type': 'application/json'
          } : undefined
        });
        
        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          
          // Try to parse error text as JSON for more detailed error
          try {
            const errorJson = JSON.parse(errorText);
            if (errorJson.error) {
              throw new Error(errorJson.error);
            }
          } catch (e) {
            // If parsing fails, use status code
          }
          
          throw new Error(`API error: ${response.status}`);
        }
        
        return;
      } else {
        // Server-side: use axios with the API URL directly
        const api = axios.create({
          baseURL: env.NEXT_PUBLIC_API_URL,
          headers: { 'Content-Type': 'application/json' },
          withCredentials: true,
        });
        await api.post(url);
        return;
      }
    } catch (error) {
      console.error(`Error enrolling student ${studentId} in activity ${activityId}:`, error);
      throw error;
    }
  },
  
  // Unenroll a student from an activity
  unenrollStudent: async (activityId: string, studentId: string): Promise<void> => {
    const useProxyApi = typeof window !== 'undefined';
    const url = useProxyApi 
      ? `/api/database/activities/${activityId}/enroll/${studentId}` 
      : `${env.NEXT_PUBLIC_API_URL}/activities/${activityId}/enroll/${studentId}`;
    
    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          method: 'DELETE',
          credentials: 'include',
          headers: session?.user?.token ? {
            'Authorization': `Bearer ${session.user.token}`,
            'Content-Type': 'application/json'
          } : undefined
        });
        
        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }
        
        return;
      } else {
        // Server-side: use axios with the API URL directly
        const api = axios.create({
          baseURL: env.NEXT_PUBLIC_API_URL,
          headers: { 'Content-Type': 'application/json' },
          withCredentials: true,
        });
        await api.delete(url);
        return;
      }
    } catch (error) {
      console.error(`Error unenrolling student ${studentId} from activity ${activityId}:`, error);
      throw error;
    }
  },
  
  // Add a time slot to an activity
  addTimeSlot: async (activityId: string, timeSlot: Omit<ActivityTime, 'id' | 'ag_id' | 'created_at'>): Promise<ActivityTime> => {
    const useProxyApi = typeof window !== 'undefined';
    const url = useProxyApi 
      ? `/api/database/activities/${activityId}/times` 
      : `${env.NEXT_PUBLIC_API_URL}/activities/${activityId}/times`;
    
    // Log the incoming timeSlot data for debugging
    console.log('Adding time slot to activity:', activityId, 'with data:', JSON.stringify(timeSlot));
    
    // The backend expects timespan_id as a number (int64)
    const preparedTimeSlot = {
      ...timeSlot,
      timespan_id: typeof timeSlot.timespan_id === 'string' 
        ? parseInt(timeSlot.timespan_id, 10) 
        : timeSlot.timespan_id
    };
    
    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          method: 'POST',
          credentials: 'include',
          headers: session?.user?.token ? {
            'Authorization': `Bearer ${session.user.token}`,
            'Content-Type': 'application/json'
          } : undefined,
          body: JSON.stringify(preparedTimeSlot)
        });
        
        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }
        
        return (await response.json()) as ActivityTime;
      } else {
        // Server-side: use axios with the API URL directly
        const api = axios.create({
          baseURL: env.NEXT_PUBLIC_API_URL,
          headers: { 'Content-Type': 'application/json' },
          withCredentials: true,
        });
        const response = await api.post(url, preparedTimeSlot);
        return response.data as ActivityTime;
      }
    } catch (error) {
      console.error(`Error adding time slot to activity ${activityId}:`, error);
      throw error;
    }
  },
  
  // Delete a time slot from an activity
  deleteTimeSlot: async (activityId: string, timeSlotId: string): Promise<void> => {
    const useProxyApi = typeof window !== 'undefined';
    const url = useProxyApi 
      ? `/api/database/activities/${activityId}/times/${timeSlotId}` 
      : `${env.NEXT_PUBLIC_API_URL}/activities/${activityId}/times/${timeSlotId}`;
    
    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          method: 'DELETE',
          credentials: 'include',
          headers: session?.user?.token ? {
            'Authorization': `Bearer ${session.user.token}`,
            'Content-Type': 'application/json'
          } : undefined
        });
        
        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }
        
        return;
      } else {
        // Server-side: use axios with the API URL directly
        const api = axios.create({
          baseURL: env.NEXT_PUBLIC_API_URL,
          headers: { 'Content-Type': 'application/json' },
          withCredentials: true,
        });
        await api.delete(url);
        return;
      }
    } catch (error) {
      console.error(`Error deleting time slot ${timeSlotId} from activity ${activityId}:`, error);
      throw error;
    }
  },
  
  // Get enrolled students for an activity
  getEnrolledStudents: async (activityId: string): Promise<Student[]> => {
    const useProxyApi = typeof window !== 'undefined';
    const url = useProxyApi 
      ? `/api/database/activities/${activityId}/students` 
      : `${env.NEXT_PUBLIC_API_URL}/activities/${activityId}/students`;
    
    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          credentials: 'include',
          headers: session?.user?.token ? {
            'Authorization': `Bearer ${session.user.token}`,
            'Content-Type': 'application/json'
          } : undefined
        });
        
        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }
        
        return await response.json() as Student[];
      } else {
        // Server-side: use axios with the API URL directly
        const api = axios.create({
          baseURL: env.NEXT_PUBLIC_API_URL,
          headers: { 'Content-Type': 'application/json' },
          withCredentials: true,
        });
        const response = await api.get(url);
        return response.data as Student[];
      }
    } catch (error) {
      console.error(`Error fetching enrolled students for activity ${activityId}:`, error);
      throw error;
    }
  },
  
  // Get activities a student is enrolled in
  getStudentActivities: async (studentId: string): Promise<Activity[]> => {
    const useProxyApi = typeof window !== 'undefined';
    const url = useProxyApi 
      ? `/api/database/activities/student/${studentId}/ags` 
      : `${env.NEXT_PUBLIC_API_URL}/activities/student/${studentId}/ags`;
    
    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          credentials: 'include',
          headers: session?.user?.token ? {
            'Authorization': `Bearer ${session.user.token}`,
            'Content-Type': 'application/json'
          } : undefined
        });
        
        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }
        
        return mapActivityResponse(await response.json());
      } else {
        // Server-side: use axios with the API URL directly
        const api = axios.create({
          baseURL: env.NEXT_PUBLIC_API_URL,
          headers: { 'Content-Type': 'application/json' },
          withCredentials: true,
        });
        const response = await api.get(url);
        return mapActivityResponse(response.data);
      }
    } catch (error) {
      console.error(`Error fetching activities for student ${studentId}:`, error);
      throw error;
    }
  },
  
  // Get activities a student can enroll in
  getAvailableActivities: async (studentId: string): Promise<Activity[]> => {
    const useProxyApi = typeof window !== 'undefined';
    const url = useProxyApi 
      ? `/api/database/activities/student/available?student_id=${studentId}` 
      : `${env.NEXT_PUBLIC_API_URL}/activities/student/available?student_id=${studentId}`;
    
    try {
      if (useProxyApi) {
        // Browser environment: use fetch with our Next.js API route
        const session = await getSession();
        const response = await fetch(url, {
          credentials: 'include',
          headers: session?.user?.token ? {
            'Authorization': `Bearer ${session.user.token}`,
            'Content-Type': 'application/json'
          } : undefined
        });
        
        if (!response.ok) {
          const errorText = await response.text();
          console.error(`API error: ${response.status}`, errorText);
          throw new Error(`API error: ${response.status}`);
        }
        
        return mapActivityResponse(await response.json());
      } else {
        // Server-side: use axios with the API URL directly
        const api = axios.create({
          baseURL: env.NEXT_PUBLIC_API_URL,
          headers: { 'Content-Type': 'application/json' },
          withCredentials: true,
        });
        const response = await api.get(url);
        return mapActivityResponse(response.data);
      }
    } catch (error) {
      console.error(`Error fetching available activities for student ${studentId}:`, error);
      throw error;
    }
  }
};