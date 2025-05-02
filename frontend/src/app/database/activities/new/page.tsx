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
  
  // Mock supervisors for demo purposes - in production, these would come from an API
  const [supervisors, setSupervisors] = useState<Array<{id: string, name: string}>>([
    { id: "1", name: "Supervisor 1" },
    { id: "2", name: "Supervisor 2" },
    { id: "3", name: "Supervisor 3" }
  ]);
  
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
        console.error('API error when fetching categories:', apiErr);
        setError('Fehler beim Laden der Kategorien. Bitte versuchen Sie es später erneut.');
      }
    } catch (err) {
      console.error('Error fetching categories:', err);
      setError('Fehler beim Laden der Kategorien. Bitte versuchen Sie es später erneut.');
    } finally {
      setLoading(false);
    }
  };

  // Handle form submission
  const handleSubmit = async (formData: Partial<Activity>) => {
    try {
      setSaving(true);
      
      // Add timestamps
      const activityData = {
        ...formData,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      };
      
      // Create the activity
      const newActivity = await activityService.createActivity(activityData);
      
      // Redirect to the new activity
      router.push(`/database/activities/${newActivity.id}`);
    } catch (err) {
      console.error('Error creating activity:', err);
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