"use client";

import React from "react";

interface PageTitleProps {
  title: string;
  className?: string;
}

export function PageTitle({ title, className = "" }: PageTitleProps) {
  return (
    <div className={`mb-6 ml-6 ${className}`}>
      <h1 className="text-2xl font-bold text-gray-900 md:text-3xl">{title}</h1>
    </div>
  );
}
