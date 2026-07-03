import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Search } from "lucide-react";

import { InlineStatusBadge, DesktopSearchAction } from "./SearchRowHelpers";

const meta = {
  title: "ui/page-header/SearchRowHelpers",
} satisfies Meta;

export default meta;

type InlineStatusBadgeStory = StoryObj<typeof InlineStatusBadge>;
type DesktopSearchActionStory = StoryObj<typeof DesktopSearchAction>;

export const StatusOnlyMobile: InlineStatusBadgeStory = {
  render: () => (
    <InlineStatusBadge
      statusIndicator={{ color: "green", tooltip: "Online" }}
      variant="mobile"
    />
  ),
};

export const StatusAndBadgeDesktop: InlineStatusBadgeStory = {
  render: () => (
    <InlineStatusBadge
      statusIndicator={{ color: "yellow", tooltip: "Teilweise verfügbar" }}
      badge={{ count: 12, label: "Schüler", icon: <Search size={14} /> }}
      variant="desktop"
    />
  ),
};

export const BadgeOnlyMobile: InlineStatusBadgeStory = {
  render: () => (
    <InlineStatusBadge
      badge={{ count: 3, icon: <Search size={14} /> }}
      variant="mobile"
    />
  ),
};

export const EmptyRendersNothing: InlineStatusBadgeStory = {
  render: () => <InlineStatusBadge variant="desktop" />,
};

export const DesktopActionButton: DesktopSearchActionStory = {
  render: () => (
    <DesktopSearchAction
      hasTabs={false}
      hasTitle={false}
      actionButton={
        <button
          type="button"
          className="rounded-md bg-gray-900 px-4 py-2 text-sm text-white"
        >
          Neu anlegen
        </button>
      }
    />
  ),
};

export const DesktopStatusBadgeFallback: DesktopSearchActionStory = {
  render: () => (
    <DesktopSearchAction
      hasTabs={false}
      hasTitle={false}
      statusIndicator={{ color: "red", tooltip: "Offline" }}
      badge={{ count: 5, label: "Aktiv" }}
    />
  ),
};

export const DesktopNoActionNoBadge: DesktopSearchActionStory = {
  render: () => <DesktopSearchAction hasTabs={true} hasTitle={true} />,
};
