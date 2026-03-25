"use client";

interface Props {
  photoData: string | null;
}

export function PhotoViewer({ photoData }: Props) {
  if (!photoData) return null;

  return (
    <div className="absolute inset-0 flex items-center justify-center bg-black animate-fade-in">
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={photoData}
        alt="AirPlay Photo"
        className="max-w-full max-h-full object-contain"
      />
    </div>
  );
}
