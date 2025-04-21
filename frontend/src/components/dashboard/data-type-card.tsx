'use client';

import Link from 'next/link';
import { Card } from '@/components/ui/card';
import type { ReactNode } from 'react';

interface DataTypeCardProps {
  title: string;
  description: string;
  href: string;
  icon: ReactNode;
}

export function DataTypeCard({ title, description, href, icon }: DataTypeCardProps) {
  return (
    <Link href={href} className="block transition-transform hover:scale-[1.02]">
      <Card className="flex flex-col items-center text-center h-full py-8 px-4">
        <div className="flex items-center justify-center w-16 h-16 mb-4 text-blue-600">
          {icon}
        </div>
        <h3 className="text-xl font-bold mb-2">{title}</h3>
        <p className="text-gray-600 text-sm">{description}</p>
      </Card>
    </Link>
  );
}