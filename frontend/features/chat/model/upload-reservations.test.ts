import assert from "node:assert/strict";
import test from "node:test";

import {
  releaseUploadBatch,
  reserveUploadBatch,
  type UploadReservationState,
} from "./upload-reservations.ts";

type TestFile = {
  name: string;
  size: number;
};

function file(name: string): TestFile {
  return { name, size: 100 };
}

test("reserveUploadBatch shares remaining attachment slots across concurrent batches", () => {
  const initial: UploadReservationState = {};
  const first = reserveUploadBatch({
    uploadingByKey: initial,
    conversationKey: "conversation-1",
    files: [file("one.txt"), file("two.txt")],
    occupiedAttachmentCount: 1,
    maxFilesPerMessage: 4,
    batchPrefix: "first",
  });
  const second = reserveUploadBatch({
    uploadingByKey: first.nextUploadingByKey,
    conversationKey: "conversation-1",
    files: [file("three.txt"), file("four.txt")],
    occupiedAttachmentCount: 1,
    maxFilesPerMessage: 4,
    batchPrefix: "second",
  });

  assert.deepEqual(first.acceptedFiles.map((item) => item.name), ["one.txt", "two.txt"]);
  assert.deepEqual(second.acceptedFiles.map((item) => item.name), ["three.txt"]);
  assert.equal(second.overflowCount, 1);
  assert.deepEqual(
    second.nextUploadingByKey["conversation-1"]?.map((item) => item.tempID),
    ["first-0", "first-1", "second-0"],
  );
});

test("releaseUploadBatch removes only the completed batch placeholders", () => {
  const first = reserveUploadBatch({
    uploadingByKey: {},
    conversationKey: "conversation-1",
    files: [file("one.txt")],
    occupiedAttachmentCount: 0,
    maxFilesPerMessage: 3,
    batchPrefix: "first",
  });
  const second = reserveUploadBatch({
    uploadingByKey: first.nextUploadingByKey,
    conversationKey: "conversation-1",
    files: [file("two.txt")],
    occupiedAttachmentCount: 0,
    maxFilesPerMessage: 3,
    batchPrefix: "second",
  });

  const released = releaseUploadBatch(
    second.nextUploadingByKey,
    "conversation-1",
    first.placeholders.map((item) => item.tempID),
  );

  assert.deepEqual(released["conversation-1"]?.map((item) => item.tempID), ["second-0"]);
});
