'use client';

import { redirect } from 'next/navigation';
import Image from 'next/image';
import { Button } from '@/components/ui/button';

interface HeaderProps {
  userName?: string;
}

export function Header({ userName = 'Root' }: HeaderProps) {
  return (
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
            Willkommen, {userName}!
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
          <Button onClick={() => redirect('/logout')}>
            Logout
          </Button>
        </div>
      </div>
    </header>
  );
}