'use client';

interface SectionTitleProps {
  title: string;
}

export function SectionTitle({ title }: SectionTitleProps) {
  return (
    <div className="mb-12 text-center group">
      <h2 className="text-3xl font-bold bg-gradient-to-r from-blue-500 to-green-500 inline-block text-transparent bg-clip-text mb-2 transition-transform duration-300 group-hover:scale-[1.02]">
        {title}
      </h2>
      <div className="w-full max-w-sm mx-auto h-1 bg-gradient-to-r from-blue-500 to-green-500 rounded-full transition-all duration-300 group-hover:max-w-md group-hover:h-1.5"></div>
    </div>
  );
}