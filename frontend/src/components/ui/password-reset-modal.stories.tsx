import { useRef, type ComponentProps } from "react";
import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useApiFormError } from "~/contexts/ToastContext";
import { PasswordResetModal } from "./password-reset-modal";

/** The modal on the shared error path, as every owner mounts it. */
function PasswordResetModalOnErrorPath(
  props: Omit<
    ComponentProps<typeof PasswordResetModal>,
    "errorPath" | "formRef"
  >,
) {
  const formRef = useRef<HTMLFormElement>(null);
  const errorPath = useApiFormError(formRef);
  return (
    <PasswordResetModal {...props} errorPath={errorPath} formRef={formRef} />
  );
}

const meta: Meta<typeof PasswordResetModalOnErrorPath> = {
  title: "components/ui/PasswordResetModal",
  component: PasswordResetModalOnErrorPath,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof PasswordResetModalOnErrorPath>;

export const Default: Story = {
  args: {
    isOpen: true,
    onClose: () => {
      // no-op for story
    },
    onRequestReset: async () => ({ message: "ok" }),
  },
};

export const Loading: Story = {
  args: {
    isOpen: true,
    onClose: () => {
      // no-op for story
    },
    onRequestReset: () =>
      new Promise(() => {
        // never resolves, keeps the loading state visible
      }),
  },
};

export const Success: Story = {
  args: {
    isOpen: true,
    onClose: () => {
      // no-op for story
    },
    onRequestReset: async () => ({ message: "ok" }),
  },
  play: async ({ canvasElement }) => {
    const emailInput =
      canvasElement.querySelector<HTMLInputElement>("#reset-email");
    if (emailInput) {
      emailInput.value = "test@example.com";
    }
  },
};
