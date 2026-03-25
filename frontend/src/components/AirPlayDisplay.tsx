"use client";

import { useAirPlay } from "@/hooks/useAirPlay";
import { StatusBar } from "@/components/StatusBar";
import { VideoPlayer } from "@/components/VideoPlayer";
import { PhotoViewer } from "@/components/PhotoViewer";
import { MirrorView } from "@/components/MirrorView";
import { PairingPanel } from "@/components/PairingPanel";
import { AudioPlayer } from "@/components/AudioPlayer";
import { IdleScreen } from "@/components/IdleScreen";

export default function AirPlayDisplay() {
  const { connectionState, status, photoData, mirroring, volume, showPairing } =
    useAirPlay();

  const isIdle = !status?.playing && !photoData && !mirroring;

  return (
    <div className="relative w-screen h-screen bg-black overflow-hidden">
      {/* Pairing overlay (highest z) */}
      {showPairing && !status?.paired && (
        <PairingPanel paired={false} pin={status?.pin ?? "3939"} />
      )}

      {/* Content layers */}
      {mirroring ? (
        <MirrorView
          active
          width={status?.width ?? 1920}
          height={status?.height ?? 1080}
        />
      ) : photoData ? (
        <PhotoViewer photoData={photoData} />
      ) : status?.playing && status?.url ? (
        <VideoPlayer
          playing={status.playing}
          url={status.url}
          position={status.position}
          duration={status.duration}
          rate={status.rate}
        />
      ) : null}

      {/* Audio indicator */}
      <AudioPlayer status={status} volume={volume} />

      {/* Idle screen */}
      {isIdle && (
        <IdleScreen
          name={status?.name ?? "AirPlay"}
          connectionState={connectionState}
        />
      )}

      {/* Subtle top-left name badge when content is playing */}
      {!isIdle && (
        <StatusBar name={status?.name} connectionState={connectionState} />
      )}
    </div>
  );
}
