'use client';

interface SectionTitleProps {
  title: string;
}

export function SectionTitle({ title }: SectionTitleProps) {
  return (
    <div className="mb-12 text-center">
      <h2 className="text-3xl font-bold bg-gradient-to-r from-blue-500 to-green-500 inline-block text-transparent bg-clip-text mb-2">
        {title}
      </h2>
      <div className="w-full max-w-sm mx-auto h-1 bg-gradient-to-r from-blue-500 to-green-500 rounded-full"></div>
    </div>
  );
}