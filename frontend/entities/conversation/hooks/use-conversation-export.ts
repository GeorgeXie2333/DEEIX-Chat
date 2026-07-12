"use client";

import * as React from "react";
import { toast } from "sonner";

import { downloadConversationArchive } from "@/features/recent/utils/conversation-archive";
import { exportConversationArchive } from "@/shared/api/conversation";
import { resolveAccessToken } from "@/shared/auth/resolve-access-token";

type UseConversationExportOptions = {
  successMessage: string;
  failureMessage: string;
};

export function useConversationExport({
  successMessage,
  failureMessage,
}: UseConversationExportOptions) {
  return React.useCallback(
    async (conversationPublicID: string) => {
      const token = await resolveAccessToken();
      if (!token) {
        return;
      }

      try {
        const data = await exportConversationArchive(token, conversationPublicID);
        downloadConversationArchive(data);
        toast.success(successMessage);
      } catch (error) {
        toast.error(failureMessage, {
          description: error instanceof Error ? error.message : undefined,
        });
      }
    },
    [failureMessage, successMessage],
  );
}
