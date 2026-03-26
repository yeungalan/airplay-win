"use client";

import { useEffect, useRef } from "react";
import { Icon } from "@iconify/react";

interface Props {
  active: boolean;
  width: number;
  height: number;
}

export function MirrorView({ active, width, height }: Props) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const decoderRef = useRef<VideoDecoder | null>(null);
  const baseNtpRef = useRef<bigint>(BigInt(0));

  useEffect(() => {
    if (!active) return;

    function ntpToMicros(ntp: bigint): number {
      if (baseNtpRef.current === 0n) baseNtpRef.current = ntp;
      const delta = ntp - baseNtpRef.current;
      return Number((delta * 1_000_000n) >> 32n);
    }

    function initDecoder(codecStr: string, description: ArrayBuffer) {
      if (decoderRef.current) {
        try { decoderRef.current.close(); } catch { /* ignore */ }
      }
      // @ts-ignore
      const decoder = new VideoDecoder({
        output(frame: VideoFrame) {
          const canvas = canvasRef.current;
          if (canvas) {
            const ctx = canvas.getContext("2d");
            ctx?.drawImage(frame, 0, 0, canvas.width, canvas.height);
          }
          frame.close();
        },
        error(e: Error) {
          console.warn("VideoDecoder error:", e);
        },
      });
      decoder.configure({ codec: codecStr, description, optimizeForLatency: true });
      decoderRef.current = decoder;
    }

    function handleMirrorData(e: Event) {
      const ab = (e as CustomEvent<ArrayBuffer>).detail;
      const view = new DataView(ab);
      const type = view.getUint8(0);

      if (type === 0x01) {
        // Codec data (avcC)
        const avcC = new Uint8Array(ab, 1);
        if (avcC.length < 4) return;
        const profile = avcC[1].toString(16).padStart(2, "0");
        const compat  = avcC[2].toString(16).padStart(2, "0");
        const level   = avcC[3].toString(16).padStart(2, "0");
        const codecStr = `avc1.${profile}${compat}${level}`;
        const description = ab.slice(1);
        initDecoder(codecStr, description);

      } else if (type === 0x02) {
        // Video frame
        const decoder = decoderRef.current;
        if (!decoder || decoder.state !== "configured") return;
        const ntpHi = BigInt(view.getUint32(1)) << 32n;
        const ntpLo = BigInt(view.getUint32(5));
        const ntp = ntpHi | ntpLo;
        const isKey = view.getUint8(9) === 1;
        const nalData = ab.slice(10);
        try {
          // @ts-ignore
          decoder.decode(new EncodedVideoChunk({
            type: isKey ? "key" : "delta",
            timestamp: ntpToMicros(ntp),
            data: nalData,
          }));
        } catch { /* drop bad frames */ }
      }
    }

    window.addEventListener("mirrorData", handleMirrorData);
    return () => {
      window.removeEventListener("mirrorData", handleMirrorData);
      if (decoderRef.current) {
        try { decoderRef.current.close(); } catch { /* ignore */ }
        decoderRef.current = null;
      }
      baseNtpRef.current = BigInt(0);
    };
  }, [active]);

  if (!active) return null;

  return (
    <div className="absolute inset-0 bg-black animate-fade-in">
      <div className="absolute top-4 right-4 z-10 flex items-center gap-2">
        <span className="w-2 h-2 bg-red-500 rounded-full animate-pulse" />
        <Icon icon="mdi:cast-connected" className="w-4 h-4 text-white/50" />
      </div>
      <canvas
        ref={canvasRef}
        width={width}
        height={height}
        className="w-full h-full object-contain"
      />
    </div>
  );
}
