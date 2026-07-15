// The single place minor units become display strings (CLAUDE.md section 17).
// 1010 minor units of USD renders as $10.10 USD.

const SYMBOLS: Record<string, string> = { USD: "$", EUR: "€", GBP: "£", JPY: "¥" };

export function formatMoney(minor: number, currency: string): string {
  const symbol = SYMBOLS[currency] ?? "";
  const sign = minor < 0 ? "-" : "";
  const abs = Math.abs(minor);
  const major = Math.floor(abs / 100);
  const cents = String(abs % 100).padStart(2, "0");
  return `${sign}${symbol}${major.toLocaleString("en-US")}.${cents} ${currency}`;
}

export function Money({ minor, currency }: { minor: number; currency: string }) {
  return (
    <span className="tabular-nums whitespace-nowrap">{formatMoney(minor, currency)}</span>
  );
}
