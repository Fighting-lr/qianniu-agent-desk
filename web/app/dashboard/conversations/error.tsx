"use client";

import { useEffect } from "react";

export default function ConversationsError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // eslint-disable-next-line no-console
    console.error("[ConversationsError]", error);
  }, [error]);

  return (
    <div className="flex h-full flex-col items-center justify-center gap-4 p-8 text-red-600">
      <h2 className="text-xl font-bold">客服工作台加载失败</h2>
      <pre className="max-w-2xl whitespace-pre-wrap rounded bg-red-50 p-4 text-sm">
        {error.name}: {error.message}
        {"\n"}
        {error.stack}
        {error.digest ? `\n\ndigest: ${error.digest}` : ""}
      </pre>
      <button
        type="button"
        onClick={() => reset()}
        className="rounded bg-primary px-4 py-2 text-primary-foreground"
      >
        重试
      </button>
    </div>
  );
}
