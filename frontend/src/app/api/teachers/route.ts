import { type NextRequest } from "next/server";
import { createPostHandler } from "@/lib/route-wrapper";
import { env } from "@/env";

// Extend the Teacher type to include password for creation
interface TeacherCreationData {
  first_name: string;
  last_name: string;
  email: string;
  password: string;
  specialization?: string | null;
  role?: string | null;
  qualifications?: string | null;
  tag_id?: string | null;
  staff_notes?: string | null;
}

// API response types
interface AccountResponse {
  data: {
    id: string;
    email: string;
    username: string;
    name: string;
  };
}

interface PersonResponse {
  data: {
    id: number;
    first_name: string;
    last_name: string;
    account_id: number;
    tag_id?: string;
    created_at: string;
    updated_at: string;
  };
}

interface StaffResponse {
  data: {
    id: number;
    person_id: number;
    staff_notes?: string;
    is_teacher: boolean;
    teacher_id?: number;
    specialization?: string;
    role?: string;
    qualifications?: string;
    created_at: string;
    updated_at: string;
  };
}

// Create a new teacher with account
export const POST = createPostHandler(
  async (req: NextRequest, body: TeacherCreationData, token: string) => {
    try {
      // Step 1: Create an account via backend API
      const accountResponse = await fetch(
        `${env.NEXT_PUBLIC_API_URL}/auth/register`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            email: body.email,
            username: body.email.split("@")[0], // Use email prefix as username
            name: `${body.first_name} ${body.last_name}`,
            password: body.password,
            confirm_password: body.password,
          }),
        },
      );

      if (!accountResponse.ok) {
        const error = await accountResponse.text();
        throw new Error(`Account creation failed: ${error}`);
      }

      const accountResult = (await accountResponse.json()) as AccountResponse;
      const account = accountResult.data;

      // Step 2: Create a person linked to this account via backend API
      const personResponse = await fetch(
        `${env.NEXT_PUBLIC_API_URL}/api/users`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({
            first_name: body.first_name,
            last_name: body.last_name,
            account_id: Number.parseInt(account.id, 10),
            tag_id: body.tag_id ?? undefined,
          }),
        },
      );

      if (!personResponse.ok) {
        const error = await personResponse.text();
        throw new Error(`Person creation failed: ${error}`);
      }

      const personResult = (await personResponse.json()) as PersonResponse;
      const person = personResult.data;

      // Step 3: Create a staff record linked to this person
      const trimmedStaffNotes = body.staff_notes?.trim();
      const trimmedSpecialization = body.specialization?.trim();
      const trimmedRole = body.role?.trim();
      const trimmedQualifications = body.qualifications?.trim();

      const staffResponse = await fetch(
        `${env.NEXT_PUBLIC_API_URL}/api/staff`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            Authorization: `Bearer ${token}`,
          },
          body: JSON.stringify({
            person_id: person.id,
            staff_notes:
              trimmedStaffNotes && trimmedStaffNotes.length > 0
                ? trimmedStaffNotes
                : undefined,
            is_teacher: true,
            specialization:
              trimmedSpecialization && trimmedSpecialization.length > 0
                ? trimmedSpecialization
                : undefined,
            role:
              trimmedRole && trimmedRole.length > 0 ? trimmedRole : undefined,
            qualifications:
              trimmedQualifications && trimmedQualifications.length > 0
                ? trimmedQualifications
                : undefined,
          }),
        },
      );

      if (!staffResponse.ok) {
        const error = await staffResponse.text();
        throw new Error(`Staff creation failed: ${error}`);
      }

      const staffResult = (await staffResponse.json()) as StaffResponse;
      const staff = staffResult.data;

      // Map the response to match the expected Teacher interface
      const teacher = {
        id: staff.id.toString(),
        name: `${person.first_name} ${person.last_name}`,
        first_name: person.first_name,
        last_name: person.last_name,
        email: account.email,
        specialization: staff.specialization ?? null,
        role: staff.role ?? null,
        qualifications: staff.qualifications ?? null,
        tag_id: person.tag_id,
        staff_notes: staff.staff_notes,
        created_at: staff.created_at,
        updated_at: staff.updated_at,
        person_id: staff.person_id,
        is_teacher: true,
        teacher_id: staff.teacher_id,
      };

      return {
        success: true,
        message: "Teacher created successfully",
        data: teacher,
      };
    } catch (error) {
      console.error("Error creating teacher:", error);
      throw error instanceof Error
        ? error
        : new Error("Failed to create teacher");
    }
  },
);
