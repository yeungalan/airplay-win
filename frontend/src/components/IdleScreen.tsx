"use client";

import { Icon } from "@iconify/react";
import type { ConnectionState } from "@/lib/types";

interface Props {
  name: string;
  connectionState: ConnectionState;
}

export function IdleScreen({ name, connectionState }: Props) {
  return (
    <div className="absolute inset-0 flex flex-col items-center justify-center">
      {connectionState === "connected" ? (
        <div className="text-center animate-fade-in">
          <Icon
            icon="mdi:airplay"
            className="w-20 h-20 text-white/20 mx-auto mb-6"
          />
          <h1 className="text-2xl font-extralight text-white/60 tracking-wide mb-2">
            {name}
          </h1>
          <p className="text-sm text-white/30 font-light">
            Use AirPlay to stream to this display
          </p>
        </div>
      ) : connectionState === "connecting" ? (
        <div className="text-center animate-fade-in">
          <Icon
            icon="mdi:loading"
            className="w-12 h-12 text-white/20 mx-auto mb-4 animate-spin"
          />
          <p className="text-sm text-white/30 font-light">Connecting...</p>
        </div>
      ) : (
        <div className="text-center animate-fade-in">
          <Icon
            icon="mdi:wifi-off"
            className="w-12 h-12 text-white/15 mx-auto mb-4"
          />
          <p className="text-sm text-white/25 font-light">Disconnected</p>
        </div>
      )}
    </div>
  );
}
