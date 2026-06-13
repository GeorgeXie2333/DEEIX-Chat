"use client";

import * as React from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { resolveUploadPolicyRejection } from "@/features/chat/utils/attachments";
import { captureScreenshotFile } from "@/features/chat/utils/browser-media";
import { resolveMaxFilesPerMessage } from "@/features/chat/utils/chat-runtime";
import {
  releaseUploadBatch,
  reserveUploadBatch,
  type UploadReservationState,
} from "@/features/chat/model/upload-reservations";
import type { PendingAttachment } from "@/features/chat/types/chat-runtime";
import { useLocalizedErrorMessage } from "@/i18n/use-localized-error";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";
import {
  getChatFilePolicy,
  getFileProcessingStatus,
  uploadFile,
} from "@/shared/api/file";
import type { ChatFilePolicyDTO } from "@/shared/api/file.types";

function revokeAttachmentPreview(item: PendingAttachment) {
  if (item.previewURL) {
    URL.revokeObjectURL(item.previewURL);
  }
}

export function useChatAttachments({
  conversationKey,
  attachments,
  setAttachments,
  appendAttachmentsForKey,
}: {
  conversationKey: string;
  attachments: PendingAttachment[];
  setAttachments: React.Dispatch<React.SetStateAction<PendingAttachment[]>>;
  appendAttachmentsForKey: (conversationKey: string, items: PendingAttachment[]) => void;
}) {
  const t = useTranslations("chat.attachments");
  const resolveErrorMessage = useLocalizedErrorMessage();
  const [uploadingByKey, setUploadingByKey] = React.useState<UploadReservationState>({});
  const [maxFilesPerMessage, setMaxFilesPerMessage] = React.useState(() => resolveMaxFilesPerMessage());
  const [chatFilePolicy, setChatFilePolicy] = React.useState<ChatFilePolicyDTO | null>(null);
  const attachmentsRef = React.useRef<PendingAttachment[]>(attachments);
  const previousAttachmentsRef = React.useRef<PendingAttachment[]>(attachments);
  const currentConversationKeyRef = React.useRef(conversationKey);
  const uploadingByKeyRef = React.useRef<UploadReservationState>({});
  const settledFileIDsByKeyRef = React.useRef<Record<string, Set<string>>>({});
  const uploadingAttachments = uploadingByKey[conversationKey] ?? [];
  const uploading = uploadingAttachments.length > 0;

  React.useEffect(() => {
    void (async () => {
      try {
        const token = await resolveAccessToken();
        if (!token) {
          return;
        }
        const policy = await getChatFilePolicy(token);
        setChatFilePolicy(policy);
        if (policy.maxMessageFiles > 0) {
          setMaxFilesPerMessage(policy.maxMessageFiles);
        }
      } catch {
        // Keep fallback value.
      }
    })();
  }, []);

  React.useEffect(() => {
    attachmentsRef.current = attachments;
    const settledFileIDs = settledFileIDsByKeyRef.current[conversationKey];
    if (!settledFileIDs) {
      return;
    }
    for (const item of attachments) {
      settledFileIDs.delete(item.fileID);
    }
    if (settledFileIDs.size === 0) {
      delete settledFileIDsByKeyRef.current[conversationKey];
    }
  }, [attachments, conversationKey]);

  React.useEffect(() => {
    currentConversationKeyRef.current = conversationKey;
  }, [conversationKey]);

  React.useEffect(() => {
    const previous = previousAttachmentsRef.current;
    const currentIDs = new Set(attachments.map((item) => item.fileID));
    for (const item of previous) {
      if (!currentIDs.has(item.fileID)) {
        revokeAttachmentPreview(item);
      }
    }
    previousAttachmentsRef.current = attachments;
  }, [attachments]);

  React.useEffect(() => {
    const pending = attachments.filter((item) =>
      item.processingStatus === "uploaded" ||
      item.processingStatus === "queued" ||
      item.processingStatus === "extracting" ||
      item.processingStatus === "embedding",
    );
    if (pending.length === 0) {
      return;
    }
    let cancelled = false;
    const timer = window.setInterval(() => {
      void (async () => {
        try {
          const token = await resolveAccessToken();
          if (!token || cancelled) {
            return;
          }
          const results = await Promise.allSettled(
            pending.map((item) => getFileProcessingStatus(token, item.fileID)),
          );
          if (cancelled) {
            return;
          }
          setAttachments((prev) =>
            prev.map((item) => {
              const index = pending.findIndex((candidate) => candidate.fileID === item.fileID);
              if (index < 0) {
                return item;
              }
              const result = results[index];
              if (!result || result.status !== "fulfilled") {
                return item;
              }
              return {
                ...item,
                detectedMime: result.value.detectedMIME,
                fileCategory: result.value.fileCategory,
                processingStatus: result.value.processingStatus,
                processingReady: result.value.processingReady,
                processingErrorCode: result.value.errorCode,
                processingErrorMessage: result.value.errorMessage,
                extractStatus: result.value.extractStatus,
                embedStatus: result.value.embedStatus,
                ragReady: result.value.ragReady,
                ragReason: result.value.ragReason,
                ocrUsed: result.value.ocrUsed,
              };
            }),
          );
        } catch {
          // Ignore polling failures.
        }
      })();
    }, 1500);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [attachments, setAttachments]);

  const releaseAttachments = React.useCallback((items: PendingAttachment[]) => {
    for (const item of items) {
      revokeAttachmentPreview(item);
    }
  }, []);

  const onRemoveAttachment = React.useCallback((fileID: string) => {
    setAttachments((prev) => {
      const next = prev.filter((item) => item.fileID !== fileID);
      const removed = prev.find((item) => item.fileID === fileID);
      if (removed) {
        revokeAttachmentPreview(removed);
      }
      return next;
    });
  }, [setAttachments]);

  const onUploadFiles = React.useCallback(
    async (files: File[]) => {
      if (files.length === 0) {
        return;
      }
      const targetConversationKey = conversationKey;
      const policyAcceptedFiles: File[] = [];
      const policyLabels = {
        mimeNotAllowed: t("policy.mimeNotAllowed"),
        fullContextLimitExceeded: (limitKB: number) => t("policy.fullContextLimitExceeded", { limit: limitKB }),
        sizeLimitExceeded: (limitKB: number) => t("policy.sizeLimitExceeded", { limit: limitKB }),
      };
      for (const file of files) {
        const rejection = resolveUploadPolicyRejection(file, chatFilePolicy, policyLabels);
        if (rejection) {
          toast.error(t("policyRejected"), {
            description: t("fileRejected", { name: file.name, reason: rejection }),
          });
          continue;
        }
        policyAcceptedFiles.push(file);
      }
      if (policyAcceptedFiles.length === 0) {
        return;
      }

      const currentAttachmentIDs = new Set(attachments.map((item) => item.fileID));
      const settledFileIDs = settledFileIDsByKeyRef.current[targetConversationKey];
      let unsettledAttachmentCount = 0;
      for (const fileID of settledFileIDs ?? []) {
        if (!currentAttachmentIDs.has(fileID)) {
          unsettledAttachmentCount += 1;
        }
      }
      const batchPrefix = `${Date.now()}-${Math.random().toString(16).slice(2)}`;
      const reservation = reserveUploadBatch({
        uploadingByKey: uploadingByKeyRef.current,
        conversationKey: targetConversationKey,
        files: policyAcceptedFiles,
        occupiedAttachmentCount: attachments.length + unsettledAttachmentCount,
        maxFilesPerMessage,
        batchPrefix,
      });
      if (reservation.acceptedFiles.length === 0) {
        toast.error(t("limitReached"), {
          description: t("maxUploadFiles", { count: maxFilesPerMessage }),
        });
        return;
      }
      if (reservation.overflowCount > 0) {
        toast(t("autoTruncated"), {
          description: t("autoTruncatedDescription", {
            max: maxFilesPerMessage,
            count: reservation.overflowCount,
          }),
        });
      }

      const acceptedFiles = reservation.acceptedFiles;
      const placeholders = reservation.placeholders;
      uploadingByKeyRef.current = reservation.nextUploadingByKey;
      setUploadingByKey(reservation.nextUploadingByKey);

      try {
        const token = await resolveAccessToken();
        if (!token) {
          toast.error(t("uploadFailed"), { description: t("uploadSignInRequired") });
          return;
        }

        const results = await Promise.allSettled(
          acceptedFiles.map((file) =>
            uploadFile(token, file, {
              purpose: "conversation_attachment",
            }),
          ),
        );
        const reusedCount = results.filter((result) => result.status === "fulfilled" && result.value.reused).length;

        const uploaded = results.flatMap((result, index) => {
          if (result.status !== "fulfilled") {
            return [];
          }
          const sourceFile = acceptedFiles[index];
          const previewURL = sourceFile.type.startsWith("image/") ? URL.createObjectURL(sourceFile) : undefined;
          return [
            {
              fileID: result.value.file.fileID,
              fileName: result.value.file.fileName,
              mimeType: result.value.file.mimeType,
              detectedMime: result.value.file.detectedMIME,
              fileCategory: result.value.file.fileCategory,
              sizeBytes: result.value.file.sizeBytes,
              previewURL,
              processingStatus: result.value.file.processingStatus,
              processingReady: result.value.file.processingReady,
              processingErrorCode: result.value.file.processingErrorCode,
              processingErrorMessage: result.value.file.processingErrorMessage,
              extractStatus: result.value.file.extractStatus,
              embedStatus: result.value.file.embedStatus,
              ragReady: false,
              ragReason: "",
              ocrUsed: false,
              ragOptOut: result.value.file.ragOptOut,
            },
          ];
        });
        if (uploaded.length > 0) {
          const settledIDs =
            settledFileIDsByKeyRef.current[targetConversationKey] ??
            new Set<string>();
          const existingIDs = new Set(
            currentConversationKeyRef.current === targetConversationKey
              ? attachmentsRef.current.map((item) => item.fileID)
              : [],
          );
          for (const fileID of settledIDs) {
            existingIDs.add(fileID);
          }
          const nextUploaded = uploaded.filter((item) => {
            if (existingIDs.has(item.fileID)) {
              revokeAttachmentPreview(item);
              return false;
            }
            existingIDs.add(item.fileID);
            settledIDs.add(item.fileID);
            return true;
          });
          if (settledIDs.size > 0) {
            settledFileIDsByKeyRef.current[targetConversationKey] = settledIDs;
          }
          appendAttachmentsForKey(targetConversationKey, nextUploaded);
          if (currentConversationKeyRef.current !== targetConversationKey) {
            releaseAttachments(uploaded);
          }
        }
        if (reusedCount > 0) {
          toast.success(t("duplicateReused"));
        }
        if (uploaded.length < acceptedFiles.length) {
          toast.error(t("partialUploadFailed"), { description: t("retryFailedFiles") });
        }
      } catch (error) {
        const description = resolveErrorMessage(error, t("retryLater"));
        toast.error(t("uploadFailed"), { description });
      } finally {
        const nextUploadingByKey = releaseUploadBatch(
          uploadingByKeyRef.current,
          targetConversationKey,
          placeholders.map((item) => item.tempID),
        );
        uploadingByKeyRef.current = nextUploadingByKey;
        setUploadingByKey(nextUploadingByKey);
        if (
          currentConversationKeyRef.current !== targetConversationKey &&
          (nextUploadingByKey[targetConversationKey]?.length ?? 0) === 0
        ) {
          delete settledFileIDsByKeyRef.current[targetConversationKey];
        }
      }
    },
    [
      appendAttachmentsForKey,
      attachments,
      chatFilePolicy,
      conversationKey,
      maxFilesPerMessage,
      releaseAttachments,
      resolveErrorMessage,
      t,
    ],
  );

  const onCaptureScreenshot = React.useCallback(async () => {
    if (typeof navigator === "undefined" || !navigator.mediaDevices?.getDisplayMedia) {
      toast.error(t("screenshotUnsupported"));
      return;
    }

    let stream: MediaStream | null = null;
    try {
      stream = await navigator.mediaDevices.getDisplayMedia({
        video: true,
        audio: false,
      });
      const screenshot = await captureScreenshotFile(stream);
      await onUploadFiles([screenshot]);
    } catch {
      toast.error(t("screenshotFailed"), { description: t("retry") });
    } finally {
      stream?.getTracks().forEach((track) => track.stop());
    }
  }, [onUploadFiles, t]);

  React.useEffect(() => {
    return () => {
      for (const item of attachmentsRef.current) {
        revokeAttachmentPreview(item);
      }
    };
  }, []);

  return {
    attachments,
    uploading,
    uploadingAttachments,
    maxFilesPerMessage,
    fileMode: chatFilePolicy?.fileMode ?? "auto",
    releaseAttachments,
    onRemoveAttachment,
    onUploadFiles,
    onCaptureScreenshot,
  };
}
