'use client';

import Link from 'next/link';
import { Card } from '@/components/ui/card';
import type { ReactNode } from 'react';

interface FeatureCardProps {
  title: string;
  description: string;
  href: string;
  icon: ReactNode;
}

export function FeatureCard({ title, description, href, icon }: FeatureCardProps) {
  return (
    <Link href={href} className="block transition-transform hover:scale-[1.02]">
      <Card className="flex flex-col items-center text-center h-full">
        <div className="flex items-center justify-center w-16 h-16 mb-4 rounded-full bg-blue-100 text-blue-500">
          {icon}
        </div>
        <h3 className="text-xl font-bold mb-2">{title}</h3>
        <p className="text-gray-600">{description}</p>
      </Card>
    </Link>
  );
}