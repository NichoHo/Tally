"use client";

import { Bar, BarChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

type Day = { date: string; volume_minor: number; count: number };

export function VolumeChart({ data }: { data: Day[] }) {
  const rows = data.map((d) => ({
    // Show as "Jul 14" style labels; volume in major units for the axis.
    label: new Date(d.date + "T00:00:00").toLocaleDateString("en-US", {
      month: "short",
      day: "numeric",
    }),
    volume: d.volume_minor / 100,
    count: d.count,
  }));

  return (
    <div className="h-56 w-full">
      <ResponsiveContainer>
        <BarChart data={rows} margin={{ top: 8, right: 8, bottom: 0, left: 8 }}>
          <XAxis
            dataKey="label"
            tickLine={false}
            axisLine={{ stroke: "#E7E7E2" }}
            tick={{ fill: "#6B6B66", fontSize: 12 }}
          />
          <YAxis
            tickLine={false}
            axisLine={false}
            tick={{ fill: "#6B6B66", fontSize: 12 }}
            tickFormatter={(v: number) => `$${v.toLocaleString("en-US")}`}
            width={70}
          />
          <Tooltip
            cursor={{ fill: "#FAFAF8" }}
            formatter={(value: number | string) => [
              `$${Number(value).toLocaleString("en-US", { minimumFractionDigits: 2 })}`,
              "volume",
            ]}
            contentStyle={{
              border: "1px solid #E7E7E2",
              borderRadius: 8,
              fontSize: 12,
            }}
          />
          <Bar dataKey="volume" fill="#1D7A8C" radius={[3, 3, 0, 0]} maxBarSize={48} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
