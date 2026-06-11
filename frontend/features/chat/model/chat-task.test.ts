import assert from "node:assert/strict";
import test from "node:test";

import { resolveChatSubmitDecision } from "./chat-task.ts";
import type { ChatModelOption, PendingAttachment } from "../types/chat-runtime.ts";

const videoModel: ChatModelOption = {
  platformModelName: "sora-2",
  description: "",
  icon: "",
  vendor: "openai",
  kinds: ["video_gen"],
  protocols: ["openai_video_generations"],
  defaultOptions: {},
  optionControls: [],
  nativeToolKeys: [],
  nativeTools: [],
  pricing: null,
};

function attachment(fileName: string, mimeType: string, fileCategory?: string): PendingAttachment {
  return {
    fileID: `file_${fileName}`,
    fileName,
    mimeType,
    fileCategory,
    sizeBytes: 100,
  };
}

test("resolveChatSubmitDecision routes video models to video generation", () => {
  assert.deepEqual(resolveChatSubmitDecision(videoModel, []).task, "video_generation");
  assert.equal(resolveChatSubmitDecision(videoModel, [attachment("ref.png", "image/png", "image")]).blockedReason, null);
  assert.equal(
    resolveChatSubmitDecision(videoModel, [
      attachment("one.png", "image/png", "image"),
      attachment("two.png", "image/png", "image"),
    ]).blockedReason,
    "video_generation_too_many_reference_images",
  );
  assert.equal(
    resolveChatSubmitDecision(videoModel, [attachment("notes.txt", "text/plain", "text")]).blockedReason,
    "video_generation_rejects_non_image_attachments",
  );
});
