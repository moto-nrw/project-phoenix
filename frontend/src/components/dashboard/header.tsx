'use client';

import Image from 'next/image';
import Link from 'next/link';
import { Button } from '@/components/ui/button';

interface HeaderProps {
  userName?: string;
}

export function Header({ userName = 'Root' }: HeaderProps) {
  return (
    <header className="w-full bg-white/80 backdrop-blur-sm shadow-sm p-4">
      <div className="container mx-auto flex justify-between items-center">
        <div className="flex items-center">
          {/* Logo and title - always in the same position */}
          <div className="flex items-center gap-3 w-[160px] justify-center">
            <img 
              src="/images/moto_transparent.png" 
              alt="Logo" 
              width={40} 
              height={40} 
              className="h-10 w-auto"
            />
          </div>
          
          {/* Title section */}
          <div className="flex items-center">
            <h1 className="text-xl font-bold">
              <span className="hidden md:inline">Willkommen, {userName}!</span>
            </h1>
          </div>
        </div>
        
        {/* Logout button on the right */}
        <div>
          <Link href="/logout">
            <Button>
              Logout
            </Button>
          </Link>
        </div>
      </div>
    </header>
  );
}