import { NextRequest, NextResponse } from "next/server";
import { RouteWrapper } from "@/lib/route-wrapper";
import { authService } from "@/lib/auth-service";
import { env } from "@/env";
import { getServerSession } from "next-auth";
import { authOptions } from "@/server/auth/config";

// Extend the Teacher type to include password for creation
interface TeacherCreationData {
    first_name: string;
    last_name: string;
    email: string;
    password: string;
    specialization: string;
    role?: string | null;
    qualifications?: string | null;
    tag_id?: string | null;
    staff_notes?: string | null;
}

// Create a new teacher with account
export const POST = RouteWrapper(async (req: NextRequest) => {
    const session = await getServerSession(authOptions);
    
    // Verify user has permission to create teachers
    if (!session?.user?.token) {
        return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    const data = await req.json() as TeacherCreationData;

    try {
        // Step 1: Create an account via backend API
        const accountResponse = await fetch(`${env.NEXT_PUBLIC_API_URL}/auth/register`, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
            },
            body: JSON.stringify({
                email: data.email,
                username: data.email.split('@')[0], // Use email prefix as username
                name: `${data.first_name} ${data.last_name}`,
                password: data.password,
                confirm_password: data.password
            }),
        });

        if (!accountResponse.ok) {
            const error = await accountResponse.text();
            throw new Error(`Account creation failed: ${error}`);
        }

        const accountResult = await accountResponse.json();
        const account = accountResult.data;

        // Step 2: Create a person linked to this account via backend API
        const personResponse = await fetch(`${env.NEXT_PUBLIC_API_URL}/api/users`, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                Authorization: `Bearer ${session.user.token}`,
            },
            body: JSON.stringify({
                first_name: data.first_name,
                last_name: data.last_name,
                account_id: parseInt(account.id),
                tag_id: data.tag_id || undefined,
            }),
        });

        if (!personResponse.ok) {
            const error = await personResponse.text();
            throw new Error(`Person creation failed: ${error}`);
        }

        const personResult = await personResponse.json();
        const person = personResult.data;

        // Step 3: Create a staff record linked to this person
        const staffResponse = await fetch(`${env.NEXT_PUBLIC_API_URL}/api/staff`, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                Authorization: `Bearer ${session.user.token}`,
            },
            body: JSON.stringify({
                person_id: person.id,
                staff_notes: data.staff_notes || "",
                is_teacher: true,
                specialization: data.specialization,
                role: data.role || "",
                qualifications: data.qualifications || "",
            }),
        });

        if (!staffResponse.ok) {
            const error = await staffResponse.text();
            throw new Error(`Staff creation failed: ${error}`);
        }

        const staffResult = await staffResponse.json();
        const staff = staffResult.data;

        // Map the response to match the expected Teacher interface
        const teacher = {
            id: staff.id.toString(),
            name: `${person.first_name} ${person.last_name}`,
            first_name: person.first_name,
            last_name: person.last_name,
            email: account.email,
            specialization: staff.specialization,
            role: staff.role,
            qualifications: staff.qualifications,
            tag_id: person.tag_id,
            staff_notes: staff.staff_notes,
            created_at: staff.created_at,
            updated_at: staff.updated_at,
            person_id: staff.person_id,
            is_teacher: true,
            teacher_id: staff.teacher_id,
        };

        return NextResponse.json({
            success: true,
            message: "Teacher created successfully",
            data: teacher,
        });
    } catch (error) {
        console.error('Error creating teacher:', error);
        return NextResponse.json(
            { success: false, message: error instanceof Error ? error.message : 'Failed to create teacher' },
            { status: 500 }
        );
    }
});