import { NextResponse } from 'next/server';
import { Pool } from 'pg';

const pool = new Pool({
  connectionString: process.env.DATABASE_URL
});

export async function GET(req: Request) {
  try {
    const { searchParams } = new URL(req.url);
    const driverId = searchParams.get('driver_id');

    if (!driverId) {
      return NextResponse.json({ error: 'Driver ID required' }, { status: 400 });
    }

    // Orders
    const ordersResult = await pool.query(
      `SELECT * FROM orders WHERE driver_id = $1 AND status IN ('assigned', 'picked_up') ORDER BY delivery_sequence_priority DESC, created_at DESC`,
      [driverId]
    );

    // Wallet
    const walletResult = await pool.query(
      `SELECT * FROM driver_wallets WHERE driver_id = $1`,
      [driverId]
    );

    // Completed today
    const completedResult = await pool.query(
      `SELECT COUNT(*) as count FROM orders WHERE driver_id = $1 AND status = 'delivered' AND DATE(updated_at) = CURRENT_DATE`,
      [driverId]
    );

    return NextResponse.json({
      orders: ordersResult.rows,
      wallet: walletResult.rows[0] || null,
      completedToday: parseInt(completedResult.rows[0].count, 10)
    });
  } catch (error) {
    console.error('Driver init error:', error);
    return NextResponse.json({ error: 'Internal Server Error' }, { status: 500 });
  }
}
