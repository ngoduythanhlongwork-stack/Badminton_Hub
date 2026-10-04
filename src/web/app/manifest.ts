import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "Badminton Hub",
    short_name: "Badminton Hub",
    description: "Muốn đánh cầu — luôn có kèo phù hợp.",
    start_url: "/",
    display: "standalone",
    background_color: "#f4f8f5",
    theme_color: "#0b6e4f",
    lang: "vi",
  };
}
