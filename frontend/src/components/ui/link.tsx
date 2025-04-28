'use client';

import NextLink from 'next/link';
import type { ComponentProps } from 'react';

type LinkProps = ComponentProps<typeof NextLink> & {
  variant?: 'primary' | 'secondary';
};

export function Link({ 
  children, 
  variant = 'primary',
  className = '',
  ...props 
}: LinkProps) {
  const variantStyles = {
    primary: 'text-teal-600 hover:text-teal-800 font-medium',
    secondary: 'text-gray-600 hover:text-gray-800'
  };

  return (
    <NextLink
      className={`${variantStyles[variant]} ${className}`}
      {...props}
    >
      {children}
    </NextLink>
  );
}