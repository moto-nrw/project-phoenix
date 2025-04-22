'use client';

import { useEffect, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { PageHeader } from '@/components/dashboard';
import type { Student } from '@/lib/api';
import { studentService } from '@/lib/api';

// Mock student data for development
const mockStudents: Student[] = [
  { id: '1', name: 'Anna Müller', school_class: '1A', grade: '1A', studentId: 'ST001', in_house: true },
  { id: '2', name: 'Max Schmidt', school_class: '1A', grade: '1A', studentId: 'ST002', in_house: false },
  { id: '3', name: 'Sophie Weber', school_class: '2B', grade: '2B', studentId: 'ST003', in_house: true },
  { id: '4', name: 'Lena Fischer', school_class: '2B', grade: '2B', studentId: 'ST004', in_house: false },
  { id: '5', name: 'Noah Meyer', school_class: '3C', grade: '3C', studentId: 'ST005', in_house: true },
  { id: '6', name: 'Emma Wagner', school_class: '3C', grade: '3C', studentId: 'ST006', in_house: false },
  { id: '7', name: 'Luis Becker', school_class: '4D', grade: '4D', studentId: 'ST007', in_house: true },
  { id: '8', name: 'Mia Hoffmann', school_class: '4D', grade: '4D', studentId: 'ST008', in_house: false },
  { id: '9', name: 'Finn Schneider', school_class: '5E', grade: '5E', studentId: 'ST009', in_house: true },
  { id: '10', name: 'Lara Schulz', school_class: '5E', grade: '5E', studentId: 'ST010', in_house: false },
];

export default function StudentDetailPage() {
  const router = useRouter();
  const params = useParams();
  const studentId = params.id as string;
  
  const [student, setStudent] = useState<Student | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [formData, setFormData] = useState({
    name: '',
    grade: '',
    studentId: '',
  });

  useEffect(() => {
    const fetchStudent = async () => {
      try {
        setLoading(true);
        // Try to fetch from API first, fall back to mock data
        try {
          const data = await studentService.getStudent(studentId);
          setStudent(data);
          // Initialize form data with student data
          setFormData({
            name: data.name,
            grade: data.grade ?? data.school_class ?? '',
            studentId: data.studentId ?? data.id ?? '',
          });
          setError(null);
        } catch (apiErr) {
          console.warn('Using mock data due to API error:', apiErr);
          // Find the student in our mock data
          const mockStudent = mockStudents.find(s => s.id === studentId);
          if (mockStudent) {
            setStudent(mockStudent);
            setFormData({
              name: mockStudent.name,
              grade: mockStudent.grade ?? mockStudent.school_class ?? '',
              studentId: mockStudent.studentId ?? mockStudent.id ?? '',
            });
            setError(null);
          } else {
            throw new Error('Student not found in mock data');
          }
        }
      } catch (err) {
        console.error('Error fetching student:', err);
        setError('Fehler beim Laden der Schülerdaten. Bitte versuchen Sie es später erneut.');
        setStudent(null);
      } finally {
        setLoading(false);
      }
    };

    if (studentId) {
      void fetchStudent();
    }
  }, [studentId]);

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const { name, value } = e.target;
    setFormData(prev => ({
      ...prev,
      [name]: value,
    }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    // Validate form
    if (!formData.name || !formData.grade || !formData.studentId) {
      setError('Bitte füllen Sie alle Felder aus.');
      return;
    }
    
    try {
      setLoading(true);
      setError(null);
      
      // Try to update on API, but fall back to mock update
      try {
        // Update student
        const updatedStudent = await studentService.updateStudent(studentId, {
          name: formData.name,
          school_class: formData.grade, // Map grade to school_class
          grade: formData.grade,
          studentId: formData.studentId,
        });
        
        setStudent(updatedStudent);
      } catch (apiErr) {
        console.warn('Using mock update due to API error:', apiErr);
        // Update in our local state only
        const updatedMockStudent: Student = {
          id: studentId,
          name: formData.name,
          school_class: formData.grade, // Map grade to school_class
          grade: formData.grade,
          studentId: formData.studentId,
          in_house: student?.in_house ?? false,
        };
        setStudent(updatedMockStudent);
      }
      
      setIsEditing(false);
    } catch (err) {
      console.error('Error updating student:', err);
      setError('Fehler beim Aktualisieren des Schülers. Bitte versuchen Sie es später erneut.');
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async () => {
    if (window.confirm('Sind Sie sicher, dass Sie diesen Schüler löschen möchten?')) {
      try {
        setLoading(true);
        try {
          await studentService.deleteStudent(studentId);
        } catch (apiErr) {
          console.warn('Mock delete due to API error:', apiErr);
          // Just redirect in development
        }
        router.push('/database/students');
      } catch (err) {
        console.error('Error deleting student:', err);
        setError('Fehler beim Löschen des Schülers. Bitte versuchen Sie es später erneut.');
        setLoading(false);
      }
    }
  };

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <div className="bg-red-50 text-red-800 p-4 rounded-lg max-w-md">
          <h2 className="font-semibold mb-2">Fehler</h2>
          <p>{error}</p>
          <button 
            onClick={() => router.back()} 
            className="mt-4 px-4 py-2 bg-red-100 hover:bg-red-200 text-red-800 rounded transition-colors"
          >
            Zurück
          </button>
        </div>
      </div>
    );
  }

  if (!student) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center p-4">
        <div className="bg-yellow-50 text-yellow-800 p-4 rounded-lg max-w-md">
          <h2 className="font-semibold mb-2">Schüler nicht gefunden</h2>
          <p>Der angeforderte Schüler konnte nicht gefunden werden.</p>
          <button 
            onClick={() => router.push('/database/students')} 
            className="mt-4 px-4 py-2 bg-yellow-100 hover:bg-yellow-200 text-yellow-800 rounded transition-colors"
          >
            Zurück zur Übersicht
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen">
      {/* Header */}
      <PageHeader 
        title={isEditing ? 'Schüler bearbeiten' : 'Schülerdetails'}
        backUrl="/database/students"
      />
      
      {/* Main Content */}
      <main className="max-w-3xl mx-auto p-4">
        <div className="bg-white shadow-md rounded-lg p-6">
          {isEditing ? (
            <form onSubmit={handleSubmit}>
              <div className="space-y-4">
                {/* Name field */}
                <div>
                  <label htmlFor="name" className="block text-sm font-medium text-gray-700 mb-1">
                    Name
                  </label>
                  <input
                    type="text"
                    id="name"
                    name="name"
                    value={formData.name}
                    onChange={handleChange}
                    className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-all duration-200"
                  />
                </div>
                
                {/* Grade field */}
                <div>
                  <label htmlFor="grade" className="block text-sm font-medium text-gray-700 mb-1">
                    Klasse
                  </label>
                  <input
                    type="text"
                    id="grade"
                    name="grade"
                    value={formData.grade}
                    onChange={handleChange}
                    className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-all duration-200"
                  />
                </div>
                
                {/* Student ID field */}
                <div>
                  <label htmlFor="studentId" className="block text-sm font-medium text-gray-700 mb-1">
                    Schüler-ID
                  </label>
                  <input
                    type="text"
                    id="studentId"
                    name="studentId"
                    value={formData.studentId}
                    onChange={handleChange}
                    className="w-full px-4 py-2 rounded-lg border border-gray-300 focus:outline-none focus:ring-2 focus:ring-blue-500 transition-all duration-200"
                  />
                </div>
                
                {/* Form actions */}
                <div className="flex justify-end pt-4">
                  <button
                    type="button"
                    onClick={() => setIsEditing(false)}
                    className="px-4 py-2 text-gray-700 mr-2 hover:bg-gray-100 rounded-lg transition-colors"
                    disabled={loading}
                  >
                    Abbrechen
                  </button>
                  <button
                    type="submit"
                    className="px-6 py-2 bg-gradient-to-r from-teal-500 to-blue-600 text-white rounded-lg hover:from-teal-600 hover:to-blue-700 hover:shadow-lg transition-all duration-200"
                    disabled={loading}
                  >
                    {loading ? 'Speichern...' : 'Speichern'}
                  </button>
                </div>
              </div>
            </form>
          ) : (
            <>
              <div className="flex justify-between items-center mb-6">
                <h2 className="text-xl font-bold text-gray-800">{student.name}</h2>
                <div className="flex space-x-2">
                  <button
                    onClick={() => setIsEditing(true)}
                    className="px-4 py-2 bg-blue-50 text-blue-600 rounded-lg hover:bg-blue-100 transition-colors"
                  >
                    Bearbeiten
                  </button>
                  <button
                    onClick={handleDelete}
                    className="px-4 py-2 bg-red-50 text-red-600 rounded-lg hover:bg-red-100 transition-colors"
                  >
                    Löschen
                  </button>
                </div>
              </div>
              
              <div className="space-y-4">
                <div className="border-b pb-2">
                  <div className="text-sm text-gray-500">Name</div>
                  <div className="text-lg">{student.name}</div>
                </div>
                
                <div className="border-b pb-2">
                  <div className="text-sm text-gray-500">Klasse</div>
                  <div className="text-lg">{student.grade}</div>
                </div>
                
                <div className="border-b pb-2">
                  <div className="text-sm text-gray-500">Schüler-ID</div>
                  <div className="text-lg">{student.studentId}</div>
                </div>
              </div>
            </>
          )}
        </div>
      </main>
    </div>
  );
}