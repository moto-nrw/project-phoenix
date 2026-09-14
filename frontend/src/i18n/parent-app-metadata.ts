import type { AppLocale } from "./locales";

const parentAppMetadata = {
  de: {
    name: "moto Eltern",
    description:
      "Die Betreuung Ihres Kindes im Blick: Tagesstatus, Nachrichten an die OGS, Krankmeldung und Termine.",
  },
  en: {
    name: "moto Parent Portal",
    description:
      "Keep track of your child's care: daily status, messages, absence reports and appointments.",
  },
  ru: {
    name: "moto для родителей",
    description:
      "Всё об уходе за ребёнком: статус дня, сообщения, уведомления об отсутствии и встречи.",
  },
  sq: {
    name: "moto për prindërit",
    description:
      "Kujdesi i fëmijës suaj në një vend: statusi ditor, mesazhet, mungesat dhe takimet.",
  },
  pl: {
    name: "moto dla rodziców",
    description:
      "Opieka nad Państwa dzieckiem w jednym miejscu: status dnia, wiadomości do OGS, zgłoszenie choroby i terminy.",
  },
  tr: {
    name: "moto Veli",
    description:
      "Çocuğunuzun bakımı tek yerde: günlük durum, OGS'ye mesajlar, hastalık bildirimi ve randevular.",
  },
  uk: {
    name: "moto для батьків",
    description:
      "Догляд за вашою дитиною в одному місці: статус дня, повідомлення для OGS, повідомлення про хворобу та події.",
  },
} as const satisfies Record<
  AppLocale,
  { readonly name: string; readonly description: string }
>;

export function getParentAppMetadata(locale: AppLocale) {
  return parentAppMetadata[locale];
}
