'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { PageHeader } from '@/components/dashboard';
import { GroupForm } from '@/components/groups';
import type { Group } from '@/lib/api';
import { groupService } from '@/lib/api';

export default function NewGroupPage() {
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleCreateGroup = async (groupData: Partial<Group>) => {
    try {
      setLoading(true);
      setError(null);
      
      // Prepare group data
      const newGroup: Omit<Group, 'id'> = {
        ...groupData,
        name: groupData.name ?? '',
      };
      
      // Create group
      await groupService.createGroup(newGroup);
      
      // Navigate back to groups list on success
      router.push('/database/groups');
    } catch (err) {
      console.error('Error creating group:', err);
      setError('Fehler beim Erstellen der Gruppe. Bitte versuchen Sie es später erneut.');
      throw err; // Re-throw to be caught by the form component
    } finally {
      setLoading(false);
    }
  };
  
  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <PageHeader 
        title="Neue Gruppe"
        backUrl="/database/groups"
      />
      
      {/* Main Content */}
      <main className="max-w-4xl mx-auto p-4">
        {error && (
          <div className="bg-red-50 text-red-800 p-4 rounded-lg mb-6">
            <p>{error}</p>
          </div>
        )}
        
        <GroupForm
          initialData={{ 
            name: '',
            room_id: '',
            representative_id: '',
          }}
          onSubmitAction={handleCreateGroup}
          onCancelAction={() => router.back()}
          isLoading={loading}
          formTitle="Gruppe erstellen"
          submitLabel="Erstellen"
        />
      </main>
    </div>
  );
}