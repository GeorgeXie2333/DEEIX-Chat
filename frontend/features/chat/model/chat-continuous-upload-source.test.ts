import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const hookSource = readFileSync(
  new URL("../hooks/use-chat-attachments.ts", import.meta.url),
  "utf8",
);
const inputSource = readFileSync(
  new URL("../components/sections/chat-input.tsx", import.meta.url),
  "utf8",
);
const areaSource = readFileSync(
  new URL("../components/app-chat-area.tsx", import.meta.url),
  "utf8",
);

test("chat attachment uploads accept another batch while uploads are active", () => {
  assert.doesNotMatch(hookSource, /files\.length === 0 \|\| uploading/);
  assert.match(hookSource, /uploadingByKeyRef/);
  assert.match(hookSource, /reserveUploadBatch/);
});

test("chat composer keeps upload entry points available while sending stays blocked", () => {
  assert.match(inputSource, /const canSend = [^;]+!uploading/);
  assert.match(inputSource, /disabled=\{sending \|\| loading\}\s+readOnly=/);
  assert.match(
    inputSource,
    /id="chat-tools-menu-trigger"[^]*?disabled=\{sending \|\| loading\}/,
  );
  assert.match(areaSource, /const dragUploadDisabled = loading \|\| generating \|\| isConversationLoadFailed/);
});
