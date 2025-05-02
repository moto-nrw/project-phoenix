'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { activityService } from '~/lib/activity-api';
import type { ActivityCategory } from '~/lib/activity-api';
import { ActivityForm } from '~/components/activities';
import { PageHeader } from '~/components/dashboard';

export default function NewActivityPage() {
  const [categories, setCategories] = useState<ActivityCategory[]>([]);
  const [supervisors, setSupervisors] = useState<Array<{id: string, name: string}>>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  
  const router = useRouter();

  useEffect(() => {
    const fetchData = async () => {
      try {
        setLoading(true);
        const categoriesData = await activityService.getCategories();
        setCategories(categoriesData);
        
        // TODO: Replace with actual API call to fetch supervisors
        // For now, just use dummy data
        setSupervisors([
          { id: '1', name: 'Supervisor 1' },
          { id: '2', name: 'Supervisor 2' }
        ]);
        
        setError(null);
      } catch (err) {
        console.error('Error fetching categories:', err);
        setError('Fehler beim Laden der Kategorien. Bitte versuchen Sie es später erneut.');
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, []);

  const handleCreateActivity = async (activityData: any) => {
    try {
      setIsSubmitting(true);
      
      const newActivity = await activityService.createActivity(activityData);
      router.push(`/activities/${newActivity.id}`);
    } catch (err) {
      console.error('Error creating activity:', err);
      setError('Fehler beim Erstellen der Aktivität. Bitte versuchen Sie es später erneut.');
    } finally {
      setIsSubmitting(false);
    }
  };

  if (loading) {
    return (
      <div className="min-h-screen">
        <PageHeader 
          title="Neue Aktivität erstellen" 
          description="Daten werden geladen"
          backUrl="/activities"
        />
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
          <div className="text-center py-8">
            <p className="text-gray-500">Daten werden geladen...</p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen">
      {/* Header */}
      <PageHeader 
        title="Neue Aktivität erstellen" 
        description="Erstellen Sie eine neue Aktivitätsgruppe"
        backUrl="/activities"
      />
      
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        
        {error && (
          <div className="bg-red-50 text-red-800 p-4 rounded-lg mb-6">
            <p>{error}</p>
          </div>
        )}
        
        <ActivityForm
          onSubmitAction={handleCreateActivity}
          onCancelAction={() => router.push('/activities')}
          isLoading={isSubmitting}
          formTitle="Neue Aktivität erstellen"
          submitLabel="Erstellen"
          categories={categories}
          supervisors={supervisors}
        />
      </div>
    </div>
  );
}