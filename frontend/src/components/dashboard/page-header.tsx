'use client';

import { redirect } from 'next/navigation';
import Image from 'next/image';
import Link from 'next/link';
import { Button } from '@/components/ui/button';

interface PageHeaderProps {
  title: string;
  backUrl?: string;
  showHelpButton?: boolean;
}

export function PageHeader({ 
  title, 
  backUrl = '/dashboard',
  showHelpButton = true 
}: PageHeaderProps) {
  return (
    <header className="w-full bg-white/80 backdrop-blur-sm shadow-sm p-4">
      <div className="container mx-auto flex justify-between items-center">
        <div className="flex items-center gap-3">
          {backUrl && (
            <Link href={backUrl}>
              <Button variant="outline" className="flex items-center gap-1">
                <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
                </svg>
                Zurück
              </Button>
            </Link>
          )}
          <div className="flex items-center gap-3 ml-2">
            <Image 
              src="/images/moto_transparent.png" 
              alt="Logo" 
              width={36} 
              height={36} 
              className="h-9 w-auto"
            />
            <h1 className="text-xl font-bold">
              {title}
            </h1>
          </div>
        </div>
        <div className="flex gap-3">
          {showHelpButton && (
            <Button 
              variant="outline" 
              onClick={() => redirect('/help')}
              className="flex h-9 w-9 items-center justify-center rounded-full p-0"
            >
              ?
            </Button>
          )}
        </div>
      </div>
    </header>
  );
}