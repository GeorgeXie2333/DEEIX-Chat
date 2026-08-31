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

test("chat composer keeps upload entry points available while queueing messages", () => {
  assert.match(inputSource, /const canSend = [^;]+!uploading/);
  assert.match(inputSource, /disabled=\{loading \|\| uploading\}\s+readOnly=/);
  assert.match(
    inputSource,
    /id="chat-tools-menu-trigger"[\s\S]*?disabled=\{loading \|\| uploading\}/,
  );
  assert.match(areaSource, /const uploadDropDisabled = temporaryMode \|\| loading \|\| uploading/);
  assert.match(areaSource, /onDragEnter=\{onFileDragEnter\}/);
  assert.match(areaSource, /dropActive: temporaryMode \? false : fileDragActive/);
  assert.doesNotMatch(areaSource, /useChatWindowFileDrop|dragUploadTitle|UploadCloud/);
});

test("temporary chat does not expose or accept file uploads", () => {
  assert.match(inputSource, /disabled=\{temporaryMode\}/);
  assert.match(inputSource, /if \(!temporaryMode && files\.length > 0\)/);
  assert.match(inputSource, /\{!temporaryMode \? \(/);
  assert.match(areaSource, /attachments: temporaryMode \? EMPTY_LIST : attachments/);
});

test("chat attachment uploads propagate cancellation and release controllers", () => {
  assert.match(hookSource, /signal: controller\.signal/);
  assert.match(hookSource, /if \(controller\.signal\.aborted \|\| !mountedRef\.current\)/);
  assert.match(hookSource, /uploadControllersRef\.current\.delete\(controller\)/);
});

test("chat tools popover stops pointer and click events before the composer addon", () => {
  const triggerIndex = inputSource.indexOf('id="chat-tools-menu-trigger"');
  const popoverEndIndex = inputSource.indexOf("</PopoverContent>", triggerIndex);
  assert.ok(triggerIndex >= 0);
  assert.ok(popoverEndIndex > triggerIndex);

  const toolsPopoverSource = inputSource.slice(triggerIndex, popoverEndIndex);
  assert.match(toolsPopoverSource, /onPointerDown=\{\(event\) => event\.stopPropagation\(\)\}/);
  assert.match(toolsPopoverSource, /onMouseDown=\{\(event\) => event\.stopPropagation\(\)\}/);
  assert.match(toolsPopoverSource, /onClick=\{\(event\) => event\.stopPropagation\(\)\}/);
});
