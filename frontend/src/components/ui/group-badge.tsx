"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { Badge } from "./badge";

export interface GroupBadgeProps {
  groupName: string;
  groupId: string | number;
  onClick?: (e: React.MouseEvent) => void;
  asLink?: boolean; // New prop to control if it should be a link
}

export function GroupBadge({ groupName, groupId, onClick, asLink = true }: GroupBadgeProps) {
  const router = useRouter();
  
  const handleClick = (e: React.MouseEvent) => {
    e.stopPropagation(); // Prevent triggering parent click events
    onClick?.(e);
    
    // If not rendering as a link, handle navigation on click
    if (!asLink) {
      router.push(`/database/groups/${groupId}`);
    }
  };

  const badgeContent = (
    <Badge
      variant="blue"
      className="hover:bg-blue-200 transition-colors duration-150 cursor-pointer"
      icon={
        <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
          <path d="M13 6a3 3 0 11-6 0 3 3 0 016 0zM18 8a2 2 0 11-4 0 2 2 0 014 0zM14 15a4 4 0 00-8 0v3h8v-3zM6 8a2 2 0 11-4 0 2 2 0 014 0zM16 18v-3a5.972 5.972 0 00-.75-2.906A3.005 3.005 0 0119 15v3h-3zM4.75 12.094A5.973 5.973 0 004 15v3H1v-3a3 3 0 013.75-2.906z" />
        </svg>
      }
    >
      {groupName}
    </Badge>
  );

  // If asLink is true, wrap in Link component
  if (asLink) {
    return (
      <Link
        href={`/database/groups/${groupId}`}
        onClick={handleClick}
        className="inline-flex"
      >
        {badgeContent}
      </Link>
    );
  }

  // Otherwise, render as a clickable div
  return (
    <div onClick={handleClick} className="inline-flex">
      {badgeContent}
    </div>
  );
}