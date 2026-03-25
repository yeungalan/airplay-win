"use client";

import { Icon } from "@iconify/react";

interface Props {
  active: boolean;
  width: number;
  height: number;
}

export function MirrorView({ active, width, height }: Props) {
  if (!active) return null;

  return (
    <div className="absolute inset-0 bg-black animate-fade-in">
      <div className="absolute top-4 right-4 z-10 flex items-center gap-2">
        <span className="w-2 h-2 bg-red-500 rounded-full animate-pulse" />
        <Icon icon="mdi:cast-connected" className="w-4 h-4 text-white/50" />
      </div>
      <canvas
        id="mirror-canvas"
        width={width}
        height={height}
        className="w-full h-full object-contain"
      />
    </div>
  );
}
