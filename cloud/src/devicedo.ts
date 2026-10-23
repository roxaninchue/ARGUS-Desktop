import { DurableObject } from "cloudflare:workers";
import { DeviceCore, type DeviceRecord } from "./devicecore";

/** One instance per device code ("d:<sha256(device_code)>") and per user code ("u:<ABCD-EFGH>", which only holds the device key). */
export class DeviceDO extends DurableObject {
  private core: DeviceCore;
  constructor(ctx: DurableObjectState, env: unknown) {
    super(ctx, env as never);
    const st = ctx.storage;
    this.core = new DeviceCore({
      get: <T>(k: string) => st.get<T>(k),
      put: (k, v) => st.put(k, v),
      deleteAll: () => st.deleteAll(),
      setAlarm: ms => st.setAlarm(ms),
    });
  }
  init(rec: DeviceRecord) { return this.core.init(rec); }
  get() { return this.core.get(); }
  decide(status: "approved" | "denied", uid?: string) { return this.core.decide(status, uid); }
  wait(ms: number) { return this.core.wait(ms); }
  take() { return this.core.take(); }
  async alarm() { await this.core.alarm(); }
}
