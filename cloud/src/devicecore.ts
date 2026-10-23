// 0.9.7: device-code state in a Durable Object (strongly consistent, one instance per code), so a poll sees an
// approval the moment it lands. Workers KV is cached at the edge (~60 s) and eventually consistent across colos:
// approve -> signed-in took ~1 min on the CLI. The DO also holds long-polls (POST /device/poll {wait}) open and
// wakes them on approve/deny. Logic lives here (plain class, unit-tested); src/devicedo.ts wraps it as the DO.

export interface DeviceRecord { user_code: string; client: string; status: "pending" | "approved" | "denied" | "used"; uid?: string; exp: number; key?: string }
export interface Store {
  get<T>(k: string): Promise<T | undefined>;
  put(k: string, v: unknown): Promise<void>;
  deleteAll(): Promise<void>;
  setAlarm(ms: number): Promise<void>;
}
const nowS = () => Math.floor(Date.now() / 1000);

export class DeviceCore {
  private waiters: (() => void)[] = [];
  constructor(private s: Store) {}
  private wake() { const w = this.waiters; this.waiters = []; for (const f of w) f(); }
  async init(rec: DeviceRecord): Promise<boolean> {
    if (await this.s.get("rec")) return false;
    await this.s.put("rec", rec);
    await this.s.setAlarm((rec.exp + 60) * 1000);
    return true;
  }
  async get(): Promise<DeviceRecord | null> {
    const r = await this.s.get<DeviceRecord>("rec");
    return r && r.exp >= nowS() ? r : null;
  }
  /** pending -> approved/denied. Returns the record before the change (null when not pending / gone). */
  async decide(status: "approved" | "denied", uid?: string): Promise<DeviceRecord | null> {
    const r = await this.get();
    if (!r || r.status !== "pending") return null;
    const next: DeviceRecord = { ...r, status, ...(uid ? { uid } : {}) };
    await this.s.put("rec", next);
    this.wake();
    return r;
  }
  /** Wait up to [ms] while pending; returns the record as it then stands. */
  async wait(ms: number): Promise<DeviceRecord | null> {
    let r = await this.get();
    if (!r || r.status !== "pending" || ms <= 0) return r;
    await new Promise<void>(res => {
      const t = setTimeout(() => { this.waiters = this.waiters.filter(f => f !== done); res(); }, ms);
      const done = () => { clearTimeout(t); res(); };
      this.waiters.push(done);
    });
    r = await this.get();
    return r;
  }
  /** The first poll that sees "approved" takes it (single use); later ones get null. */
  async take(): Promise<DeviceRecord | null> {
    const r = await this.get();
    if (!r || r.status !== "approved") return null;
    await this.s.put("rec", { ...r, status: "used" });
    return r;
  }
  async alarm(): Promise<void> { await this.s.deleteAll(); }
}
