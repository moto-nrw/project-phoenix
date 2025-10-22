"use client";

import React from "react";

interface PageTitleProps {
  title: string;
  className?: string;
}

export function PageTitle({ title, className = "" }: PageTitleProps) {
  return (
    <div className={`mb-6 ml-6 ${className}`}>
      <div className="relative inline-block">
        <h1 className="text-2xl md:text-3xl font-bold text-gray-900 pb-3">
          {title}
        </h1>
        {/* Underline indicator - matches tab style */}
        <div
          className="absolute bottom-0 left-0 h-0.5 bg-gray-900 rounded-full"
          style={{ width: '80%' }}
        />
      </div>
    </div>
  );
}
