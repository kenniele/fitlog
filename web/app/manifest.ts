import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    id: "/",
    name: "FitLog",
    short_name: "FitLog",
    description: "Восстановление, сон и тренировки — в одной сводке.",
    start_url: "/dashboard/summary",
    scope: "/",
    display: "standalone",
    background_color: "#0d1110",
    theme_color: "#0d1110",
    lang: "ru",
    icons: [
      { src: "/icons/fitlog-192.png", sizes: "192x192", type: "image/png" },
      {
        src: "/icons/fitlog-512.png",
        sizes: "512x512",
        type: "image/png",
        purpose: "any",
      },
      {
        src: "/icons/fitlog-maskable.png",
        sizes: "512x512",
        type: "image/png",
        purpose: "maskable",
      },
    ],
  };
}
