import { useMemo } from "react";
import { useTranslations } from "next-intl";

import type { ComposerAttachmentLabels } from "~/components/messaging/message-composer";
import type { AttachmentViewerLabels } from "~/components/ui/attachment-viewer";
import type { MessageAttachmentLabels } from "~/components/ui/message-attachments";
import {
  MAX_MESSAGE_ATTACHMENT_MB,
  MAX_MESSAGE_ATTACHMENTS,
} from "~/lib/message-attachments";

/**
 * Die Texte der Anhänge in der Sprache der Eltern-App (#6685): für das
 * Eingabefeld und für die Fotos und Dateien einer Nachricht.
 */
export function useParentAttachmentLabels(): {
  composer: ComposerAttachmentLabels;
  bubble: MessageAttachmentLabels;
} {
  const t = useTranslations("messageAttachments");
  return useMemo(() => {
    const limits = {
      max: MAX_MESSAGE_ATTACHMENTS,
      mb: MAX_MESSAGE_ATTACHMENT_MB,
    };
    const viewer: AttachmentViewerLabels = {
      close: t("close"),
      previous: t("previous"),
      next: t("next"),
      download: t("download"),
      openInNewTab: t("openInNewTab"),
      notViewable: t("notViewable"),
      notViewableOnDevice: t("notViewableOnDevice"),
      fileObject: t("fileObject"),
      position: (current, total) => t("position", { current, total }),
    };
    return {
      composer: {
        attach: t("attach"),
        attachHint: t("attachHint", limits),
        drop: t("drop"),
        remove: (filename) => t("remove", { filename }),
        uploading: (filename) => t("uploading", { filename }),
        open: (filename) => t("openFile", { filename }),
        viewer,
        tooMany: t("tooMany", limits),
        tooLarge: (filename) => t("tooLarge", { filename, ...limits }),
        wrongType: (filename) => t("wrongType", { filename }),
        fileObject: t("fileObject"),
      },
      bubble: {
        openImage: (filename) => t("openImage", { filename }),
        openFile: (filename) => t("openFile", { filename }),
        downloadFile: (filename) => t("downloadFile", { filename }),
        moreImages: (count) => t("moreImages", { count }),
        lightbox: viewer,
      },
    };
  }, [t]);
}
