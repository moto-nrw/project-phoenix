'use client';

import { useSession } from 'next-auth/react';
import { redirect, useRouter, useParams } from 'next/navigation';
import { useState, useEffect } from 'react';
import { PageHeader, SectionTitle } from '@/components/dashboard';
import ActivityForm from '@/components/activities/activity-form';
import type { Activity, ActivityCategory } from '@/lib/activity-api';
import { activityService } from '@/lib/activity-api';
import Link from 'next/link';

export default function EditActivityPage() {
  const router = useRouter();
  const params = useParams();
  const { id } = params;
  const [activity, setActivity] = useState<Activity | null>(null);
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  const [supervisors, setSupervisors] = useState<Array<{id: string, name: string}>>([]);
  
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect('/login');
    },
  });

  // Function to fetch the activity details and categories
  const fetchData = async () => {
    if (!id) return;

    try {
      setLoading(true);
      
      try {
        // Fetch activity from API
        const activityData = await activityService.getActivity(id as string);
        setActivity(activityData);
        
        // Fetch categories
        const categoriesData = await activityService.getCategories();
        setCategories(categoriesData);
        
        // Fetch all supervisors from API
        const response = await fetch('/api/users/supervisors');
        if (!response.ok) {
          throw new Error(`Failed to fetch supervisors: ${response.statusText}`);
        }
        const supervisorsData = await response.json();
        setSupervisors(supervisorsData);
        
        setError(null);
      } catch (apiErr) {
          setError('Fehler beim Laden der Daten. Bitte versuchen Sie es später erneut.');
        setActivity(null);
      }
    } catch (err) {
      setError('Fehler beim Laden der Daten. Bitte versuchen Sie es später erneut.');
      setActivity(null);
    } finally {
      setLoading(false);
    }
  };

  // Handle form submission
  const handleSubmit = async (formData: Partial<Activity>) => {
    if (!id || !activity) return;
    
    try {
      setSaving(true);
      
      // Ensure category ID is included from original activity if not in form data
      const dataToSubmit: Partial<Activity> = {
        ...formData
      };

      // Make sure we preserve the category ID if it's not in formData but exists in original activity
      if (!dataToSubmit.ag_category_id && activity.ag_category_id) {
        console.log('Adding missing ag_category_id from original activity:', activity.ag_category_id);
        dataToSubmit.ag_category_id = activity.ag_category_id;
      }
      
      // Update the activity
      await activityService.updateActivity(id as string, dataToSubmit);
      
      // Redirect back to activity details
      router.push(`/database/activities/${id}`);
    } catch (err) {
      setError('Fehler beim Speichern der Aktivität. Bitte versuchen Sie es später erneut.');
      throw err; // Rethrow so the form can handle it
    } finally {
      setSaving(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchData();
  }, [id]);

  if (status === 'loading' || loading) {
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
        <div className="bg-red-50 text-red-800 p-4 rounded-lg max-w-md">
          <h2 className="font-semibold mb-2">Fehler</h2>
          <p>{error}</p>
          <button 
            onClick={() => fetchData()} 
            className="mt-4 px-4 py-2 bg-red-100 hover:bg-red-200 text-red-800 rounded transition-colors"
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
        <div className="bg-orange-50 text-orange-800 p-4 rounded-lg max-w-md">
          <h2 className="font-semibold mb-2">Aktivität nicht gefunden</h2>
          <p>Die angeforderte Aktivität konnte nicht gefunden werden.</p>
          <Link href="/database/activities">
            <button className="mt-4 px-4 py-2 bg-orange-100 hover:bg-orange-200 text-orange-800 rounded transition-colors">
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

      <main className="max-w-4xl mx-auto p-4">
        <div className="mb-8">
          <SectionTitle title="Aktivitätsdetails bearbeiten" />
        </div>

        <ActivityForm
          initialData={activity}
          onSubmitAction={handleSubmit}
          onCancelAction={() => router.push(`/database/activities/${activity.id}`)}
          isLoading={saving}
          formTitle="Aktivität bearbeiten"
          submitLabel="Änderungen speichern"
          categories={categories}
          supervisors={supervisors}
        />
      </main>
    </div>
  );
}