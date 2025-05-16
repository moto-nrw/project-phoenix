// This file contains the Teacher API service and related types

// Define Teacher interface aligned with database structure
export interface Teacher {
    id: string;
    name: string;
    first_name: string;
    last_name: string;
    specialization: string;
    role?: string | null;
    qualifications?: string | null;
    tag_id?: string | null;
    staff_notes?: string | null;
    created_at?: string;
    updated_at?: string;
    activities?: any[]; // Using any[] to avoid circular dependencies
}

// Teacher service with API methods
class TeacherService {
    // Get all teachers with optional filters
    async getTeachers(filters?: { search?: string }): Promise<Teacher[]> {
        try {
            let url = "/api/teachers";

            // Add query parameters if filters are provided
            if (filters) {
                const params = new URLSearchParams();
                if (filters.search) {
                    params.append("search", filters.search);
                }

                if (params.toString()) {
                    url += `?${params.toString()}`;
                }
            }

            const response = await fetch(url);
            if (!response.ok) {
                throw new Error(`Failed to fetch teachers: ${response.statusText}`);
            }

            const data = await response.json();
            return data as Teacher[];
        } catch (error) {
            console.error("Error fetching teachers:", error);
            throw error;
        }
    }

    // Get a single teacher by ID
    async getTeacher(id: string): Promise<Teacher> {
        try {
            const response = await fetch(`/api/teachers/${id}`);
            if (!response.ok) {
                throw new Error(`Failed to fetch teacher: ${response.statusText}`);
            }

            const data = await response.json();
            return data as Teacher;
        } catch (error) {
            console.error(`Error fetching teacher with ID ${id}:`, error);
            throw error;
        }
    }

    // Create a new teacher
    async createTeacher(teacherData: Omit<Teacher, "id" | "name" | "created_at" | "updated_at">): Promise<Teacher> {
        try {
            const response = await fetch("/api/teachers", {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify(teacherData),
            });

            if (!response.ok) {
                throw new Error(`Failed to create teacher: ${response.statusText}`);
            }

            const data = await response.json();
            return data as Teacher;
        } catch (error) {
            console.error("Error creating teacher:", error);
            throw error;
        }
    }

    // Update an existing teacher
    async updateTeacher(id: string, teacherData: Partial<Teacher>): Promise<Teacher> {
        try {
            const response = await fetch(`/api/teachers/${id}`, {
                method: "PUT",
                headers: {
                    "Content-Type": "application/json",
                },
                body: JSON.stringify(teacherData),
            });

            if (!response.ok) {
                throw new Error(`Failed to update teacher: ${response.statusText}`);
            }

            const data = await response.json();
            return data as Teacher;
        } catch (error) {
            console.error(`Error updating teacher with ID ${id}:`, error);
            throw error;
        }
    }

    // Delete a teacher
    async deleteTeacher(id: string): Promise<void> {
        try {
            const response = await fetch(`/api/teachers/${id}`, {
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
    async getTeacherActivities(id: string): Promise<any[]> {
        try {
            const response = await fetch(`/api/teachers/${id}/activities`);
            if (!response.ok) {
                throw new Error(`Failed to fetch teacher activities: ${response.statusText}`);
            }

            const data = await response.json();
            return data;
        } catch (error) {
            console.error(`Error fetching activities for teacher with ID ${id}:`, error);
            throw error;
        }
    }
}

export const teacherService = new TeacherService();