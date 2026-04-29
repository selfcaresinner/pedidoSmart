import { NextResponse } from 'next/server';

export async function POST(request: Request) {
  try {
    const body = await request.json();
    
    // We forward the request to the Go backend.
    // The Go backend runs on a different port locally or a known URL in prod.
    const goBackendUrl = process.env.GO_BACKEND_URL || 'http://localhost:8080';
    
    const res = await fetch(`${goBackendUrl}/api/merchant/confirm`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
    });

    if (!res.ok) {
        return NextResponse.json({ error: 'Backend error' }, { status: res.status });
    }

    const data = await res.json();
    return NextResponse.json(data);

  } catch (err: any) {
    return NextResponse.json({ error: err.message }, { status: 500 });
  }
}
