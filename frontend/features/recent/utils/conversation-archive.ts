import type { ConversationArchiveDTO } from "@/shared/api/conversation.types";

export const CONVERSATION_ARCHIVE_SCHEMA = "deeix-chat.conversation.v1";
export const CONVERSATION_ARCHIVE_MAX_BYTES = 10 * 1024 * 1024;

function sanitizeArchiveFilenamePart(value: string): string {
  return value
    .trim()
    .replace(/[\\/:*?"<>|]+/g, "-")
    .replace(/\s+/g, " ")
    .slice(0, 64)
    .trim() || "conversation";
}

export function conversationArchiveFilename(archive: ConversationArchiveDTO, fallbackTitle?: string): string {
  const title = sanitizeArchiveFilenamePart(archive.conversation?.title || fallbackTitle || "conversation");
  const timestampSource = archive.exportedAt || new Date().toISOString();
  const date = new Date(timestampSource);
  const timestamp = (Number.isNaN(date.getTime()) ? new Date() : date).toISOString().replace(/[:.]/g, "-");
  return `${title}.${timestamp}.deeix-chat.json`;
}

export function downloadConversationArchive(archive: ConversationArchiveDTO, fallbackTitle?: string): void {
  const blob = new Blob([JSON.stringify(archive, null, 2)], {
    type: "application/json;charset=utf-8",
  });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = conversationArchiveFilename(archive, fallbackTitle);
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

export async function readConversationArchiveFile(file: File): Promise<ConversationArchiveDTO> {
  if (file.size > CONVERSATION_ARCHIVE_MAX_BYTES) {
    throw new Error("conversation archive file too large");
  }
  const text = await file.text();
  const parsed = JSON.parse(text) as unknown;
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error("conversation archive must be a JSON object");
  }
  const archive = parsed as ConversationArchiveDTO;
  if (archive.schema !== CONVERSATION_ARCHIVE_SCHEMA || !Array.isArray(archive.messages)) {
    throw new Error("invalid conversation archive");
  }
  return archive;
}
