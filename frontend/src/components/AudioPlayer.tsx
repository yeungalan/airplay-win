"use client";

import { Icon } from "@iconify/react";
import type { ServerStatus } from "@/lib/types";

interface Props {
  status: ServerStatus | null;
  volume: number;
}

export function AudioPlayer({ status, volume }: Props) {
  if (!status?.playing) return null;

  return (
    <div className="absolute bottom-8 left-1/2 -translate-x-1/2 z-10 animate-slide-up">
      <div className="flex items-center gap-4 bg-white/10 backdrop-blur-xl rounded-2xl px-6 py-4 border border-white/5">
        <div className="w-12 h-12 bg-white/10 rounded-xl flex items-center justify-center">
          <Icon icon="mdi:music-note" className="w-6 h-6 text-white/80" />
        </div>
        <div>
          <p className="text-sm text-white font-light">Audio Streaming</p>
          <div className="flex items-center gap-2 mt-1">
            <Icon icon="mdi:volume-high" className="w-3.5 h-3.5 text-white/40" />
            <div className="w-24 h-1 bg-white/20 rounded-full">
              <div
                className="h-full bg-white/60 rounded-full transition-all"
                style={{ width: `${volume}%` }}
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
