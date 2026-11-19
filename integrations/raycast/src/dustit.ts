import { execFile } from "node:child_process";
import { existsSync } from "node:fs";
import { homedir } from "node:os";

// Raycast runs commands with a minimal PATH, so look where DustIt is usually installed.
const candidates = [
  "/opt/homebrew/bin/dustit",
  "/usr/local/bin/dustit",
  `${homedir()}/.local/bin/dustit`,
  `${homedir()}/go/bin/dustit`,
];

export function dustitPath(): string | undefined {
  return candidates.find((p) => existsSync(p));
}

export const installHint = "Install DustIt first: brew install iamolayemi/tap/dustit";

export function run(args: string[]): Promise<string> {
  const bin = dustitPath();
  if (!bin) return Promise.reject(new Error(installHint));
  return new Promise((resolve, reject) => {
    execFile(bin, [...args, "--no-color"], { maxBuffer: 64 * 1024 * 1024, timeout: 10 * 60 * 1000 }, (err, stdout, stderr) => {
      if (err) reject(new Error((stderr || err.message).trim()));
      else resolve(stdout);
    });
  });
}

export async function runJSON<T>(args: string[]): Promise<T> {
  return JSON.parse(await run([...args, "--json"])) as T;
}

export function human(bytes: number): string {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let v = bytes;
  let i = 0;
  while (v >= 1000 && i < units.length - 1) {
    v /= 1000;
    i++;
  }
  return i === 0 ? `${bytes} B` : `${v.toFixed(1)} ${units[i]}`;
}

export interface Category {
  id: string;
  name: string;
  description: string;
  risk: "safe" | "review";
  size_bytes: number;
  item_count: number;
}

export interface Scan {
  free_bytes: number;
  safe_bytes: number;
  review_bytes: number;
  categories: Category[];
}

export interface Group {
  name: string;
  hint?: string;
  size_bytes: number;
}

export interface Finding {
  check: string;
  severity: "ok" | "info" | "warn" | "bad";
  summary: string;
  short: string;
  advice?: string[];
}

export interface Change {
  name: string;
  before: number;
  after: number;
  delta: number;
}
