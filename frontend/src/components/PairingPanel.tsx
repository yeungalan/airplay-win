"use client";

import { Icon } from "@iconify/react";

interface Props {
  paired: boolean;
  pin: string;
}

export function PairingPanel({ paired, pin }: Props) {
  if (paired) return null;

  return (
    <div className="absolute inset-0 flex items-center justify-center z-30 bg-black/80 backdrop-blur-sm animate-fade-in">
      <div className="text-center animate-slide-up">
        <Icon icon="mdi:lock-outline" className="w-10 h-10 text-white/40 mx-auto mb-6" />
        <p className="text-sm text-white/50 font-light mb-6 tracking-wide">
          Enter the code on your device
        </p>
        <div className="flex justify-center gap-3">
          {pin.split("").map((digit, i) => (
            <div
              key={i}
              className="w-16 h-20 bg-white/10 backdrop-blur rounded-2xl flex items-center justify-center border border-white/10"
            >
              <span className="text-3xl font-extralight text-white">
                {digit}
              </span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
