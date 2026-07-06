"use client";

import * as React from "react";

import type { PendingExchange } from "@/features/chat/types/chat-runtime";
import { clearLiveUpstreamThinkTrace, upsertLiveUpstreamThinkTrace } from "@/features/chat/model/upstream-think-store";
import type { StreamMessageEvent } from "@/shared/api/conversation.types";

const STREAM_TEXT_FLUSH_INTERVAL_MS = 50;
const STREAM_THINK_FLUSH_INTERVAL_MS = 40;
const STREAM_THINK_BASE_CHARS_PER_FLUSH = 48;
const STREAM_THINK_CATCHUP_THRESHOLD = 1024;
const STREAM_THINK_CATCHUP_CHARS_PER_FLUSH = 256;

type UpstreamThinkDeltaEvent = Extract<StreamMessageEvent, { type: "upstream_think_delta" }>;

function resolveThinkFlushSize(pendingLength: number) {
  if (pendingLength > STREAM_THINK_CATCHUP_THRESHOLD) {
    return Math.min(pendingLength, STREAM_THINK_CATCHUP_CHARS_PER_FLUSH);
  }
  return Math.min(pendingLength, STREAM_THINK_BASE_CHARS_PER_FLUSH);
}

export function useChatStreamBuffer({
  setPendingExchange,
}: {
  setPendingExchange: React.Dispatch<React.SetStateAction<PendingExchange | null>>;
}) {
  const streamingMessageKeyRef = React.useRef<string | null>(null);
  const streamingRunIDRef = React.useRef<string | null>(null);
  const pendingStreamTextRef = React.useRef("");
  const streamFlushFrameRef = React.useRef<number | null>(null);
  const streamFlushTimeoutRef = React.useRef<number | null>(null);
  const lastStreamFlushAtRef = React.useRef(0);
  const pendingThinkDeltaRef = React.useRef("");
  const pendingThinkEventRef = React.useRef<UpstreamThinkDeltaEvent | null>(null);
  const thinkFlushFrameRef = React.useRef<number | null>(null);
  const thinkFlushTimeoutRef = React.useRef<number | null>(null);
  const lastThinkFlushAtRef = React.useRef(0);
  const scheduleThinkFlushRef = React.useRef<(() => void) | null>(null);

  const flushStreamText = React.useCallback(function flushStreamText() {
    streamFlushFrameRef.current = null;
    lastStreamFlushAtRef.current = performance.now();
    const pendingText = pendingStreamTextRef.current;
    const exchangeKey = streamingMessageKeyRef.current;
    if (!exchangeKey || !pendingText) {
      return;
    }
    pendingStreamTextRef.current = "";

    setPendingExchange((prev) => {
      if (!prev || prev.key !== exchangeKey) {
        return prev;
      }
      return {
        ...prev,
        assistantPending: false,
        assistantStreaming: true,
        assistantText: prev.assistantText + pendingText,
      };
    });
  }, [setPendingExchange]);

  const flushUpstreamThink = React.useCallback(function flushUpstreamThink() {
    thinkFlushFrameRef.current = null;
    lastThinkFlushAtRef.current = performance.now();
    const runID = streamingRunIDRef.current;
    const pendingEvent = pendingThinkEventRef.current;
    if (!runID || !pendingEvent) {
      return;
    }

    const pendingDelta = pendingThinkDeltaRef.current;
    const flushSize = resolveThinkFlushSize(pendingDelta.length);
    const delta = flushSize > 0 ? pendingDelta.slice(0, flushSize) : "";
    pendingThinkDeltaRef.current = flushSize > 0 ? pendingDelta.slice(flushSize) : "";
    if (!pendingThinkDeltaRef.current) {
      pendingThinkEventRef.current = null;
    }

    const event: UpstreamThinkDeltaEvent = {
      ...pendingEvent,
      delta,
      contentMarkdown: flushSize > 0 ? undefined : pendingEvent.contentMarkdown,
    };

    upsertLiveUpstreamThinkTrace(runID, event);

    if (pendingThinkDeltaRef.current) {
      scheduleThinkFlushRef.current?.();
    }
  }, []);

  const scheduleStreamFlush = React.useCallback(() => {
    if (streamFlushFrameRef.current !== null || streamFlushTimeoutRef.current !== null) {
      return;
    }

    const elapsed = performance.now() - lastStreamFlushAtRef.current;
    if (elapsed >= STREAM_TEXT_FLUSH_INTERVAL_MS) {
      streamFlushFrameRef.current = window.requestAnimationFrame(flushStreamText);
      return;
    }

    streamFlushTimeoutRef.current = window.setTimeout(() => {
      streamFlushTimeoutRef.current = null;
      streamFlushFrameRef.current = window.requestAnimationFrame(flushStreamText);
    }, STREAM_TEXT_FLUSH_INTERVAL_MS - elapsed);
  }, [flushStreamText]);

  const enqueueStreamText = React.useCallback(
    (delta: string) => {
      if (!delta) {
        return;
      }
      pendingStreamTextRef.current += delta;
      scheduleStreamFlush();
    },
    [scheduleStreamFlush],
  );

  const scheduleUpstreamThinkFlush = React.useCallback(() => {
    if (thinkFlushFrameRef.current !== null || thinkFlushTimeoutRef.current !== null) {
      return;
    }

    const elapsed = performance.now() - lastThinkFlushAtRef.current;
    if (elapsed >= STREAM_THINK_FLUSH_INTERVAL_MS) {
      thinkFlushFrameRef.current = window.requestAnimationFrame(flushUpstreamThink);
      return;
    }

    thinkFlushTimeoutRef.current = window.setTimeout(() => {
      thinkFlushTimeoutRef.current = null;
      thinkFlushFrameRef.current = window.requestAnimationFrame(flushUpstreamThink);
    }, STREAM_THINK_FLUSH_INTERVAL_MS - elapsed);
  }, [flushUpstreamThink]);

  React.useEffect(() => {
    scheduleThinkFlushRef.current = scheduleUpstreamThinkFlush;
  }, [scheduleUpstreamThinkFlush]);

  const enqueueUpstreamThinkDelta = React.useCallback(
    (event: UpstreamThinkDeltaEvent) => {
      if (event.trace?.enabled || typeof event.contentMarkdown === "string") {
        pendingThinkDeltaRef.current = "";
        pendingThinkEventRef.current = event;
        scheduleUpstreamThinkFlush();
        return;
      }
      if (event.delta) {
        pendingThinkDeltaRef.current += event.delta;
      }
      pendingThinkEventRef.current = {
        ...event,
        delta: "",
      };
      scheduleUpstreamThinkFlush();
    },
    [scheduleUpstreamThinkFlush],
  );

  const startStream = React.useCallback((exchangeKey: string, runID?: string) => {
    pendingStreamTextRef.current = "";
    pendingThinkDeltaRef.current = "";
    pendingThinkEventRef.current = null;
    streamingMessageKeyRef.current = exchangeKey;
    streamingRunIDRef.current = runID?.trim() || null;
    clearLiveUpstreamThinkTrace(streamingRunIDRef.current);
    lastStreamFlushAtRef.current = 0;
    lastThinkFlushAtRef.current = 0;
  }, []);

  const flushStreamTextNow = React.useCallback(() => {
    if (streamFlushFrameRef.current !== null) {
      window.cancelAnimationFrame(streamFlushFrameRef.current);
      streamFlushFrameRef.current = null;
    }
    if (streamFlushTimeoutRef.current !== null) {
      window.clearTimeout(streamFlushTimeoutRef.current);
      streamFlushTimeoutRef.current = null;
    }
    flushStreamText();
  }, [flushStreamText]);

  const flushUpstreamThinkNow = React.useCallback(() => {
    if (thinkFlushFrameRef.current !== null) {
      window.cancelAnimationFrame(thinkFlushFrameRef.current);
      thinkFlushFrameRef.current = null;
    }
    if (thinkFlushTimeoutRef.current !== null) {
      window.clearTimeout(thinkFlushTimeoutRef.current);
      thinkFlushTimeoutRef.current = null;
    }
    const runID = streamingRunIDRef.current;
    const pendingEvent = pendingThinkEventRef.current;
    if (!runID || !pendingEvent) {
      return;
    }
    const event: UpstreamThinkDeltaEvent = {
      ...pendingEvent,
      delta: pendingThinkDeltaRef.current,
      contentMarkdown: pendingThinkDeltaRef.current ? undefined : pendingEvent.contentMarkdown,
    };
    pendingThinkDeltaRef.current = "";
    pendingThinkEventRef.current = null;

    upsertLiveUpstreamThinkTrace(runID, event);
  }, []);

  const resetStreamBuffer = React.useCallback(() => {
    if (streamFlushFrameRef.current !== null) {
      window.cancelAnimationFrame(streamFlushFrameRef.current);
      streamFlushFrameRef.current = null;
    }
    if (streamFlushTimeoutRef.current !== null) {
      window.clearTimeout(streamFlushTimeoutRef.current);
      streamFlushTimeoutRef.current = null;
    }
    if (thinkFlushFrameRef.current !== null) {
      window.cancelAnimationFrame(thinkFlushFrameRef.current);
      thinkFlushFrameRef.current = null;
    }
    if (thinkFlushTimeoutRef.current !== null) {
      window.clearTimeout(thinkFlushTimeoutRef.current);
      thinkFlushTimeoutRef.current = null;
    }
    pendingStreamTextRef.current = "";
    pendingThinkDeltaRef.current = "";
    pendingThinkEventRef.current = null;
    streamingMessageKeyRef.current = null;
    streamingRunIDRef.current = null;
  }, []);

  React.useEffect(() => {
    return () => {
      resetStreamBuffer();
    };
  }, [resetStreamBuffer]);

  return {
    enqueueUpstreamThinkDelta,
    enqueueStreamText,
    flushStreamTextNow,
    flushUpstreamThinkNow,
    resetStreamBuffer,
    startStream,
  };
}
