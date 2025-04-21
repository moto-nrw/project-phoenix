'use client';

import { useSession } from 'next-auth/react';
import { redirect } from 'next/navigation';
import Image from 'next/image';
import Link from 'next/link';
import { Card } from '../../components/ui/card';
import { Button } from '../../components/ui/button';

export default function DashboardPage() {
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

  return (
    <div className="min-h-screen">
      {/* Header */}
      <header className="w-full bg-white/80 backdrop-blur-sm shadow-sm p-4">
        <div className="container mx-auto flex justify-between items-center">
          <div className="flex items-center gap-3">
            <Image 
              src="/images/moto_transparent.png" 
              alt="Logo" 
              width={40} 
              height={40} 
              className="h-10 w-auto"
            />
            <h1 className="text-xl font-bold">
              Willkommen, {session?.user?.name ?? 'Root'}!
            </h1>
          </div>
          <div className="flex gap-3">
            <Button 
              variant="outline" 
              onClick={() => redirect('/help')}
              className="hidden sm:flex"
            >
              ?
            </Button>
            <Button onClick={() => redirect('/api/auth/signout')}>
              Logout
            </Button>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="container mx-auto p-4 py-8">
        <div className="mb-12 text-center">
          <h2 className="text-3xl font-bold bg-gradient-to-r from-blue-500 to-green-500 inline-block text-transparent bg-clip-text mb-2">
            Übersicht
          </h2>
          <div className="w-full max-w-sm mx-auto h-1 bg-gradient-to-r from-blue-500 to-green-500 rounded-full"></div>
        </div>

        {/* Dashboard Cards */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 max-w-6xl mx-auto">
          {/* CSV Import Card */}
          <Link href="/import" className="block transition-transform hover:scale-[1.02]">
            <Card className="flex flex-col items-center text-center h-full">
              <div className="flex items-center justify-center w-16 h-16 mb-4 rounded-full bg-blue-100 text-blue-500">
                <svg xmlns="http://www.w3.org/2000/svg" className="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M9 19l3 3m0 0l3-3m-3 3V10" />
                </svg>
              </div>
              <h3 className="text-xl font-bold mb-2">CSV-Import</h3>
              <p className="text-gray-600">Daten importieren</p>
            </Card>
          </Link>

          {/* Password Card */}
          <Link href="/password" className="block transition-transform hover:scale-[1.02]">
            <Card className="flex flex-col items-center text-center h-full">
              <div className="flex items-center justify-center w-16 h-16 mb-4 rounded-full bg-blue-100 text-blue-500">
                <svg xmlns="http://www.w3.org/2000/svg" className="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
                </svg>
              </div>
              <h3 className="text-xl font-bold mb-2">Root Passwort</h3>
              <p className="text-gray-600">Passwort ändern</p>
            </Card>
          </Link>

          {/* Database Card */}
          <Link href="/database" className="block transition-transform hover:scale-[1.02]">
            <Card className="flex flex-col items-center text-center h-full">
              <div className="flex items-center justify-center w-16 h-16 mb-4 rounded-full bg-blue-100 text-blue-500">
                <svg xmlns="http://www.w3.org/2000/svg" className="h-8 w-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 7v10c0 2 1.5 3 3.5 3s3.5-1 3.5-3V7c0-2-1.5-3-3.5-3S4 5 4 7zm14-1v12c0 1.1-.9 2-2 2H9.5" />
                </svg>
              </div>
              <h3 className="text-xl font-bold mb-2">Datenbank</h3>
              <p className="text-gray-600">Datensätze bearbeiten</p>
            </Card>
          </Link>
        </div>
      </main>
    </div>
  );
}