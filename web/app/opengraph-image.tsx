import { ImageResponse } from "next/og";

export const alt = "Switchboard: every call answered, every job booked";
export const size = { width: 1200, height: 630 };
export const contentType = "image/png";

export default function Image() {
  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          flexDirection: "column",
          justifyContent: "space-between",
          backgroundColor: "#111111",
          color: "#EAE5DC",
          padding: "64px",
        }}
      >
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: "16px",
            fontSize: 28,
            fontWeight: 600,
          }}
        >
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              width: 56,
              height: 56,
              borderRadius: 16,
              backgroundColor: "#C87941",
              color: "#111111",
              fontSize: 24,
            }}
          >
            SB
          </div>
          Switchboard
        </div>

        <div
          style={{
            display: "flex",
            flexDirection: "column",
            fontSize: 72,
            fontWeight: 600,
            lineHeight: 1.05,
            maxWidth: 900,
          }}
        >
          <span>Every call answered.</span>
          <span>Every job booked.</span>
        </div>

        <div style={{ fontSize: 28, color: "#9A948B" }}>
          AI voice reception for home services
        </div>
      </div>
    ),
    { ...size },
  );
}
