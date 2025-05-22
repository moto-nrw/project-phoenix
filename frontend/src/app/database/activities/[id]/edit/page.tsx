"use client";

import { useSession } from "next-auth/react";
import { redirect, useRouter, useParams } from "next/navigation";
import { useState, useEffect, useCallback } from "react";
import { PageHeader, SectionTitle } from "@/components/dashboard";
import ActivityForm from "@/components/activities/activity-form";
import type { Activity, ActivityCategory } from "@/lib/activity-helpers";
import { activityService } from "@/lib/activity-service";
import { teacherService } from "@/lib/teacher-api";
import Link from "next/link";

export default function EditActivityPage() {
  const router = useRouter();
  const params = useParams();
  const { id } = params;
  const [activity, setActivity] = useState<Activity | null>(null);
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [supervisors, setSupervisors] = useState<
    Array<{ id: string; name: string }>
  >([]);

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  // Function to fetch the activity details and categories
  const fetchData = useCallback(async () => {
    if (!id) return;

    try {
      setLoading(true);

      try {
        
        // Fetch all data in parallel for faster loading
        const [activityData, categoriesData, teachersData] = await Promise.all([
          activityService.getActivity(id as string),
          activityService.getCategories(),
          teacherService.getTeachers()
        ]);
        
        
        // Convert teachers to supervisors format
        const supervisorsData = teachersData.map(teacher => ({
          id: teacher.id,
          name: teacher.name
        }));
        
        setSupervisors(supervisorsData);
        
        setActivity(activityData);
        setCategories(categoriesData);

        setError(null);
      } catch (error) {
        console.error("Error loading activity data:", error);
        setError(
          "Fehler beim Laden der Daten. Bitte versuchen Sie es später erneut.",
        );
        setActivity(null);
        
        // Don't let this error prevent the UI from loading
        setSupervisors([]);
        setCategories([]);
      }
    } catch (error) {
      console.error("Outer error loading data:", error);
      setError(
        "Fehler beim Laden der Daten. Bitte versuchen Sie es später erneut.",
      );
      setActivity(null);
    } finally {
      setLoading(false);
    }
  }, [id]);

  // Handle form submission
  const handleSubmit = async (formData: Partial<Activity> & { schedules?: Array<{ weekday: string; timeframe_id?: number }> }) => {
    if (!id || !activity) return;

    try {
      setSaving(true);

      // Ensure category ID is included from original activity if not in form data
      const dataToSubmit: Partial<Activity> = {
        ...formData,
      };

      // Make sure we preserve the category ID if it's not in formData but exists in original activity
      if (!dataToSubmit.ag_category_id && activity.ag_category_id) {
        dataToSubmit.ag_category_id = activity.ag_category_id;
      }

      // Update the activity - convert from Activity type to UpdateActivityRequest type
      const updateRequest = {
        name: dataToSubmit.name ?? '',
        max_participants: dataToSubmit.max_participant ?? 0,
        is_open: dataToSubmit.is_open_ags ?? false,
        category_id: parseInt(dataToSubmit.ag_category_id ?? '0', 10),
        planned_room_id: dataToSubmit.planned_room_id ? parseInt(dataToSubmit.planned_room_id, 10) : undefined,
        supervisor_ids: dataToSubmit.supervisor_id ? [parseInt(dataToSubmit.supervisor_id, 10)] : [],
        // Include schedules from form data if present
        schedules: formData.schedules ?? []
      };
      
      await activityService.updateActivity(id as string, updateRequest);

      // Redirect back to activity details
      router.push(`/database/activities/${id as string}`);
    } catch (err) {
      setError(
        "Fehler beim Speichern der Aktivität. Bitte versuchen Sie es später erneut.",
      );
      throw err; // Rethrow so the form can handle it
    } finally {
      setSaving(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchData();
  }, [id, fetchData]);

  if (status === "loading" || loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    );
  }

  // Show error if loading failed
  if (error) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <div className="max-w-md rounded-lg bg-red-50 p-4 text-red-800">
          <h2 className="mb-2 font-semibold">Fehler</h2>
          <p>{error}</p>
          <button
            onClick={() => fetchData()}
            className="mt-4 rounded bg-red-100 px-4 py-2 text-red-800 transition-colors hover:bg-red-200"
          >
            Erneut versuchen
          </button>
        </div>
      </div>
    );
  }

  if (!activity) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <div className="max-w-md rounded-lg bg-orange-50 p-4 text-orange-800">
          <h2 className="mb-2 font-semibold">Aktivität nicht gefunden</h2>
          <p>Die angeforderte Aktivität konnte nicht gefunden werden.</p>
          <Link href="/database/activities">
            <button className="mt-4 rounded bg-orange-100 px-4 py-2 text-orange-800 transition-colors hover:bg-orange-200">
              Zurück zur Übersicht
            </button>
          </Link>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen">
      <PageHeader
        title={`Aktivität bearbeiten: ${activity.name}`}
        backUrl={`/database/activities/${activity.id}`}
      />

      <main className="mx-auto max-w-4xl p-4">
        <div className="mb-8">
          <SectionTitle title="Aktivitätsdetails bearbeiten" />
        </div>

        <ActivityForm
          initialData={activity}
          onSubmitAction={handleSubmit}
          onCancelAction={() =>
            router.push(`/database/activities/${activity.id}`)
          }
          isLoading={saving}
          formTitle="Aktivität bearbeiten"
          submitLabel="Änderungen speichern"
          categories={categories}
          supervisors={supervisors} // Already passing supervisors here
        />
      </main>
    </div>
  );
}
