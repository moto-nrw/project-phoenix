'use client';

import { useSession } from 'next-auth/react';
import { redirect, useRouter } from 'next/navigation';
import { useState } from 'react';
import { DataListPage } from '@/components/dashboard';

// Mock student data until we connect to the API
interface Student {
  id: string;
  name: string;
  grade: string;
  studentId: string;
}

const mockStudents: Student[] = [
  { id: '1', name: 'Anna Müller', grade: '1A', studentId: 'ST001' },
  { id: '2', name: 'Max Schmidt', grade: '1A', studentId: 'ST002' },
  { id: '3', name: 'Sophie Weber', grade: '2B', studentId: 'ST003' },
  { id: '4', name: 'Lena Fischer', grade: '2B', studentId: 'ST004' },
  { id: '5', name: 'Noah Meyer', grade: '3C', studentId: 'ST005' },
  { id: '6', name: 'Emma Wagner', grade: '3C', studentId: 'ST006' },
  { id: '7', name: 'Luis Becker', grade: '4D', studentId: 'ST007' },
  { id: '8', name: 'Mia Hoffmann', grade: '4D', studentId: 'ST008' },
  { id: '9', name: 'Finn Schneider', grade: '5E', studentId: 'ST009' },
  { id: '10', name: 'Lara Schulz', grade: '5E', studentId: 'ST010' },
];

export default function StudentsPage() {
  const router = useRouter();
  const [selectedStudent, setSelectedStudent] = useState<Student | null>(null);
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect('/login');
    },
  });

  if (status === 'loading') {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    );
  }

  const handleSelectStudent = (student: Student) => {
    setSelectedStudent(student);
    router.push(`/database/students/${student.id}`);
  };

  // Custom renderer for student items
  const renderStudent = (student: Student) => (
    <>
      <div className="flex flex-col group-hover:translate-x-1 transition-transform duration-200">
        <span className="font-semibold text-gray-900 group-hover:text-blue-600 transition-colors duration-200">{student.name}</span>
        <span className="text-sm text-gray-500">Klasse: {student.grade} | ID: {student.studentId}</span>
      </div>
      <svg 
        xmlns="http://www.w3.org/2000/svg" 
        className="h-5 w-5 text-gray-400 group-hover:text-blue-500 group-hover:transform group-hover:translate-x-1 transition-all duration-200" 
        fill="none" 
        viewBox="0 0 24 24" 
        stroke="currentColor"
      >
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
      </svg>
    </>
  );

  return (
    <DataListPage
      title="Schülerauswahl"
      sectionTitle="Schüler auswählen"
      backUrl="/database"
      newEntityLabel="Neuen Schüler erstellen"
      newEntityUrl="/database/students/new"
      data={mockStudents}
      onSelectEntity={handleSelectStudent}
      renderEntity={renderStudent}
    />
  );
}