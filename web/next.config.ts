import type { NextConfig } from "next";

const internalAPIOrigin = (process.env.FITLOG_API_INTERNAL_URL ?? "http://localhost:8080").replace(/\/$/, "");

const nextConfig: NextConfig = {
  output: "standalone",
  reactStrictMode: true,
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${internalAPIOrigin}/api/:path*` }];
  },
};

export default nextConfig;
