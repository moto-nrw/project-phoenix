import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ReactNode } from "react";
import { fn } from "storybook/test";

import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import {
  ActiveSupervisionLoadingView,
  EmptyRoomsView,
  NoActiveSupervisionAccessView,
  ReleaseSupervisionModal,
  OpenRoomNotice,
  SchulhofSuperviseButton,
} from "./states";

function WithBreadcrumb({ children }: { readonly children: ReactNode }) {
  return <BreadcrumbProvider>{children}</BreadcrumbProvider>;
}

const meta = {
  title: "active-supervisions/states",
} satisfies Meta;

export default meta;

type Story = StoryObj<typeof meta>;

export const Loading: Story = {
  render: () => <ActiveSupervisionLoadingView />,
};

export const NoAccess: Story = {
  render: () => (
    <WithBreadcrumb>
      <NoActiveSupervisionAccessView />
    </WithBreadcrumb>
  ),
};

export const EmptyRooms: Story = {
  render: () => (
    <EmptyRoomsView
      onClaimed={fn()}
      cachedActiveGroups={[]}
      currentStaffId={undefined}
    />
  ),
};

export const ReleaseModal: Story = {
  render: () => (
    <ReleaseSupervisionModal
      isOpen={true}
      isConfirmLoading={false}
      onClose={fn()}
      onConfirm={fn()}
    />
  ),
};

export const OpenRoomShared: Story = {
  render: () => <OpenRoomNotice isUserSupervising={false} />,
};

export const OpenRoomOwnSupervision: Story = {
  render: () => <OpenRoomNotice isUserSupervising />,
};

export const OpenRoomWithSchulhofOffer: Story = {
  render: () => (
    <OpenRoomNotice
      isUserSupervising={false}
      supervisorNames={["Anna Meier", "Ben Fischer"]}
      action={<SchulhofSuperviseButton isToggling={false} onToggle={fn()} />}
    />
  ),
};
