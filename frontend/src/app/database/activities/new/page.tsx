'use client';

import { useSession } from 'next-auth/react';
import { redirect, useRouter } from 'next/navigation';
import { useState, useEffect } from 'react';
import { PageHeader, SectionTitle } from '@/components/dashboard';
import ActivityForm from '@/components/activities/activity-form';
import type { Activity, ActivityCategory } from '@/lib/activity-api';
import { activityService } from '@/lib/activity-api';
import Link from 'next/link';

export default function NewActivityPage() {
  const router = useRouter();
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  // Supervisors should be fetched from an API
  const [supervisors, setSupervisors] = useState<Array<{id: string, name: string}>>([]);
  
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect('/login');
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
        setError('Fehler beim Laden der Kategorien. Bitte versuchen Sie es später erneut.');
      }
    } catch (err) {
      setError('Fehler beim Laden der Kategorien. Bitte versuchen Sie es später erneut.');
    } finally {
      setLoading(false);
    }
  };

  // Handle form submission
  const handleSubmit = async (formData: Partial<Activity>) => {
    try {
      setSaving(true);
      
      // Ensure all required fields are set
      if (!formData.name || !formData.max_participant || !formData.supervisor_id || !formData.ag_category_id) {
        setError('Bitte füllen Sie alle Pflichtfelder aus.');
        return;
      }
      
      // Create a complete activity object with all required fields
      const activityData: Omit<Activity, 'id'> = {
        name: formData.name,
        max_participant: formData.max_participant,
        is_open_ags: formData.is_open_ags ?? false,
        supervisor_id: formData.supervisor_id,
        ag_category_id: formData.ag_category_id,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      };
      
      // Add optional times if present
      if (formData.times) {
        activityData.times = formData.times;
      }
      
      // Create the activity
      const newActivity = await activityService.createActivity(activityData);
      
      // Redirect to the new activity
      router.push(`/database/activities/${newActivity.id}`);
    } catch (err) {
      setError('Fehler beim Erstellen der Aktivität. Bitte versuchen Sie es später erneut.');
      throw err; // Rethrow so the form can handle it
    } finally {
      setSaving(false);
    }
  };

  // Initial data load
  useEffect(() => {
    void fetchCategories();
  }, []);

  if (status === 'loading' || loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen">
      <PageHeader 
        title="Neue Aktivität erstellen"
        backUrl="/database/activities"
      />

      <main className="max-w-4xl mx-auto p-4">
        <div className="mb-8">
          <SectionTitle title="Aktivitätsdetails" />
        </div>

        {error && (
          <div className="mb-6 bg-red-50 border border-red-200 rounded-lg p-4 text-red-800">
            {error}
          </div>
        )}

        <ActivityForm
          initialData={{
            is_open_ags: true,
            max_participant: 10,
          }}
          onSubmitAction={handleSubmit}
          onCancelAction={() => router.push('/database/activities')}
          isLoading={saving}
          formTitle="Neue Aktivität erstellen"
          submitLabel="Aktivität erstellen"
          categories={categories}
          supervisors={supervisors}
        />
        
        <div className="mt-6 text-sm text-gray-500 bg-gray-50 p-4 rounded-lg">
          <p>
            Hinweis: Nach dem Erstellen können Sie zusätzliche Teilnehmer hinzufügen.
          </p>
        </div>
      </main>
    </div>
  );
}