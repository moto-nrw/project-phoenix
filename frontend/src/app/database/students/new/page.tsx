'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { PageHeader } from '@/components/dashboard';
import StudentForm from '@/components/students/student-form';
import type { Student } from '@/lib/api';
import { studentService } from '@/lib/api';

export default function NewStudentPage() {
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleCreateStudent = async (studentData: Partial<Student>) => {
    try {
      setLoading(true);
      setError(null);
      
      // Prepare student data
      const newStudent: Omit<Student, 'id'> = {
        ...studentData,
        name: `${studentData.first_name} ${studentData.second_name}`,
        in_house: studentData.in_house || false,
        wc: studentData.wc || false,
        school_yard: studentData.school_yard || false,
        bus: studentData.bus || false,
        school_class: studentData.school_class || '',
      };
      
      // Create student
      await studentService.createStudent(newStudent);
      
      // Navigate back to students list on success
      router.push('/database/students');
    } catch (err) {
      console.error('Error creating student:', err);
      setError('Fehler beim Erstellen des Schülers. Bitte versuchen Sie es später erneut.');
      throw err; // Re-throw to be caught by the form component
    } finally {
      setLoading(false);
    }
  };
  
  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <PageHeader 
        title="Neuer Schüler"
        backUrl="/database/students"
      />
      
      {/* Main Content */}
      <main className="max-w-4xl mx-auto p-4">
        {error && (
          <div className="bg-red-50 text-red-800 p-4 rounded-lg mb-6">
            <p>{error}</p>
          </div>
        )}
        
        <StudentForm
          initialData={{ 
            in_house: false,
            wc: false,
            school_yard: false,
            bus: false,
            group_id: '1',
          }}
          onSubmit={handleCreateStudent}
          onCancel={() => router.back()}
          isLoading={loading}
          formTitle="Schüler erstellen"
          submitLabel="Erstellen"
        />
      </main>
    </div>
  );
}