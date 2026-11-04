"use client";

import { useEffect, useRef } from "react";
import { useSWRAuth } from "~/lib/swr";
import { useDiscardRefusedEntity } from "~/lib/swr/hooks";
import { useOptimisticAction } from "~/lib/swr/optimistic";
import { parentKeys } from "~/lib/swr/parent-keys";
import { MEAL_PARTICIPATION_REFUSALS } from "~/lib/everyday-action-refusals";
import {
  clearMealParticipationDay,
  getMealParticipation,
  replaceMealParticipationSchedule,
  setMealParticipationDay,
  setMealParticipationWeek,
  type MealParticipation,
} from "~/lib/parent-api";

type Day = MealParticipation["days"][number];

export function useMealParticipation(
  schoolId: string,
  studentId: string,
  from: string,
  to: string,
  asOf = from,
) {
  const discard = useDiscardRefusedEntity();
  const key = parentKeys.mealParticipation(schoolId, studentId, from, to, asOf);
  const reader = useSWRAuth(
    key,
    () => getMealParticipation(studentId, from, to),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
    },
  );
  useEffect(() => {
    // Discard after SWR stores the read error, so the refusal stays visible.
    discard(reader.error, {
      codes: MEAL_PARTICIPATION_REFUSALS,
      matches: (candidate) =>
        candidate.startsWith(
          parentKeys.mealParticipationFamily(schoolId, studentId),
        ),
    });
  }, [discard, reader.error, schoolId, studentId]);
  const action = useOptimisticAction(key, reader.data);
  const inFlight = useRef(false);
  async function change<R>(
    update: (data: MealParticipation) => MealParticipation,
    request: () => Promise<R>,
    confirmed: (
      data: MealParticipation,
      result: R,
    ) => MealParticipation = update,
  ) {
    if (inFlight.current || !reader.data)
      return { status: "discarded" } as const;
    inFlight.current = true;
    try {
      const outcome = await action.run({
        optimistic: update,
        request,
        commit: (result) => {
          void reader.mutate((data) => data && confirmed(data, result), {
            revalidate: true,
          });
        },
        refusal: {
          codes: MEAL_PARTICIPATION_REFUSALS,
          matches: (candidate) =>
            candidate.startsWith(
              parentKeys.mealParticipationFamily(schoolId, studentId),
            ),
        },
      });
      return outcome;
    } finally {
      inFlight.current = false;
    }
  }
  const days =
    (dates: readonly string[], participating: boolean | null) =>
    (data: MealParticipation): MealParticipation => ({
      ...data,
      days: data.days.map((day): Day => {
        if (
          !dates.includes(day.date) ||
          day.source === "closed" ||
          day.source === "sick" ||
          day.source === "absent"
        )
          return day;
        const next = participating ?? day.regular_participating;
        if (
          next !== day.participating &&
          !(next ? day.can_register : day.can_cancel)
        )
          return day;
        return {
          ...day,
          participating: next,
          source:
            participating === null ? (next ? "regular" : "none") : "override",
        };
      }),
    });
  return {
    ...reader,
    data: action.data,
    pending: action.pending,
    saveDay: (date: string, participating: boolean | null) =>
      change(days([date], participating), () =>
        participating === null
          ? clearMealParticipationDay(studentId, date)
          : setMealParticipationDay(studentId, date, participating),
      ),
    saveWeek: (dates: string[], monday: string, participating: boolean) =>
      change(
        days(dates, participating),
        () => setMealParticipationWeek(studentId, monday, participating),
        (data, result) => days(result.changed, participating)(data),
      ),
    saveSchedule: (weekdays: number[]) =>
      change(
        (data) => ({ ...data, weekdays }),
        () => replaceMealParticipationSchedule(studentId, weekdays),
        (data, result) => ({
          ...data,
          weekdays,
          effective_from: result.effective_from,
        }),
      ),
  };
}
