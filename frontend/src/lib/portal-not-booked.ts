import type { AppLocale } from "~/i18n/locales";

/**
 * The portals that follow a module of their own (#4895): the parents portal
 * follows Eltern-App, the school portal Unterricht am Vormittag. The OGS
 * team portal is Basis; its pages explain a missing module themselves
 * (`moduleNotBookedExplanation`).
 */
export type ModulePortal = "staff" | "parent" | "school";

/**
 * Parents read the explanation in their portal language. They cannot book a
 * module, so the text names their school as the next step, not the moto team.
 */
const PARENT_PORTAL_NOT_BOOKED: Record<AppLocale, string> = {
  de: "Ihre Schule nutzt das Elternportal zurzeit nicht. Bei Fragen wenden Sie sich bitte an Ihre Schule.",
  en: "Your school is not using the parent portal at the moment. If you have questions, please contact your school.",
  pl: "Szkoła Państwa obecnie nie korzysta z portalu dla rodziców. W razie pytań proszę skontaktować się ze szkołą.",
  ru: "Ваша школа сейчас не пользуется родительским порталом. Если у вас есть вопросы, обратитесь, пожалуйста, в школу.",
  sq: "Shkolla juaj aktualisht nuk e përdor portalin e prindërve. Për pyetje, ju lutemi drejtohuni shkollës suaj.",
  tr: "Okulunuz şu anda veli portalını kullanmıyor. Sorularınız için lütfen okulunuza başvurun.",
  uk: "Ваша школа зараз не користується батьківським порталом. Якщо у вас є запитання, зверніться, будь ласка, до школи.",
};

/** The school portal is German only. */
const SCHOOL_PORTAL_NOT_BOOKED =
  "Ihre Schule nutzt moto schule zurzeit nicht. Bei Fragen wenden Sie sich bitte an Ihre OGS-Verwaltung.";

/**
 * Why the parents or the school portal refuses its user: the school has not
 * booked the portal's module. Shown at the login and when a request of an
 * open session is refused with `module.not_booked`.
 */
export function portalNotBookedExplanation(
  portal: Exclude<ModulePortal, "staff">,
  locale: AppLocale,
): string {
  return portal === "parent"
    ? PARENT_PORTAL_NOT_BOOKED[locale]
    : SCHOOL_PORTAL_NOT_BOOKED;
}
