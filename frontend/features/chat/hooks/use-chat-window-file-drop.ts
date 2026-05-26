"use client";

import * as React from "react";

type UseChatWindowFileDropOptions = {
  disabled: boolean;
  onDropFiles: (files: File[]) => void | Promise<void>;
};

function hasFileTransfer(dataTransfer: DataTransfer | null): boolean {
  if (!dataTransfer) {
    return false;
  }
  return Array.from(dataTransfer.types).includes("Files");
}

export function useChatWindowFileDrop({
  disabled,
  onDropFiles,
}: UseChatWindowFileDropOptions) {
  const [isDraggingFiles, setIsDraggingFiles] = React.useState(false);
  const disabledRef = React.useRef(disabled);
  const onDropFilesRef = React.useRef(onDropFiles);
  const dragDepthRef = React.useRef(0);
  const isDraggingFilesRef = React.useRef(false);

  React.useEffect(() => {
    disabledRef.current = disabled;
  }, [disabled]);

  React.useEffect(() => {
    onDropFilesRef.current = onDropFiles;
  }, [onDropFiles]);

  React.useEffect(() => {
    const showDragging = () => {
      if (isDraggingFilesRef.current) {
        return;
      }
      isDraggingFilesRef.current = true;
      setIsDraggingFiles(true);
    };

    const resetDragging = () => {
      dragDepthRef.current = 0;
      if (!isDraggingFilesRef.current) {
        return;
      }
      isDraggingFilesRef.current = false;
      setIsDraggingFiles(false);
    };

    const updateDropEffect = (event: DragEvent) => {
      if (event.dataTransfer) {
        event.dataTransfer.dropEffect = disabledRef.current ? "none" : "copy";
      }
    };

    const onDragEnter = (event: DragEvent) => {
      if (!hasFileTransfer(event.dataTransfer)) {
        return;
      }
      event.preventDefault();
      updateDropEffect(event);
      dragDepthRef.current += 1;
      showDragging();
    };

    const onDragOver = (event: DragEvent) => {
      if (!hasFileTransfer(event.dataTransfer)) {
        return;
      }
      event.preventDefault();
      updateDropEffect(event);
      showDragging();
    };

    const onDragLeave = (event: DragEvent) => {
      if (!hasFileTransfer(event.dataTransfer)) {
        return;
      }
      event.preventDefault();
      dragDepthRef.current = Math.max(0, dragDepthRef.current - 1);
      if (dragDepthRef.current === 0) {
        resetDragging();
      }
    };

    const onDrop = (event: DragEvent) => {
      if (!hasFileTransfer(event.dataTransfer)) {
        return;
      }
      event.preventDefault();
      const files = Array.from(event.dataTransfer?.files ?? []);
      resetDragging();
      if (disabledRef.current || files.length === 0) {
        return;
      }
      void Promise.resolve(onDropFilesRef.current(files)).catch(() => {
        // Existing upload handlers surface user-facing failures.
      });
    };

    const listenerOptions = { capture: true };
    window.addEventListener("dragenter", onDragEnter, listenerOptions);
    window.addEventListener("dragover", onDragOver, listenerOptions);
    window.addEventListener("dragleave", onDragLeave, listenerOptions);
    window.addEventListener("drop", onDrop, listenerOptions);
    window.addEventListener("blur", resetDragging);

    return () => {
      window.removeEventListener("dragenter", onDragEnter, listenerOptions);
      window.removeEventListener("dragover", onDragOver, listenerOptions);
      window.removeEventListener("dragleave", onDragLeave, listenerOptions);
      window.removeEventListener("drop", onDrop, listenerOptions);
      window.removeEventListener("blur", resetDragging);
    };
  }, []);

  return { isDraggingFiles };
}
