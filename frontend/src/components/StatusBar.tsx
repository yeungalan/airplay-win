"use client";

import { Icon } from "@iconify/react";
import type { ConnectionState } from "@/lib/types";

interface Props {
  name: string | undefined;
  connectionState: ConnectionState;
}

export function StatusBar({ name, connectionState }: Props) {
  if (connectionState !== "connected") {
    return null;
  }

  return (
    <div className="absolute top-6 left-8 z-20 flex items-center gap-2 opacity-60">
      <Icon icon="mdi:airplay" className="w-4 h-4 text-white" />
      <span className="text-xs text-white/80 font-light tracking-wide">
        {name || "AirPlay"}
      </span>
    </div>
  );
}
