(() => {
  class BlazeEmblem extends HTMLElement {
    constructor() {
      super();
      this.canvas = document.createElement('canvas');
      this.ctx = this.canvas.getContext('2d');
      this.raf = 0;
      this.observer = null;
      this.animate = this.hasAttribute('animate');
      this.attachShadow({ mode: 'open' });
      this.shadowRoot.appendChild(this.canvas);
    }
    connectedCallback() {
      const size = Number(this.getAttribute('size')) || 22;
      this.canvas.width = size * devicePixelRatio;
      this.canvas.height = size * devicePixelRatio;
      this.canvas.style.width = size + 'px';
      this.canvas.style.height = size + 'px';
      this._draw();
      if (!this.observer) {
        this.observer = new IntersectionObserver(entries => {
          const visible = entries.some(e => e.isIntersecting);
          if (!visible || !this.animate || !window.BlazeMotion || !window.BlazeMotion.enabled) this.stop();
          else this.start();
        }, { threshold: 0.1 });
        this.observer.observe(this);
      }
      document.addEventListener('visibilitychange', () => document.hidden ? this.stop() : this.start());
    }
    start() {
      if (this.raf || !this.animate || !window.BlazeMotion || !window.BlazeMotion.enabled) return;
      const tick = t => {
        this._draw(t * 0.001);
        this.raf = requestAnimationFrame(tick);
      };
      this.raf = requestAnimationFrame(tick);
    }
    stop() {
      if (this.raf) cancelAnimationFrame(this.raf);
      this.raf = 0;
    }
    _draw(t = 0) {
      const ctx = this.ctx; if (!ctx) return;
      const dpr = devicePixelRatio || 1; const size = this.canvas.width / dpr;
      const cx = size / 2, cy = size / 2, r = size * 0.19;
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
      ctx.clearRect(0, 0, size, size);
      ctx.beginPath();
      ctx.arc(cx, cy, r * 1.75, 0, Math.PI * 2);
      ctx.strokeStyle = 'rgba(255,201,77,.85)';
      ctx.lineWidth = r * 0.2;
      ctx.shadowBlur = 14; ctx.shadowColor = 'rgba(255,201,77,.42)';
      ctx.stroke();
      ctx.shadowBlur = 0;
      ctx.beginPath(); ctx.arc(cx, cy, r * 1.12, 0, Math.PI * 2); ctx.fillStyle = 'rgba(255,255,255,.16)'; ctx.fill();
      ctx.beginPath(); ctx.moveTo(cx, cy - r * 1.4); ctx.bezierCurveTo(cx + r * 1.1, cy - r * 1.2, cx + r * 1.4, cy + r * .8, cx, cy + r * 1.5); ctx.bezierCurveTo(cx - r * 1.4, cy + r * .8, cx - r * 1.1, cy - r * 1.2, cx, cy - r * 1.4); ctx.closePath();
      ctx.fillStyle = 'rgba(255,255,255,.66)'; ctx.fill();
      ctx.beginPath(); ctx.arc(cx + r * .25, cy - r * .22, r * .18, 0, Math.PI * 2); ctx.fillStyle = 'rgba(255,255,255,.92)'; ctx.fill();
      ctx.beginPath(); ctx.arc(cx - r * .2, cy + r * .15, r * .08, 0, Math.PI * 2); ctx.fillStyle = 'rgba(255,255,255,.36)'; ctx.fill();
      ctx.beginPath(); ctx.arc(cx, cy + r * .18, r * .85, 0, Math.PI * 2); ctx.fillStyle = 'rgba(26,26,30,.18)'; ctx.fill();
    }
  }
  customElements.define('blaze-emblem', BlazeEmblem);

  const edgeCanvas = document.createElement('canvas');
  edgeCanvas.id = 'edge-particles';
  Object.assign(edgeCanvas.style, { position: 'fixed', inset: 0, pointerEvents: 'none', zIndex: 0 });
  document.body.appendChild(edgeCanvas);
  const drawEdgeParticles = () => {
    if (!window.BlazeMotion || !window.BlazeMotion.enabled) { edgeCanvas.style.opacity = '0'; return; }
    const ctx = edgeCanvas.getContext('2d'); if (!ctx) return;
    const w = window.innerWidth, h = window.innerHeight;
    edgeCanvas.width = w * devicePixelRatio; edgeCanvas.height = h * devicePixelRatio; edgeCanvas.style.width = w + 'px'; edgeCanvas.style.height = h + 'px';
    ctx.setTransform(devicePixelRatio, 0, 0, devicePixelRatio, 0, 0);
    ctx.clearRect(0, 0, w, h);
    edgeCanvas.style.opacity = '0.55';
    const dots = Array.from({ length: 24 }, (_, i) => ({ x: (i / 24) * w, y: i % 2 ? h * 0.12 : h * 0.92, r: 1.5 + (i % 7) * 0.4 }));
    dots.forEach((d, i) => {
      ctx.beginPath(); ctx.fillStyle = i % 4 === 0 ? 'rgba(255,255,255,.88)' : i % 7 === 0 ? 'rgba(189,189,194,.85)' : 'rgba(255,255,255,.45)'; ctx.arc(d.x, d.y, d.r, 0, Math.PI * 2); ctx.fill();
    });
  };
  window.addEventListener('resize', drawEdgeParticles);
  drawEdgeParticles();
})();
