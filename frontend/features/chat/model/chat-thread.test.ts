import assert from "node:assert/strict";
import test from "node:test";

import type { MessageDTO } from "@/shared/api/conversation.types";
import { mapServerMessage } from "./chat-thread.ts";

test("mapServerMessage preserves an external live activity label", () => {
  const item = {
    publicID: "assistant-1",
    role: "assistant",
    runID: "run-1",
    status: "pending",
    contentType: "text",
    content: "",
  } as unknown as MessageDTO;

  const mapped = mapServerMessage(
    item,
    { generationInterrupted: "Generation interrupted" },
    {
      liveRunIDs: new Set(["run-1"]),
      liveActivityLabels: new Map([["run-1", "Checking moderation"]]),
    },
  );

  assert.equal(mapped.activityLabel, "Checking moderation");
});
