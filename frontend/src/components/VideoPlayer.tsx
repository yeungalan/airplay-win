"use client";

import { Icon } from "@iconify/react";

interface Props {
  playing: boolean;
  url: string;
  position: number;
  duration: number;
  rate: number;
}

function formatTime(seconds: number): string {
  if (!seconds || !isFinite(seconds)) return "0:00";
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  if (h > 0) return `${h}:${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
  return `${m}:${s.toString().padStart(2, "0")}`;
}

export function VideoPlayer({ playing, url, position, duration, rate }: Props) {
  if (!playing && !url) return null;

  const progress = duration > 0 ? (position / duration) * 100 : 0;

  return (
    <div className="absolute inset-0 flex flex-col justify-end animate-fade-in">
      {url && (
        <video
          src={url}
          autoPlay={playing}
          className="absolute inset-0 w-full h-full object-contain"
        />
      )}

      {/* Bottom transport bar */}
      <div className="relative z-10 bg-gradient-to-t from-black/90 via-black/40 to-transparent pt-20 pb-8 px-12">
        {/* Progress bar */}
        <div className="w-full h-1 bg-white/20 rounded-full mb-4">
          <div
            className="h-full bg-white rounded-full transition-all duration-300"
            style={{ width: `${progress}%` }}
          />
        </div>

        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <Icon
              icon={rate === 0 ? "mdi:pause" : "mdi:play"}
              className="w-6 h-6 text-white"
            />
            <span className="text-sm text-white/70 font-light tabular-nums">
              {formatTime(position)}
            </span>
          </div>
          <span className="text-sm text-white/50 font-light tabular-nums">
            -{formatTime(Math.max(0, duration - position))}
          </span>
        </div>
      </div>
    </div>
  );
}
