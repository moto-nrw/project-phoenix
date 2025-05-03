"use client";

import Link from "next/link";
import { Card } from "@/components/ui/card";
import type { ReactNode } from "react";

interface DataTypeCardProps {
  title: string;
  description: string;
  href: string;
  icon: ReactNode;
}

export function DataTypeCard({
  title,
  description,
  href,
  icon,
}: DataTypeCardProps) {
  return (
    <Link href={href} className="block transition-transform hover:scale-[1.02]">
      <Card className="flex h-full flex-col items-center text-center">
        <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-blue-100 text-blue-500">
          {icon}
        </div>
        <h3 className="mb-2 text-xl font-bold">{title}</h3>
        <p className="text-gray-600">{description}</p>
      </Card>
    </Link>
  );
}
