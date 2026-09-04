// Profile dropdown component for header
// Extracted to reduce cognitive complexity in header.tsx

"use client";

import { forwardRef } from "react";
import { NavLink } from "~/components/ui/nav-link";
import { useTranslations } from "next-intl";
import { EyeIcon, UserIcon } from "@phosphor-icons/react";
import { Avatar } from "~/components/ui/avatar";
import { Button } from "~/components/ui/button";

// UserAvatar is a thin compatibility wrapper around the shared <Avatar>.
// Keeping the prop names (avatarUrl, userName) avoids touching every call
// site below; the body delegates so user-avatar pixels match student/staff
// avatars 1:1.
interface UserAvatarProps {
  readonly avatarUrl?: string | null;
  readonly userName: string;
  readonly size?: "sm" | "md";
}

function UserAvatar({ avatarUrl, userName, size = "sm" }: UserAvatarProps) {
  return <Avatar imageUrl={avatarUrl} name={userName} size={size} decorative />;
}

/**
 * Logout icon SVG
 */
function LogoutIcon({ className }: Readonly<{ className?: string }>) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
    >
      <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
      <polyline points="16 17 21 12 16 7" />
      <line x1="21" y1="12" x2="9" y2="12" />
    </svg>
  );
}

/**
 * Chevron down icon for dropdown toggle
 */
interface ChevronDownIconProps {
  readonly isOpen: boolean;
}

function ChevronDownIcon({ isOpen }: ChevronDownIconProps) {
  return (
    <svg
      className={`h-4 w-4 text-gray-400 transition-[color,transform] duration-200 ${isOpen ? "rotate-180 text-gray-600" : ""}`}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M19 9l-7 7-7-7"
      />
    </svg>
  );
}

/**
 * Profile icon (user silhouette)
 */
function ProfileIcon() {
  return (
    <UserIcon
      aria-hidden="true"
      weight="regular"
      className="mr-3 h-4 w-4 text-gray-400 transition-colors group-hover:text-gray-600 group-active:text-white"
    />
  );
}

/**
 * Profile dropdown trigger button
 */
interface ProfileTriggerProps {
  readonly displayName: string;
  readonly ariaLabel: string;
  readonly displayAvatar?: string | null;
  readonly userRole: string;
  readonly isOpen: boolean;
  readonly onClick: () => void;
  readonly compactOnTablet?: boolean;
  readonly menuId?: string;
}

export const ProfileTrigger = forwardRef<
  HTMLButtonElement,
  ProfileTriggerProps
>(function ProfileTrigger(
  {
    displayName,
    ariaLabel,
    displayAvatar,
    userRole,
    isOpen,
    onClick,
    compactOnTablet = false,
    menuId,
  }: ProfileTriggerProps,
  ref,
) {
  return (
    <button
      ref={ref}
      type="button"
      onClick={onClick}
      aria-label={ariaLabel}
      aria-haspopup="dialog"
      aria-expanded={isOpen}
      aria-controls={menuId}
      className="flex touch-manipulation items-center space-x-2 rounded-lg p-1.5 transition-colors duration-200 hover:bg-gray-100 active:bg-gray-200"
    >
      <UserAvatar avatarUrl={displayAvatar} userName={displayName} size="sm" />

      <div
        className={`hidden text-left ${compactOnTablet ? "lg:block" : "md:block"}`}
      >
        <div className="text-sm font-medium text-gray-900">{displayName}</div>
        <div className="text-xs text-gray-500">{userRole}</div>
      </div>

      <ChevronDownIcon isOpen={isOpen} />
    </button>
  );
});

/**
 * Profile dropdown menu
 */
interface ProfileDropdownMenuProps {
  readonly isOpen: boolean;
  readonly displayName: string;
  readonly displayAvatar?: string | null;
  readonly userEmail: string;
  readonly onClose: () => void;
  readonly onLogout: () => void;
  readonly profileUrl?: string | null;
  readonly profileLabel?: string;
  /**
   * Mitarbeiter-Vorschau (#2893): nur im Mitarbeiter-Portal und nur für
   * Admins gesetzt. Öffnet den Auswahl-Dialog.
   */
  readonly onStartPreview?: () => void;
}

export function ProfileDropdownMenu({
  isOpen,
  displayName,
  displayAvatar,
  userEmail,
  onClose,
  onLogout,
  profileUrl,
  profileLabel,
  onStartPreview,
}: ProfileDropdownMenuProps) {
  // parentNav carries German values in the staff/operator shells (via
  // ShellIntlProvider), so those portals render unchanged; only the parents
  // portal swaps in the localized catalog.
  const t = useTranslations("parentNav");
  const handleLogoutClick = (e: React.MouseEvent) => {
    e.preventDefault();
    onClose();
    onLogout();
  };

  return (
    <div
      className={`moto-popover-surface w-72 rounded-2xl border transition-[opacity,transform,visibility] duration-150 ease-out ${
        isOpen
          ? "visible translate-y-0 opacity-100"
          : "invisible -translate-y-2 opacity-0"
      }`}
    >
      {/* User info header */}
      <div className="border-b border-gray-100/50 px-4 py-4">
        <div className="flex items-center space-x-3">
          <UserAvatar
            avatarUrl={displayAvatar}
            userName={displayName}
            size="md"
          />
          <div className="min-w-0 flex-1">
            <div className="truncate font-semibold text-gray-900">
              {displayName}
            </div>
            <div className="truncate text-xs text-gray-500" title={userEmail}>
              {userEmail}
            </div>
          </div>
        </div>
      </div>

      {/* Menu items */}
      <div className="p-2">
        {profileUrl && (
          <NavLink
            href={profileUrl}
            onClick={onClose}
            className="group flex items-center rounded-xl px-3 py-2.5 text-sm font-medium text-gray-700 transition-[background-color,color] duration-200 ease-out hover:bg-gray-100 hover:text-gray-900 active:bg-gray-900 active:text-white"
          >
            <ProfileIcon />
            {profileLabel ?? t("profile")}
          </NavLink>
        )}

        {onStartPreview && (
          // Kit Button (ghost) with the dropdown's menu-item geometry layered
          // on top, so the entry sits flush with its hand-rolled neighbours
          // while keeping the kit's interaction contract (focus ring, states).
          <Button
            type="button"
            variant="ghost"
            size="md"
            onClick={() => {
              onClose();
              onStartPreview();
            }}
            className="group w-full justify-start rounded-xl px-3 py-2.5 text-left font-medium text-gray-700 active:bg-gray-900 active:text-white"
          >
            <EyeIcon
              aria-hidden="true"
              weight="regular"
              className="mr-3 h-4 w-4 text-gray-400 transition-colors group-hover:text-gray-600 group-active:text-white"
            />
            Ansicht eines Mitarbeitenden
          </Button>
        )}

        {/* Divider */}
        <div className="my-2 h-px bg-gradient-to-r from-transparent via-gray-200 to-transparent" />

        {/* Logout button */}
        <button
          type="button"
          onClick={handleLogoutClick}
          className="group text-moto-red hover:bg-moto-red-soft hover:text-moto-red-strong active:bg-moto-red flex w-full items-center rounded-xl px-3 py-2.5 text-left text-sm font-medium transition-[background-color,color] duration-200 ease-out active:text-white"
        >
          <LogoutIcon className="mr-3 h-4 w-4 transition-colors group-active:text-white" />
          {t("logout")}
        </button>
      </div>
    </div>
  );
}
