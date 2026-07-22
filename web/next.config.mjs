// The browser talks to same-origin /api/* and Next proxies it to the gateway,
// so no CORS setup is needed anywhere. Server components use API_URL directly.
// Render's fromService hostport has no scheme, so add one if missing.
let apiUrl = process.env.API_URL || "http://localhost:8080";
if (!apiUrl.startsWith("http")) apiUrl = `http://${apiUrl}`;

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "standalone",
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${apiUrl}/:path*` }];
  },
};

export default nextConfig;
