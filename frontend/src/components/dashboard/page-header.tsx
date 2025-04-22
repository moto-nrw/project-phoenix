'use client';

import Image from 'next/image';
import Link from 'next/link';
import { Button } from '@/components/ui/button';
import React from 'react';

interface PageHeaderProps {
  title: string | React.ReactNode;
  description?: string;
  backUrl?: string;
}

export function PageHeader({ 
  title, 
  description,
  backUrl = '/dashboard'
}: PageHeaderProps) {
  return (
    <header className="w-full bg-white/80 backdrop-blur-sm shadow-sm p-4">
      <div className="container mx-auto flex justify-between items-center">
        <div className="flex items-center">
          {/* Logo and title - always in the same position */}
          <div className="flex items-center gap-3 w-[160px] justify-center">
            <Image 
              src="/images/moto_transparent.png" 
              alt="Logo" 
              width={40} 
              height={40} 
              className="h-10 w-auto"
            />
          </div>
          
          {/* Title section */}
          <div className="flex flex-col">
            <h1 className="text-xl font-bold">
              <span className="hidden md:inline">{title}</span>
            </h1>
            {description && (
              <p className="text-sm text-gray-500 hidden md:block">{description}</p>
            )}
          </div>
        </div>
        
        {/* Back button on the right */}
        <div>
          {backUrl && (
            <Link href={backUrl}>
              <Button variant="outline" className="flex items-center gap-1">
                <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 19l-7-7m0 0l7-7m-7 7h18" />
                </svg>
                <span className="hidden md:inline">Zurück</span>
              </Button>
            </Link>
          )}
        </div>
      </div>
    </header>
  );
}