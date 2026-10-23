// vitest stand-in for "cloudflare:workers" (the DO base class); the real one exists only in workerd.
export class DurableObject { ctx: unknown; env: unknown; constructor(ctx: unknown, env: unknown) { this.ctx = ctx; this.env = env; } }
