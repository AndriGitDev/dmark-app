import { NextResponse } from "next/server";
import { mockScan, normalizeDomain, type ScanPayload } from "@/lib/scan";

type GoScanResponse = {
  domain?: string;
  summary?: {
    status?: string;
  };
  checks?: Array<{
    name?: string;
    status?: string;
    record?: string;
    lookup_count?: number;
    lookups?: number;
  }>;
};

function statusFromGo(status?: string): ScanPayload["status"] {
  const normalized = (status || "").toLowerCase();
  if (normalized === "pass") return "PASS";
  if (normalized === "warn" || normalized === "warning") return "WARNING";
  return "FAIL";
}

function mapGoResponse(payload: GoScanResponse, fallbackDomain: string): ScanPayload {
  const checks = payload.checks || [];
  const spf = checks.find((check) => check.name?.toLowerCase() === "spf");
  const dmarc = checks.find((check) => check.name?.toLowerCase() === "dmarc");
  const policy = dmarc?.record?.match(/p=([^;\s]+)/i)?.[1] || "none";
  const spfLookups = spf?.lookup_count || spf?.lookups || 0;

  return {
    domain: payload.domain || fallbackDomain,
    status: statusFromGo(payload.summary?.status),
    spf: {
      valid: statusFromGo(spf?.status) !== "FAIL",
      lookups: spfLookups,
      record: spf?.record || "No SPF record returned.",
    },
    dmarc: {
      valid: policy !== "none" && statusFromGo(dmarc?.status) !== "FAIL",
      policy,
      record: dmarc?.record || "No DMARC record returned.",
    },
    spoofable: policy === "none" || statusFromGo(payload.summary?.status) === "FAIL",
  };
}

export async function POST(request: Request) {
  const body = (await request.json().catch(() => ({}))) as { domain?: string };
  const domain = normalizeDomain(body.domain || "");

  if (!domain) {
    return NextResponse.json({ error: "Domain is required." }, { status: 400 });
  }

  const upstream = process.env.DMARK_SCAN_API_URL;
  const useMock = process.env.DMARK_USE_MOCK !== "false";

  if (!upstream || useMock) {
    await new Promise((resolve) => setTimeout(resolve, 900));
    return NextResponse.json(mockScan(domain));
  }

  const url = new URL(upstream);
  url.searchParams.set("domain", domain);

  const response = await fetch(url, {
    headers: {
      accept: "application/json",
      ...(process.env.DMARK_API_TOKEN
        ? { authorization: `Bearer ${process.env.DMARK_API_TOKEN}` }
        : {}),
    },
    next: { revalidate: 0 },
  });

  if (!response.ok) {
    return NextResponse.json({ error: "Scan service unavailable." }, { status: 502 });
  }

  const payload = (await response.json()) as GoScanResponse;
  return NextResponse.json(mapGoResponse(payload, domain));
}
