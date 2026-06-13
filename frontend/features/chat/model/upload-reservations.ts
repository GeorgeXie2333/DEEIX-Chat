import type { UploadingAttachment } from "@/features/chat/types/chat-runtime";

type UploadCandidate = {
  name: string;
  size: number;
};

export type UploadReservationState = Record<string, UploadingAttachment[]>;

export function reserveUploadBatch<T extends UploadCandidate>({
  uploadingByKey,
  conversationKey,
  files,
  occupiedAttachmentCount,
  maxFilesPerMessage,
  batchPrefix,
}: {
  uploadingByKey: UploadReservationState;
  conversationKey: string;
  files: T[];
  occupiedAttachmentCount: number;
  maxFilesPerMessage: number;
  batchPrefix: string;
}) {
  const currentUploads = uploadingByKey[conversationKey] ?? [];
  const remainingSlots = Math.max(
    0,
    maxFilesPerMessage - Math.max(0, occupiedAttachmentCount) - currentUploads.length,
  );
  const acceptedFiles = files.slice(0, remainingSlots);
  const placeholders = acceptedFiles.map((file, index) => ({
    tempID: `${batchPrefix}-${index}`,
    fileName: file.name,
    sizeBytes: file.size,
  }));

  return {
    acceptedFiles,
    placeholders,
    overflowCount: files.length - acceptedFiles.length,
    nextUploadingByKey:
      placeholders.length === 0
        ? uploadingByKey
        : {
            ...uploadingByKey,
            [conversationKey]: [...currentUploads, ...placeholders],
          },
  };
}

export function releaseUploadBatch(
  uploadingByKey: UploadReservationState,
  conversationKey: string,
  tempIDs: string[],
): UploadReservationState {
  const currentUploads = uploadingByKey[conversationKey] ?? [];
  if (currentUploads.length === 0 || tempIDs.length === 0) {
    return uploadingByKey;
  }

  const releasedIDs = new Set(tempIDs);
  const nextUploads = currentUploads.filter((item) => !releasedIDs.has(item.tempID));
  if (nextUploads.length === currentUploads.length) {
    return uploadingByKey;
  }
  if (nextUploads.length === 0) {
    const { [conversationKey]: _released, ...rest } = uploadingByKey;
    return rest;
  }
  return {
    ...uploadingByKey,
    [conversationKey]: nextUploads,
  };
}
