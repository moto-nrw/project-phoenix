"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { useState, useEffect } from "react";
import { PageHeader, SectionTitle } from "@/components/dashboard";
import ActivityForm from "@/components/activities/activity-form";
import type { Activity, ActivityCategory } from "@/lib/activity-api";
import { activityService } from "@/lib/activity-service";
import { teacherService } from "@/lib/teacher-api";
// import Link from 'next/link';

export default function NewActivityPage() {
  const router = useRouter();
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Fetch supervisors from API
  const [supervisors, setSupervisors] = useState<
    Array<{ id: string; name: string }>
  >([]);
  const [, setSupervisorsLoading] = useState<boolean>(true);

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  // Function to fetch categories
  const fetchCategories = async () => {
    try {
      setLoading(true);

      try {
        // Fetch categories
        const categoriesData = await activityService.getCategories();
        setCategories(categoriesData);

        setError(null);
      } catch (apiErr) {
        console.error("API error fetching categories:", apiErr);
        setError(
          "Fehler beim Laden der Kategorien. Bitte versuchen Sie es später erneut.",
        );
      }
    } catch {
      setError(
        "Fehler beim Laden der Kategorien. Bitte versuchen Sie es später erneut.",
      );
    } finally {
      setLoading(false);
    }
  };

  // Function to fetch supervisors
  const fetchSupervisors = async () => {
    try {
      setSupervisorsLoading(true);

      // Fetch supervisors from teacher service (same as groups/new)
      const teachersData = await teacherService.getTeachers();
      
      // Convert teachers to supervisors format
      const supervisorsData = teachersData.map(teacher => ({
        id: teacher.id,
        name: teacher.name
      }));
      
      setSupervisors(supervisorsData);
    } catch (err) {
      console.error("Error fetching supervisors:", err);
      // Don't set an error state that would block the UI, just log it
      setSupervisors([]); // Set empty array to prevent UI issues
    } finally {
      setSupervisorsLoading(false);
    }
  };

  // Handle form submission
  const handleSubmit = async (formData: Partial<Activity> & { schedules?: Array<{ weekday: string; timeframe_id?: number }> }) => {
    try {
      setSaving(true);

      // Ensure all required fields are set
      if (
        !formData.name ||
        !formData.max_participant ||
        !formData.supervisor_id ||
        !formData.ag_category_id
      ) {
        setError("Bitte füllen Sie alle Pflichtfelder aus.");
        return;
      }

      // Create a complete activity object with all required fields
      const activityData: Omit<Activity, "id"> = {
        name: formData.name,
        max_participant: formData.max_participant,
        is_open_ags: formData.is_open_ags ?? false,
        supervisor_id: formData.supervisor_id,
        ag_category_id: formData.ag_category_id,
        created_at: new Date(),
        updated_at: new Date(),
      };

      // Add optional times if present
      if (formData.times) {
        activityData.times = formData.times;
      }

      // Create the activity - convert from Activity type to CreateActivityRequest type
      const createRequest = {
        name: activityData.name,
        max_participants: activityData.max_participant,
        is_open: activityData.is_open_ags,
        category_id: parseInt(activityData.ag_category_id, 10),
        planned_room_id: formData.planned_room_id ? parseInt(formData.planned_room_id, 10) : undefined,
        supervisor_ids: activityData.supervisor_id ? [parseInt(activityData.supervisor_id, 10)] : [],
        // Include schedules from form data if present, converting weekday to number
        schedules: formData.schedules?.map(schedule => ({
          ...schedule,
          weekday: parseInt(schedule.weekday, 10)
        })) ?? []
      };
      
      const newActivity = await activityService.createActivity(createRequest);

      // Redirect to the new activity
      router.push(`/database/activities/${newActivity.id}`);
    } catch (err) {
      setError(
        "Fehler beim Erstellen der Aktivität. Bitte versuchen Sie es später erneut.",
      );
      throw err; // Rethrow so the form can handle it
    } finally {
      setSaving(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchCategories();
    void fetchSupervisors();
  }, []);

  if (status === "loading" || loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    );
  }

  // We don't need to block the entire UI on supervisors loading,
  // they will just show as loading in the select component

  return (
    <div className="min-h-screen">
      <PageHeader
        title="Neue Aktivität erstellen"
        backUrl="/database/activities"
      />

      <main className="mx-auto max-w-4xl p-4">
        <div className="mb-8">
          <SectionTitle title="Aktivitätsdetails" />
        </div>

        {error && (
          <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4 text-red-800">
            {error}
          </div>
        )}

        <ActivityForm
          initialData={{
            is_open_ags: true,
            max_participant: 10,
          }}
          onSubmitAction={handleSubmit}
          onCancelAction={() => router.push("/database/activities")}
          isLoading={saving}
          formTitle="Neue Aktivität erstellen"
          submitLabel="Aktivität erstellen"
          categories={categories}
          supervisors={supervisors}
        />

        <div className="mt-6 rounded-lg bg-gray-50 p-4 text-sm text-gray-500">
          <p>
            Hinweis: Nach dem Erstellen können Sie zusätzliche Teilnehmer
            hinzufügen.
          </p>
        </div>
      </main>
    </div>
  );
}
