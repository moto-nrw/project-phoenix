// This file contains the Teacher API service and related types

import type { Activity, BackendActivity } from "./activity-helpers";
import { mapActivityResponse } from "./activity-helpers";

// Define Teacher interface aligned with staff API response structure
export interface Teacher {
    id: string;
    name: string;
    first_name: string;
    last_name: string;
    email?: string;  // Email address for authentication
    specialization: string;
    role?: string | null;
    qualifications?: string | null;
    tag_id?: string | null;
    staff_notes?: string | null;
    created_at?: string;
    updated_at?: string;
    activities?: Activity[];
    // Optional fields from staff API for consistency
    person_id?: number;
    is_teacher?: boolean;
}

export interface TeacherWithCredentials extends Teacher {
    temporaryCredentials?: {
        email: string;
        password: string;
    };
}

// Teacher service with API methods
class TeacherService {
    // Get all teachers with optional filters
    async getTeachers(filters?: { search?: string }): Promise<Teacher[]> {
        try {
            let url = "/api/staff?teachers_only=true";

            // Add query parameters if filters are provided
            if (filters) {
                const params = new URLSearchParams();
                if (filters.search) {
                    params.append("search", filters.search);
                }

                if (params.toString()) {
                    url += `&${params.toString()}`;
                }
            }

            const response = await fetch(url);
            if (!response.ok) {
                throw new Error(`Failed to fetch teachers: ${response.statusText}`);
            }

            const data = await response.json();
            
            // Handle different response formats
            if (Array.isArray(data)) {
                return data as Teacher[];
            } else if (data && Array.isArray(data.data)) {
                return data.data as Teacher[];
            } else {
                console.error("Unexpected response format:", data);
                return [];
            }
        } catch (error) {
            console.error("Error fetching teachers:", error);
            throw error;
        }
    }

    // Get a single teacher by ID
    async getTeacher(id: string): Promise<Teacher> {
        try {
            const response = await fetch(`/api/staff/${id}`);
            if (!response.ok) {
                throw new Error(`Failed to fetch teacher: ${response.statusText}`);
            }

            const data = await response.json() as Teacher;
            return data;
        } catch (error) {
            console.error(`Error fetching teacher with ID ${id}:`, error);
            throw error;
        }
    }

    // Create a new teacher
    async createTeacher(teacherData: Omit<Teacher, "id" | "name" | "created_at" | "updated_at"> & { password?: string }): Promise<TeacherWithCredentials> {
        try {
            // Use provided password
            const password = teacherData.password;
            if (!password) {
                throw new Error("Password is required for creating a teacher");
            }
            
            // First create an account for the teacher
            const email = teacherData.email || `${teacherData.first_name.toLowerCase()}.${teacherData.last_name.toLowerCase()}@school.local`;
            const username = `${teacherData.first_name.toLowerCase()}_${teacherData.last_name.toLowerCase()}`;
            
            const accountResponse = await fetch("/api/auth/register", {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({
                    email: email,
                    username: username,
                    name: `${teacherData.first_name} ${teacherData.last_name}`,
                    password: password,
                    confirm_password: password,
                }),
            });

            if (!accountResponse.ok) {
                const errorData = await accountResponse.json() as { error?: string, message?: string };
                const errorMessage = errorData.error || errorData.message || accountResponse.statusText;
                throw new Error(`Failed to create account: ${errorMessage}`);
            }

            const accountData = await accountResponse.json();

            // Then create a person linked to that account
            const personResponse = await fetch("/api/users", {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({
                    first_name: teacherData.first_name,
                    last_name: teacherData.last_name,
                    tag_id: teacherData.tag_id || null,
                    account_id: accountData.data.id, // Link to the created account
                }),
            });

            if (!personResponse.ok) {
                const errorData = await personResponse.json() as { error?: string, message?: string };
                const errorMessage = errorData.error || errorData.message || personResponse.statusText;
                throw new Error(`Failed to create person: ${errorMessage}`);
            }

            const personData = await personResponse.json();
            
            // Extract the person ID - handle different response formats
            const personId = personData.id || personData.data?.id;
            
            if (!personId) {
                console.error("Unexpected person response format:", personData);
                throw new Error("Failed to get person ID from response");
            }

            // Then create staff with is_teacher flag
            const staffData = {
                person_id: personId,
                staff_notes: teacherData.staff_notes || null,
                is_teacher: true,
                specialization: teacherData.specialization,
                role: teacherData.role || null,
                qualifications: teacherData.qualifications || null,
            };

            const response = await fetch("/api/staff", {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify(staffData),
            });

            if (!response.ok) {
                throw new Error(`Failed to create teacher: ${response.statusText}`);
            }

            const data = await response.json() as Teacher;
            
            // Add the temporary credentials to the response
            return {
                ...data,
                temporaryCredentials: {
                    email: email,
                    password: password,
                }
            } as TeacherWithCredentials;
        } catch (error) {
            console.error("Error creating teacher:", error);
            throw error;
        }
    }

    // Update an existing teacher
    async updateTeacher(id: string, teacherData: Partial<Teacher>): Promise<Teacher> {
        try {
            // For staff API, we need to include is_teacher flag
            const staffData = {
                ...teacherData,
                is_teacher: true,
            };

            const response = await fetch(`/api/staff/${id}`, {
                method: "PUT",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify(staffData),
            });

            if (!response.ok) {
                throw new Error(`Failed to update teacher: ${response.statusText}`);
            }

            const data = await response.json() as Teacher;
            return data;
        } catch (error) {
            console.error(`Error updating teacher with ID ${id}:`, error);
            throw error;
        }
    }

    // Delete a teacher
    async deleteTeacher(id: string): Promise<void> {
        try {
            const response = await fetch(`/api/staff/${id}`, {
                method: "DELETE",
            });

            if (!response.ok) {
                throw new Error(`Failed to delete teacher: ${response.statusText}`);
            }
        } catch (error) {
            console.error(`Error deleting teacher with ID ${id}:`, error);
            throw error;
        }
    }

    // Get activities for a teacher
    async getTeacherActivities(id: string): Promise<Activity[]> {
        try {
            // For now, activities endpoint is not implemented for staff
            // Return empty array until implemented on the backend
            console.warn(`Activities endpoint not implemented for staff/teachers`);
            return [];
        } catch (error) {
            console.error(`Error fetching activities for teacher with ID ${id}:`, error);
            throw error;
        }
    }
}

export const teacherService = new TeacherService();