import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Server Action body limit defaults to 1 MB; therapist + room photo upload
  // accepts up to 5 MB on the frontend, so raise the limit to match.
  experimental: {
    serverActions: {
      bodySizeLimit: "5mb",
    },
  },
};

export default nextConfig;
