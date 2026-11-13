/**
 * Which palette tone a count carries, by who it is about. The kit's
 * `NotificationBadge` knows only palette names (#6582); the audience colours
 * live here once: staff orange, parents blue, feedback coral.
 */
export const COUNTER_TONE = {
  staff: "orange",
  parents: "blue",
  feedback: "coral",
} as const;

export type CounterAudience = keyof typeof COUNTER_TONE;
