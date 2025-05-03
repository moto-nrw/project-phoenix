"use client";

import React from "react";

interface SectionTitleProps {
  title: string;
}

export function SectionTitle({ title }: SectionTitleProps) {
  return (
    <div className="group mb-12 text-center">
      <h2 className="mb-2 inline-block bg-gradient-to-r from-blue-500 to-green-500 bg-clip-text text-3xl font-bold text-transparent transition-transform duration-300 group-hover:scale-[1.02]">
        {title}
      </h2>
      <div className="mx-auto h-1 w-full max-w-sm rounded-full bg-gradient-to-r from-blue-500 to-green-500 transition-all duration-300 group-hover:h-1.5 group-hover:max-w-md"></div>
    </div>
  );
}
