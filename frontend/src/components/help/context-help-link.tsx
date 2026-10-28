"use client";

import { usePathname, useSearchParams } from "next/navigation";
import { useMemo } from "react";
import { QuestionIcon } from "@phosphor-icons/react";
import { ButtonLink } from "~/components/ui/button";
import { Tooltip } from "~/components/ui/tooltip";
import { useShellAuth } from "~/lib/shell-auth-context";
import {
  useNFCEnabled,
  useOpenCareGroupMode,
  usePresenceMode,
} from "~/lib/tenant-context";
import { isHelpTopicAvailable } from "~/lib/help-topic-availability";
import {
  buildHelpHref,
  type HelpRole,
  type HelpTopicId,
} from "~/lib/help-topics";
import { cn } from "~/lib/utils";

interface ContextHelpLinkProps {
  readonly topic: HelpTopicId;
  readonly className?: string;
  readonly containerClassName?: string;
}

interface ContextHelpHrefOptions {
  readonly topic: HelpTopicId;
  readonly role: HelpRole;
  readonly nfcEnabled: boolean;
  readonly presenceMode: "detailed" | "binary";
  readonly groupMode: "fixed_groups" | "open_care";
  readonly returnTo: string;
}

export function buildContextHelpHref({
  topic,
  ...context
}: ContextHelpHrefOptions): string {
  return buildHelpHref(context, topic);
}

export function ContextHelpLink({
  topic,
  className,
  containerClassName,
}: Readonly<ContextHelpLinkProps>) {
  const rawPathname = usePathname();
  const searchParams = useSearchParams();
  const { user, mode } = useShellAuth();
  const nfcEnabled = useNFCEnabled();
  const presenceMode = usePresenceMode();
  const openCareGroupMode = useOpenCareGroupMode();
  const currentQuery = searchParams.toString();
  const returnTo = currentQuery
    ? `${rawPathname}?${currentQuery}`
    : rawPathname;
  const role: HelpRole =
    mode === "parent"
      ? "parent"
      : mode === "school"
        ? "teacher"
        : mode === "traeger"
          ? "traeger"
          : user?.roles.includes("admin")
            ? "lead"
            : "caregiver";
  const groupMode = openCareGroupMode ? "open_care" : "fixed_groups";

  // Nicht jede Seite hat fuer jede Rolle eine passende Anleitung: `/settings`
  // und `/tagesinformationen` duerfen Betreuungskraefte oeffnen, ihre Artikel
  // sind aber fuer die Leitung geschrieben. Dann fuehrte der Knopf in eine
  // fremde Seitenleiste mit einer Brotkrume, die es fuer die Rolle nicht gibt.
  // Lieber kein Fragezeichen als eines, das in die falsche Anleitung fuehrt.
  const hasArticleForRole = useMemo(
    () =>
      isHelpTopicAvailable(topic, role, presenceMode, groupMode, nfcEnabled),
    [groupMode, nfcEnabled, presenceMode, role, topic],
  );

  const href = buildContextHelpHref({
    topic,
    role,
    nfcEnabled,
    presenceMode,
    groupMode,
    returnTo,
  });

  if (!hasArticleForRole) return null;

  return (
    <Tooltip
      content="Hilfe zu dieser Seite"
      asChild
      iconOnly
      className={containerClassName}
      bubbleClassName="right-0 left-auto"
    >
      <ButtonLink
        href={href}
        variant="ghost"
        size="icon"
        aria-label="Hilfe zu dieser Seite"
        className={cn("size-11 shrink-0 rounded-full lg:size-10", className)}
      >
        <QuestionIcon className="size-5" weight="bold" aria-hidden="true" />
      </ButtonLink>
    </Tooltip>
  );
}
