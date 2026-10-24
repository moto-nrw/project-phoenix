import { useRef, type ComponentProps } from "react";
import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useApiFormError } from "~/contexts/ToastContext";
import { PasswordChangeModal } from "./password-change-modal";

/** The modal on the shared error path, as every owner mounts it. */
function PasswordChangeModalOnErrorPath(
  props: Omit<
    ComponentProps<typeof PasswordChangeModal>,
    "errorPath" | "formRef"
  >,
) {
  const formRef = useRef<HTMLFormElement>(null);
  const errorPath = useApiFormError(formRef);
  return (
    <PasswordChangeModal {...props} errorPath={errorPath} formRef={formRef} />
  );
}

const meta = {
  title: "components/ui/PasswordChangeModal",
  component: PasswordChangeModalOnErrorPath,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    isOpen: true,
    onClose: () => undefined,
  },
} satisfies Meta<typeof PasswordChangeModalOnErrorPath>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Open: Story = {};

export const Closed: Story = {
  args: {
    isOpen: false,
  },
};
