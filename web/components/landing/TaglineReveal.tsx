"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { type } from "@/components/landing/type";

export function TaglineReveal({ text }: { text: string }) {
  const words = useMemo(() => text.split(" "), [text]);
  const refs = useRef<(HTMLSpanElement | null)[]>([]);
  const [active, setActive] = useState<number[]>([]);

  useEffect(() => {
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
      setActive(words.map((_, i) => i));
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        const reached: number[] = [];
        for (const entry of entries) {
          if (!entry.isIntersecting) continue;
          const i = Number((entry.target as HTMLElement).dataset.i);
          reached.push(i);
          observer.unobserve(entry.target);
        }
        if (reached.length) {
          setActive((current) => [...new Set([...current, ...reached])]);
        }
      },
      { threshold: 1, rootMargin: "0px 0px -30% 0px" },
    );
    for (const el of refs.current) {
      if (el) observer.observe(el);
    }
    return () => observer.disconnect();
  }, [words]);

  return (
    <p className={`${type.heading3} mx-auto max-w-[680px] text-center`}>
      {words.map((word, i) => (
        <span key={`${word}-${i}`}>
          <span
            ref={(el) => {
              refs.current[i] = el;
            }}
            data-i={i}
            className={`inline-block transition-opacity duration-500 ease-[cubic-bezier(0.22,1,0.36,1)] motion-reduce:opacity-100 ${
              active.includes(i) ? "opacity-100" : "opacity-30"
            }`}
          >
            {word}
          </span>
          {i < words.length - 1 ? " " : ""}
        </span>
      ))}
    </p>
  );
}
