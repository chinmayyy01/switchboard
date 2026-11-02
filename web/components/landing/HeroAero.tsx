"use client";

import dynamic from "next/dynamic";
import { useEffect, useState } from "react";

const AeroShards = dynamic(() => import("@/components/AeroShards"), {
  ssr: false,
  loading: () => null,
});

export function HeroAero() {
  const [paused, setPaused] = useState(false);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    const query = window.matchMedia("(prefers-reduced-motion: reduce)");
    const sync = () => setPaused(query.matches);
    sync();
    query.addEventListener?.("change", sync);
    return () => query.removeEventListener?.("change", sync);
  }, []);

  if (failed) return null;

  return (
    <AeroShards
      backgroundColor="#111111"
      shardColor="#C87941"
      accentColor="#EAE5DC"
      lightMode={false}
      placement="full"
      material="glass"
      count={14}
      shardSize={1}
      spread={1}
      twist={1}
      billow={1}
      speed={0.45}
      scale={1}
      depthOfField={1}
      glow={0.45}
      grain={0.08}
      vignette={0.55}
      intro
      mouseInteraction={false}
      mouseStrength={0}
      markers="plus"
      parallax={0}
      paused={paused}
      className="h-full w-full"
      onError={() => setFailed(true)}
    />
  );
}
