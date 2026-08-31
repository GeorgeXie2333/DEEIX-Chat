import assert from "node:assert/strict";
import test from "node:test";
import type { ChatModelOption, PendingAttachment } from "../types/chat-runtime.ts";
import { resolveChatSubmitDecision } from "./chat-task.ts";

const videoModel: ChatModelOption = {
  platformModelName: "sora-2",
  description: "",
  icon: "",
  vendor: "openai",
  vendorName: "OpenAI",
  vendorIcon: "",
  displayGroupID: null,
  displayGroupName: "",
  displayGroupIcon: "",
  kinds: ["video_gen"],
  protocols: ["openai_video_generations"],
  defaultOptions: {},
  optionControls: [],
  lockedOptionPaths: [],
  nativeToolKeys: [],
  nativeTools: [],
  pricing: null,
  videoExtension: null,
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

test("resolveChatSubmitDecision routes a single MP4 to video extension first", () => {
  const extensionModel: ChatModelOption = {
    ...videoModel,
    kinds: ["chat", "video_gen", "video_extension"],
    protocols: ["xai_video", "xai_video_extensions"],
    videoExtension: { enabled: true, defaultOptions: { duration: 6 }, optionControls: [] },
  };
  const mp4 = attachment("clip.mp4", "video/mp4", "video");

  assert.deepEqual(resolveChatSubmitDecision(extensionModel, [mp4]).task, "video_extension");
  assert.equal(resolveChatSubmitDecision(extensionModel, [mp4]).blockedReason, null);
  assert.equal(
    resolveChatSubmitDecision({ ...extensionModel, videoExtension: null }, [mp4]).blockedReason,
    "video_extension_unsupported",
  );
  assert.equal(
    resolveChatSubmitDecision(extensionModel, [mp4, attachment("ref.png", "image/png", "image")]).blockedReason,
    "video_extension_requires_single_mp4",
  );
});
