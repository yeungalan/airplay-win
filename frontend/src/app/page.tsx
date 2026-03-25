"use client";

import dynamic from "next/dynamic";

const AirPlayDisplay = dynamic(() => import("@/components/AirPlayDisplay"), {
  ssr: false,
});

export default function Home() {
  return <AirPlayDisplay />;
}
