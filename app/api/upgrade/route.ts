import { NextResponse } from "next/server";
import { normalizeDomain, type UpgradePayload } from "@/lib/scan";

function isValidEmail(email: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email);
}

export async function POST(request: Request) {
  const body = (await request.json().catch(() => ({}))) as Partial<UpgradePayload>;
  const payload: UpgradePayload = {
    name: (body.name || "").trim(),
    domain: normalizeDomain(body.domain || ""),
    email: (body.email || "").trim().toLowerCase(),
    plan: body.plan === "managed" ? "managed" : "saas",
  };

  if (!payload.name || !payload.domain || !isValidEmail(payload.email)) {
    return NextResponse.json({ error: "Name, domain, and contact email are required." }, { status: 400 });
  }

  if (process.env.NOCODB_WEBHOOK_URL) {
    const response = await fetch(process.env.NOCODB_WEBHOOK_URL, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        ...payload,
        source: "dmark.is",
        requestedAt: new Date().toISOString(),
      }),
    });

    if (!response.ok) {
      return NextResponse.json({ error: "Upgrade request could not be submitted." }, { status: 502 });
    }
  }

  return NextResponse.json({ ok: true });
}
