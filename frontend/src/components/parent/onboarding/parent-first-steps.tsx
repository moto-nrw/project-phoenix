"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import {
  CaretDownIcon,
  CaretUpIcon,
  CheckCircleIcon,
  CircleIcon,
  ListChecksIcon,
} from "@phosphor-icons/react";
import { Button, ButtonLink } from "~/components/ui/button";
import { CoachMark } from "~/components/ui/coach-mark";
import { ConfirmationModal } from "~/components/ui/modal";
import { ProgressBar } from "~/components/ui/progress-bar";
import {
  buildHelpHref,
  HELP_TOPICS,
  type HelpTopicId,
} from "~/lib/help-topics";
import { LOCATION_COLORS } from "~/lib/location-helper";
import {
  completeParentFirstStep,
  readParentFirstStepsState,
  writeParentFirstStepsState,
  type ParentFirstStepsState,
} from "./parent-first-steps-state";
import {
  buildParentTours,
  useParentFirstStepsTour,
  type ParentFirstStepKey,
  type ParentTourCopy,
} from "./parent-first-steps-tours";

type View = "beacon" | "checklist";

interface StepContent {
  readonly key: ParentFirstStepKey;
  readonly title: string;
  readonly description: string;
  readonly help: readonly { label: string; topic: HelpTopicId }[];
}

const EMPTY_STATE: ParentFirstStepsState = {
  completed: [],
  dismissed: false,
  finished: false,
};

function claimFirstOpen(accountId: string): boolean {
  const key = `parent-first-steps-opened:${accountId}`;
  try {
    if (sessionStorage.getItem(key)) return false;
    sessionStorage.setItem(key, "1");
    return true;
  } catch {
    return true;
  }
}

export function ParentFirstSteps({
  accountId,
  childCount,
  newsEnabled,
}: Readonly<{
  accountId: string;
  childCount: number;
  newsEnabled: boolean;
}>) {
  const t = useTranslations("parentFirstSteps");
  const pathname = usePathname();
  const [state, setState] = useState<ParentFirstStepsState | null>(null);
  const [view, setView] = useState<View>("beacon");
  const [expanded, setExpanded] = useState<ParentFirstStepKey | null>(null);
  const [confirmDismiss, setConfirmDismiss] = useState(false);
  const hasChild = childCount > 0;

  const steps = useMemo<readonly StepContent[]>(() => {
    const list: StepContent[] = [
      {
        key: "start",
        title: t("steps.start.title"),
        description: t("steps.start.description"),
        help: [
          { label: t("actions.guide"), topic: HELP_TOPICS.parentChildOverview },
        ],
      },
    ];
    if (hasChild) {
      list.push({
        key: "childData",
        title: t("steps.childData.title"),
        description: t("steps.childData.description"),
        help: [
          { label: t("actions.guide"), topic: HELP_TOPICS.parentChildOverview },
        ],
      });
    }
    list.push({
      key: "messagesNews",
      title: newsEnabled
        ? t("steps.messagesNews.title")
        : t("steps.messages.title"),
      description: newsEnabled
        ? t("steps.messagesNews.description")
        : t("steps.messages.description"),
      help: [
        {
          label: t("actions.messagesGuide"),
          topic: HELP_TOPICS.parentMessages,
        },
        ...(newsEnabled
          ? [
              {
                label: t("actions.newsGuide"),
                topic: HELP_TOPICS.parentNews,
              },
            ]
          : []),
      ],
    });
    list.push(
      {
        key: "installApp",
        title: t("steps.installApp.title"),
        description: t("steps.installApp.description"),
        help: [
          { label: t("actions.guide"), topic: HELP_TOPICS.parentInstallApp },
        ],
      },
      {
        key: "notifications",
        title: t("steps.notifications.title"),
        description: t("steps.notifications.description"),
        help: [
          {
            label: t("actions.guide"),
            topic: HELP_TOPICS.parentNotifications,
          },
        ],
      },
    );
    return list;
  }, [hasChild, newsEnabled, t]);

  const tourCopy = useMemo<ParentTourCopy>(
    () => ({
      start: {
        title: t("steps.start.title"),
        navigation: t("tour.start.navigation"),
        todoTitle: t("tour.start.todoTitle"),
        todo: t("tour.start.todo"),
        childTitle: t("tour.start.childTitle"),
        child: t("tour.start.child"),
      },
      childData: {
        title: t("steps.childData.title"),
        navigation: t("tour.childData.navigation"),
        chooseTitle: t("tour.childData.chooseTitle"),
        choose: t("tour.childData.choose"),
        tabTitle: t("tour.childData.tabTitle"),
        tab: t("tour.childData.tab"),
        detailsTitle: t("tour.childData.detailsTitle"),
        details: t("tour.childData.details"),
        careTabTitle: t("tour.childData.careTabTitle"),
        careTab: t("tour.childData.careTab"),
        careTimesTitle: t("tour.childData.careTimesTitle"),
        careTimes: t("tour.childData.careTimes"),
        departureTitle: t("tour.childData.departureTitle"),
        departure: t("tour.childData.departure"),
        contactsTabTitle: t("tour.childData.contactsTabTitle"),
        contactsTab: t("tour.childData.contactsTab"),
        guardiansTitle: t("tour.childData.guardiansTitle"),
        guardians: t("tour.childData.guardians"),
      },
      messagesNews: {
        title: newsEnabled
          ? t("steps.messagesNews.title")
          : t("steps.messages.title"),
        messagesNavigation: t("tour.messagesNews.messagesNavigation"),
        messagesTitle: t("tour.messagesNews.messagesTitle"),
        messages: t("tour.messagesNews.messages"),
        moreTitle: t("tour.common.moreTitle"),
        more: t("tour.common.more"),
        newsNavigation: t("tour.messagesNews.newsNavigation"),
        newsTitle: t("tour.messagesNews.newsTitle"),
        news: t("tour.messagesNews.news"),
      },
      installApp: {
        title: t("steps.installApp.title"),
        settingsNavigation: t("tour.common.settingsNavigation"),
        moreTitle: t("tour.common.moreTitle"),
        more: t("tour.common.more"),
        guideTitle: t("tour.installApp.guideTitle"),
        guide: t("tour.installApp.guide"),
      },
      notifications: {
        title: t("steps.notifications.title"),
        settingsNavigation: t("tour.common.settingsNavigation"),
        moreTitle: t("tour.common.moreTitle"),
        more: t("tour.common.more"),
        topicsTitle: t("tour.notifications.topicsTitle"),
        topics: t("tour.notifications.topics"),
        deviceTitle: t("tour.notifications.deviceTitle"),
        device: t("tour.notifications.device"),
        deviceUnavailable: t("tour.notifications.deviceUnavailable"),
      },
    }),
    [newsEnabled, t],
  );
  const definitions = useMemo(
    () => buildParentTours(tourCopy, { hasChild, newsEnabled }),
    [hasChild, newsEnabled, tourCopy],
  );

  useEffect(() => {
    const stored = readParentFirstStepsState(accountId);
    setState(stored);
    if (!stored.dismissed && !stored.finished && claimFirstOpen(accountId)) {
      setView("checklist");
    }
  }, [accountId]);

  const updateState = useCallback(
    (next: ParentFirstStepsState) => {
      setState(next);
      writeParentFirstStepsState(accountId, next);
    },
    [accountId],
  );

  const onTourFinished = useCallback(
    (key: ParentFirstStepKey) => {
      setState((current) => {
        const next = completeParentFirstStep(current ?? EMPTY_STATE, key);
        writeParentFirstStepsState(accountId, next);
        return next;
      });
      setExpanded(key);
      setView("checklist");
    },
    [accountId],
  );
  const tour = useParentFirstStepsTour(definitions, onTourFinished);

  useEffect(() => {
    if (!state || expanded !== null) return;
    const next = steps.find((step) => !state.completed.includes(step.key));
    setExpanded(next?.key ?? null);
  }, [expanded, state, steps]);

  if (!state || state.dismissed || state.finished) return null;

  const finished = steps.filter((step) =>
    state.completed.includes(step.key),
  ).length;
  const allFinished = finished === steps.length;
  const helpHref = (topic: HelpTopicId) =>
    buildHelpHref(
      {
        role: "parent",
        nfcEnabled: false,
        presenceMode: "detailed",
        groupMode: "fixed_groups",
        returnTo: pathname,
      },
      topic,
    );

  if (tour.active) {
    const active = tour.active;
    return (
      <CoachMark
        searching={!active.target && !active.missing}
        target={active.target}
        title={active.stop.title}
        text={
          active.missing
            ? (active.stop.missingText ?? t("tour.targetMissing"))
            : active.stop.text
        }
        progress={t("tour.progress", {
          current: active.index + 1,
          total: active.total,
        })}
        action={
          active.stop.advance === "click" && active.target
            ? t("tour.chooseMarked")
            : undefined
        }
        onNext={
          active.stop.advance === "next" || active.missing
            ? tour.next
            : undefined
        }
        nextLabel={
          active.index + 1 === active.total
            ? t("actions.done")
            : t("actions.next")
        }
        backLabel={t("actions.back")}
        closeLabel={t("actions.endTour")}
        onBack={active.canGoBack ? tour.back : undefined}
        onClose={() => {
          tour.stop();
          setExpanded(active.key);
          setView("checklist");
        }}
      />
    );
  }

  if (view === "beacon") {
    return (
      <div className="fixed bottom-[calc(5.5rem+env(safe-area-inset-bottom))] left-4 z-40 lg:right-6 lg:bottom-6 lg:left-auto">
        <Button
          type="button"
          variant="success"
          size="md"
          onClick={() => setView("checklist")}
          aria-label={t("open", { count: steps.length - finished })}
          className="gap-2 rounded-full py-2.5 pr-3 pl-4 shadow-lg hover:shadow-xl"
        >
          <ListChecksIcon size={20} weight="bold" aria-hidden />
          <span>{t("shortTitle")}</span>
          <span
            aria-hidden
            className="flex h-6 min-w-6 items-center justify-center rounded-full bg-white px-1.5 text-xs font-semibold text-gray-900"
          >
            {steps.length - finished}
          </span>
          <CaretUpIcon size={16} weight="bold" aria-hidden />
        </Button>
      </div>
    );
  }

  return (
    <>
      <section
        aria-label={t("title")}
        className="moto-popover-surface fixed right-2 bottom-[calc(5.5rem+env(safe-area-inset-bottom))] left-2 z-40 flex max-h-[calc(100dvh-7rem-env(safe-area-inset-bottom))] flex-col overflow-hidden rounded-xl border sm:right-4 sm:left-4 lg:right-6 lg:bottom-6 lg:left-auto lg:w-96"
      >
        <header className="flex flex-col gap-2 border-b border-gray-100 p-4">
          <div className="flex items-center justify-between gap-2">
            <h2 className="text-base font-semibold text-gray-900">
              {t("title")}
            </h2>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              onClick={() => setView("beacon")}
              aria-label={t("collapse")}
            >
              <CaretDownIcon size={18} aria-hidden />
            </Button>
          </div>
          <p className="text-sm text-gray-600">
            {t("progress", { finished, total: steps.length })}
          </p>
          <ProgressBar
            value={finished}
            max={steps.length}
            label={t("progressLabel", { finished, total: steps.length })}
          />
        </header>

        <div className="flex-1 overflow-y-auto p-2">
          {allFinished ? (
            <div className="flex flex-col gap-3 p-2">
              <h3 className="text-base font-semibold text-gray-900">
                {t("complete.title")}
              </h3>
              <p className="text-sm text-gray-700">{t("complete.body")}</p>
              <Button
                type="button"
                size="md"
                onClick={() => updateState({ ...state, finished: true })}
              >
                {t("complete.action")}
              </Button>
            </div>
          ) : (
            <ol className="flex flex-col gap-1">
              {steps.map((step) => {
                const done = state.completed.includes(step.key);
                const isExpanded = expanded === step.key;
                const panelId = `parent-first-step-${step.key}`;
                return (
                  <li key={step.key} className="rounded-lg">
                    <button
                      type="button"
                      className="flex min-h-11 w-full items-center gap-3 rounded-lg px-2 py-2 text-left hover:bg-gray-50"
                      aria-expanded={isExpanded}
                      aria-controls={panelId}
                      onClick={() => setExpanded(isExpanded ? null : step.key)}
                    >
                      {done ? (
                        <CheckCircleIcon
                          size={20}
                          weight="fill"
                          style={{ color: LOCATION_COLORS.GROUP_ROOM }}
                          aria-hidden
                        />
                      ) : (
                        <CircleIcon
                          size={20}
                          className="text-gray-400"
                          aria-hidden
                        />
                      )}
                      <span
                        className={`flex-1 text-sm font-medium ${
                          done ? "text-gray-500" : "text-gray-900"
                        }`}
                      >
                        {step.title}
                      </span>
                      {isExpanded ? (
                        <CaretUpIcon
                          size={16}
                          className="text-gray-400"
                          aria-hidden
                        />
                      ) : (
                        <CaretDownIcon
                          size={16}
                          className="text-gray-400"
                          aria-hidden
                        />
                      )}
                    </button>
                    {isExpanded && (
                      <div
                        id={panelId}
                        className="flex flex-col gap-3 px-2 pb-3 pl-10"
                      >
                        <p className="text-sm text-gray-700">
                          {step.description}
                        </p>
                        <div className="flex flex-wrap gap-2">
                          <Button
                            type="button"
                            size="md"
                            onClick={() => {
                              setView("beacon");
                              tour.start(step.key);
                            }}
                          >
                            {done ? t("actions.showAgain") : t("actions.show")}
                          </Button>
                          {step.help.map((help) => (
                            <ButtonLink
                              key={help.topic}
                              href={helpHref(help.topic)}
                              target="_blank"
                              rel="noopener noreferrer"
                              variant="surface"
                              size="md"
                              onClick={() => {
                                if (step.key !== "installApp") return;
                                updateState(
                                  completeParentFirstStep(state, step.key),
                                );
                              }}
                            >
                              {help.label}
                            </ButtonLink>
                          ))}
                        </div>
                      </div>
                    )}
                  </li>
                );
              })}
            </ol>
          )}
        </div>

        {!allFinished && (
          <footer className="border-t border-gray-100 p-3 text-center">
            <button
              type="button"
              onClick={() => setConfirmDismiss(true)}
              className="min-h-11 px-3 text-sm text-gray-600 underline-offset-4 hover:text-gray-900 hover:underline"
            >
              {t("dismiss.action")}
            </button>
          </footer>
        )}
      </section>

      <ConfirmationModal
        isOpen={confirmDismiss}
        onClose={() => setConfirmDismiss(false)}
        onConfirm={() => {
          updateState({ ...state, dismissed: true });
          setConfirmDismiss(false);
        }}
        title={t("dismiss.title")}
        confirmText={t("dismiss.confirm")}
      >
        <p className="text-sm text-gray-700">{t("dismiss.body")}</p>
      </ConfirmationModal>
    </>
  );
}
