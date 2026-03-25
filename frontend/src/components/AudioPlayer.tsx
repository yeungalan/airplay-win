"use client";

import type { ServerStatus } from "@/lib/types";

interface Props {
  status: ServerStatus | null;
}

export function AudioPlayer({ status }: Props) {
  return (
    <div className="bg-gray-900 rounded-xl border border-gray-800 p-4">
      <h3 className="text-sm font-medium text-gray-300 mb-3">Audio</h3>
      <div className="flex items-center gap-4">
        <div className="w-16 h-16 bg-gray-800 rounded-lg flex items-center justify-center text-3xl">
          🎵
        </div>
        <div className="flex-1">
          <p className="text-sm text-white">
            {status?.playing ? "Audio streaming" : "No audio"}
          </p>
          <p className="text-xs text-gray-500 mt-1">
            44100 Hz · 16-bit · Stereo
          </p>
          <div className="flex items-center gap-2 mt-2">
            <span className="text-xs text-gray-400">🔊</span>
            <div className="flex-1 bg-gray-700 rounded-full h-1">
              <div className="bg-green-500 h-1 rounded-full w-3/4" />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
